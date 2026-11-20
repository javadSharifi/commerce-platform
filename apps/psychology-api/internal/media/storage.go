package media

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrObjectNotFound = errors.New("storage object not found")
)

type ObjectMeta struct {
	Key         string
	Size        int64
	ContentType string
}

type StorageProvider interface {
	PresignPutURL(ctx context.Context, key string, contentType string, sizeBytes int64, ttl time.Duration) (string, error)
	PresignGetURL(ctx context.Context, key string, ttl time.Duration) (string, error)
	HeadObject(ctx context.Context, key string) (*ObjectMeta, error)
	CopyObject(ctx context.Context, srcKey string, dstKey string) error
	DeleteObject(ctx context.Context, key string) error
}

// MockStorageProvider provides an in-memory implementation for testing and development.
type MockStorageProvider struct {
	mu      sync.RWMutex
	objects map[string]*ObjectMeta
}

func NewMockStorageProvider() *MockStorageProvider {
	return &MockStorageProvider{
		objects: make(map[string]*ObjectMeta),
	}
}

func (m *MockStorageProvider) PutMockObject(key string, meta *ObjectMeta) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = meta
}

func (m *MockStorageProvider) GetMockObject(key string) (*ObjectMeta, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	obj, ok := m.objects[key]
	return obj, ok
}

func (m *MockStorageProvider) PresignPutURL(ctx context.Context, key string, contentType string, sizeBytes int64, ttl time.Duration) (string, error) {
	return fmt.Sprintf("https://storage.parspack.mock/staging/%s?ttl=%d", url.PathEscape(key), int(ttl.Seconds())), nil
}

func (m *MockStorageProvider) PresignGetURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.objects[key]; !ok {
		return "", ErrObjectNotFound
	}
	return fmt.Sprintf("https://storage.parspack.mock/media/%s?ttl=%d", url.PathEscape(key), int(ttl.Seconds())), nil
}

func (m *MockStorageProvider) HeadObject(ctx context.Context, key string) (*ObjectMeta, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	obj, ok := m.objects[key]
	if !ok {
		return nil, ErrObjectNotFound
	}
	return &ObjectMeta{
		Key:         obj.Key,
		Size:        obj.Size,
		ContentType: obj.ContentType,
	}, nil
}

func (m *MockStorageProvider) CopyObject(ctx context.Context, srcKey string, dstKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	src, ok := m.objects[srcKey]
	if !ok {
		return ErrObjectNotFound
	}
	m.objects[dstKey] = &ObjectMeta{
		Key:         dstKey,
		Size:        src.Size,
		ContentType: src.ContentType,
	}
	return nil
}

func (m *MockStorageProvider) DeleteObject(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	return nil
}

// S3Config holds connection parameters for S3/Parspack storage.
type S3Config struct {
	Endpoint        string // e.g. "https://s3.ir-tbz-sh1.arvanstorage.ir" or Parspack endpoint
	Bucket          string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
}

// S3StorageProvider implements StorageProvider for S3-compatible object storage via standard AWS SigV4.
type S3StorageProvider struct {
	cfg        S3Config
	httpClient *http.Client
}

func NewS3StorageProvider(cfg S3Config) *S3StorageProvider {
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	return &S3StorageProvider{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *S3StorageProvider) PresignPutURL(ctx context.Context, key string, contentType string, sizeBytes int64, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, s.cfg.Region)
	canonicalURI := "/" + s.cfg.Bucket + "/" + strings.TrimPrefix(key, "/")

	query := url.Values{}
	query.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	query.Set("X-Amz-Credential", fmt.Sprintf("%s/%s", s.cfg.AccessKeyID, credentialScope))
	query.Set("X-Amz-Date", amzDate)
	query.Set("X-Amz-Expires", strconv.Itoa(int(ttl.Seconds())))
	query.Set("X-Amz-SignedHeaders", "host")

	parsedEndpoint, err := url.Parse(s.cfg.Endpoint)
	if err != nil {
		return "", err
	}

	canonicalQueryString := query.Encode()
	canonicalHeaders := fmt.Sprintf("host:%s\n", parsedEndpoint.Host)
	canonicalRequest := fmt.Sprintf("PUT\n%s\n%s\n%s\nhost\nUNSIGNED-PAYLOAD",
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
	)

	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	)

	signingKey := getSignatureKey(s.cfg.SecretAccessKey, dateStamp, s.cfg.Region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	query.Set("X-Amz-Signature", signature)

	return fmt.Sprintf("%s%s?%s", s.cfg.Endpoint, canonicalURI, query.Encode()), nil
}

func (s *S3StorageProvider) PresignGetURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, s.cfg.Region)
	canonicalURI := "/" + s.cfg.Bucket + "/" + strings.TrimPrefix(key, "/")

	query := url.Values{}
	query.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	query.Set("X-Amz-Credential", fmt.Sprintf("%s/%s", s.cfg.AccessKeyID, credentialScope))
	query.Set("X-Amz-Date", amzDate)
	query.Set("X-Amz-Expires", strconv.Itoa(int(ttl.Seconds())))
	query.Set("X-Amz-SignedHeaders", "host")

	parsedEndpoint, err := url.Parse(s.cfg.Endpoint)
	if err != nil {
		return "", err
	}

	canonicalQueryString := query.Encode()
	canonicalHeaders := fmt.Sprintf("host:%s\n", parsedEndpoint.Host)
	canonicalRequest := fmt.Sprintf("GET\n%s\n%s\n%s\nhost\nUNSIGNED-PAYLOAD",
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
	)

	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	)

	signingKey := getSignatureKey(s.cfg.SecretAccessKey, dateStamp, s.cfg.Region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	query.Set("X-Amz-Signature", signature)

	return fmt.Sprintf("%s%s?%s", s.cfg.Endpoint, canonicalURI, query.Encode()), nil
}

func (s *S3StorageProvider) HeadObject(ctx context.Context, key string) (*ObjectMeta, error) {
	targetURL := fmt.Sprintf("%s/%s/%s", s.cfg.Endpoint, s.cfg.Bucket, strings.TrimPrefix(key, "/"))
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, targetURL, nil)
	if err != nil {
		return nil, err
	}

	s.signRequest(req, []byte(""))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrObjectNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("head object failed with status %d", resp.StatusCode)
	}

	contentLength := resp.ContentLength
	contentType := resp.Header.Get("Content-Type")

	return &ObjectMeta{
		Key:         key,
		Size:        contentLength,
		ContentType: contentType,
	}, nil
}

func (s *S3StorageProvider) CopyObject(ctx context.Context, srcKey string, dstKey string) error {
	targetURL := fmt.Sprintf("%s/%s/%s", s.cfg.Endpoint, s.cfg.Bucket, strings.TrimPrefix(dstKey, "/"))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, targetURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("x-amz-copy-source", fmt.Sprintf("/%s/%s", s.cfg.Bucket, strings.TrimPrefix(srcKey, "/")))
	s.signRequest(req, []byte(""))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrObjectNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("copy object failed with status %d", resp.StatusCode)
	}

	return nil
}

func (s *S3StorageProvider) DeleteObject(ctx context.Context, key string) error {
	targetURL := fmt.Sprintf("%s/%s/%s", s.cfg.Endpoint, s.cfg.Bucket, strings.TrimPrefix(key, "/"))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, targetURL, nil)
	if err != nil {
		return err
	}

	s.signRequest(req, []byte(""))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("delete object failed with status %d", resp.StatusCode)
	}

	return nil
}

func (s *S3StorageProvider) signRequest(req *http.Request, payload []byte) {
	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	req.Header.Set("X-Amz-Date", amzDate)
	payloadHash := sha256Hex(payload)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, s.cfg.Region)
	canonicalURI := req.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	canonicalQueryString := req.URL.Query().Encode()
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		req.Host,
		payloadHash,
		amzDate,
	)
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	)

	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	)

	signingKey := getSignatureKey(s.cfg.SecretAccessKey, dateStamp, s.cfg.Region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.cfg.AccessKeyID,
		credentialScope,
		signedHeaders,
		signature,
	)
	req.Header.Set("Authorization", authHeader)
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key []byte, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func getSignatureKey(key string, dateStamp string, regionName string, serviceName string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+key), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(regionName))
	kService := hmacSHA256(kRegion, []byte(serviceName))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}
