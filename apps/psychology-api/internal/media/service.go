package media

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"psychology/api/internal/media/mediadb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	MaxPublicImageBytes int64 = 5 * 1024 * 1024        // 5 MiB
	MaxBlogAudioBytes   int64 = 32 * 1024 * 1024       // 32 MiB
	MaxBlogVideoBytes   int64 = 128 * 1024 * 1024      // 128 MiB
	MaxCourseAudioBytes int64 = 128 * 1024 * 1024      // 128 MiB
	MaxCourseVideoBytes int64 = 512 * 1024 * 1024      // 512 MiB
	TenantQuotaLimit    int64 = 10 * 1024 * 1024 * 1024 // 10 GiB
	StagingTTL                = 15 * time.Minute
	StreamURLTTL              = 5 * time.Minute
)

var (
	ErrQuotaExceeded       = errors.New("storage quota exceeded (10 GiB limit)")
	ErrInvalidAccessClass  = errors.New("invalid access class (must be public or course_private)")
	ErrInvalidMimeType     = errors.New("unsupported or disallowed MIME type")
	ErrFileSizeExceeded    = errors.New("file size exceeds maximum allowed for this media type")
	ErrInvalidFileSize     = errors.New("file size must be greater than zero")
	ErrUploadNotFound      = errors.New("media upload reservation not found")
	ErrUploadExpired       = errors.New("media upload reservation has expired")
	ErrUploadNotReserved   = errors.New("media upload is not in reserved state")
	ErrMediaNotFound       = errors.New("media asset not found")
	ErrMediaNotPublic      = errors.New("media asset is not public")
	ErrMediaNotVerified    = errors.New("media asset is not verified")
	ErrStorageMimeMismatch = errors.New("uploaded file MIME type does not match reservation")
	ErrStorageSizeMismatch = errors.New("uploaded file size exceeds reservation")
)

type MediaQuerier interface {
	CreateMediaUpload(ctx context.Context, arg mediadb.CreateMediaUploadParams) (mediadb.MediaUpload, error)
	GetMediaUploadByID(ctx context.Context, arg mediadb.GetMediaUploadByIDParams) (mediadb.MediaUpload, error)
	GetMediaUploadByStagingKey(ctx context.Context, arg mediadb.GetMediaUploadByStagingKeyParams) (mediadb.MediaUpload, error)
	MarkMediaUploadCompleted(ctx context.Context, arg mediadb.MarkMediaUploadCompletedParams) (mediadb.MediaUpload, error)
	MarkMediaUploadExpired(ctx context.Context, arg mediadb.MarkMediaUploadExpiredParams) error
	ListExpiredMediaUploads(ctx context.Context, arg mediadb.ListExpiredMediaUploadsParams) ([]mediadb.MediaUpload, error)
	GetTotalMediaUsageBytes(ctx context.Context, arg mediadb.GetTotalMediaUsageBytesParams) (int64, error)
	CreateMediaAsset(ctx context.Context, arg mediadb.CreateMediaAssetParams) (mediadb.MediaAsset, error)
	GetMediaAssetByID(ctx context.Context, arg mediadb.GetMediaAssetByIDParams) (mediadb.MediaAsset, error)
	GetMediaAssetByStorageKey(ctx context.Context, arg mediadb.GetMediaAssetByStorageKeyParams) (mediadb.MediaAsset, error)
	ListMediaAssetsByTenant(ctx context.Context, tenantID pgtype.UUID) ([]mediadb.MediaAsset, error)
}

type Service struct {
	db      MediaQuerier
	storage StorageProvider
}

func NewService(db MediaQuerier, storage StorageProvider) *Service {
	return &Service{
		db:      db,
		storage: storage,
	}
}

type RequestUploadInput struct {
	OriginalName string
	MimeType     string
	SizeBytes    int64
	AccessClass  string
}

type RequestUploadOutput struct {
	UploadID   uuid.UUID
	UploadURL  string
	StagingKey string
	ExpiresAt  time.Time
}

type FinalizeUploadInput struct {
	UploadID uuid.UUID
}

type MediaAssetOutput struct {
	ID               uuid.UUID `json:"id"`
	StorageKey       string    `json:"storage_key"`
	OriginalName     string    `json:"original_name"`
	MimeType         string    `json:"mime_type"`
	SizeBytes        int64     `json:"size_bytes"`
	AccessClass      string    `json:"access_class"`
	ValidationStatus string    `json:"validation_status"`
	CreatedAt        time.Time `json:"created_at"`
}

type MediaListOutput struct {
	Items           []MediaAssetOutput `json:"items"`
	QuotaUsedBytes  int64              `json:"quota_used_bytes"`
	QuotaTotalBytes int64              `json:"quota_total_bytes"`
}

type StreamURLOutput struct {
	StreamURL        string `json:"stream_url"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

// ValidateFileSpecs validates the MIME type and file size against the access class and type limits.
func ValidateFileSpecs(accessClass string, mimeType string, sizeBytes int64) error {
	if sizeBytes <= 0 {
		return ErrInvalidFileSize
	}

	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	switch accessClass {
	case "public":
		switch mimeType {
		case "image/jpeg", "image/png", "image/webp":
			if sizeBytes > MaxPublicImageBytes {
				return ErrFileSizeExceeded
			}
		case "audio/mpeg":
			if sizeBytes > MaxBlogAudioBytes {
				return ErrFileSizeExceeded
			}
		case "video/mp4":
			if sizeBytes > MaxBlogVideoBytes {
				return ErrFileSizeExceeded
			}
		default:
			return ErrInvalidMimeType
		}
	case "course_private":
		switch mimeType {
		case "audio/mpeg":
			if sizeBytes > MaxCourseAudioBytes {
				return ErrFileSizeExceeded
			}
		case "video/mp4":
			if sizeBytes > MaxCourseVideoBytes {
				return ErrFileSizeExceeded
			}
		default:
			return ErrInvalidMimeType
		}
	default:
		return ErrInvalidAccessClass
	}

	return nil
}

func sanitizeFilename(name string) string {
	base := filepath.Base(filepath.Clean(name))
	base = strings.ReplaceAll(base, " ", "_")
	base = strings.ReplaceAll(base, "/", "")
	base = strings.ReplaceAll(base, "\\", "")
	base = strings.ReplaceAll(base, "\x00", "")
	if base == "" || base == "." {
		return "file"
	}
	return base
}

func (s *Service) RequestStagingUpload(ctx context.Context, tenantID, userID uuid.UUID, input RequestUploadInput) (*RequestUploadOutput, error) {
	if err := ValidateFileSpecs(input.AccessClass, input.MimeType, input.SizeBytes); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var tid pgtype.UUID
	_ = tid.Scan(tenantID.String())

	var nowPg pgtype.Timestamptz
	_ = nowPg.Scan(now)

	totalUsed, err := s.db.GetTotalMediaUsageBytes(ctx, mediadb.GetTotalMediaUsageBytesParams{
		TenantID:  tid,
		ExpiresAt: nowPg,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to check media usage: %w", err)
	}

	if totalUsed+input.SizeBytes > TenantQuotaLimit {
		return nil, ErrQuotaExceeded
	}

	uploadID := uuid.New()
	safeFilename := sanitizeFilename(input.OriginalName)
	stagingKey := fmt.Sprintf("staging/%s/%s_%s", tenantID.String(), uploadID.String(), safeFilename)
	expiresAt := now.Add(StagingTTL)

	uploadURL, err := s.storage.PresignPutURL(ctx, stagingKey, input.MimeType, input.SizeBytes, StagingTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate presigned upload url: %w", err)
	}

	var uid pgtype.UUID
	_ = uid.Scan(userID.String())

	var upID pgtype.UUID
	_ = upID.Scan(uploadID.String())

	var expPg pgtype.Timestamptz
	_ = expPg.Scan(expiresAt)

	_, err = s.db.CreateMediaUpload(ctx, mediadb.CreateMediaUploadParams{
		ID:                upID,
		TenantID:          tid,
		StagingKey:        stagingKey,
		OriginalName:      safeFilename,
		AccessClass:       input.AccessClass,
		ExpectedMimeType:  input.MimeType,
		ExpectedSizeBytes: input.SizeBytes,
		ExpiresAt:         expPg,
		InitiatedBy:       uid,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to reserve media upload: %w", err)
	}

	return &RequestUploadOutput{
		UploadID:   uploadID,
		UploadURL:  uploadURL,
		StagingKey: stagingKey,
		ExpiresAt:  expiresAt,
	}, nil
}

func (s *Service) FinalizeUpload(ctx context.Context, tenantID, userID uuid.UUID, input FinalizeUploadInput) (*MediaAssetOutput, error) {
	var tid pgtype.UUID
	_ = tid.Scan(tenantID.String())

	var upID pgtype.UUID
	_ = upID.Scan(input.UploadID.String())

	upload, err := s.db.GetMediaUploadByID(ctx, mediadb.GetMediaUploadByIDParams{
		TenantID: tid,
		ID:       upID,
	})
	if err != nil {
		return nil, ErrUploadNotFound
	}

	if upload.Status != "reserved" {
		return nil, ErrUploadNotReserved
	}

	now := time.Now().UTC()
	if upload.ExpiresAt.Time.Before(now) {
		return nil, ErrUploadExpired
	}

	// Verify object exists in storage and inspect actual headers
	head, err := s.storage.HeadObject(ctx, upload.StagingKey)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect staging object: %w", err)
	}

	if head.Size <= 0 {
		return nil, ErrInvalidFileSize
	}
	if head.Size > upload.ExpectedSizeBytes {
		return nil, ErrStorageSizeMismatch
	}

	// Move object to immutable final destination
	assetID := uuid.New()
	finalKey := fmt.Sprintf("assets/%s/%s_%s", tenantID.String(), assetID.String(), upload.OriginalName)

	if err := s.storage.CopyObject(ctx, upload.StagingKey, finalKey); err != nil {
		return nil, fmt.Errorf("failed to copy object to final key: %w", err)
	}

	// Remove staging object
	_ = s.storage.DeleteObject(ctx, upload.StagingKey)

	// Mark reservation completed
	_, err = s.db.MarkMediaUploadCompleted(ctx, mediadb.MarkMediaUploadCompletedParams{
		TenantID: tid,
		ID:       upID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to complete upload reservation: %w", err)
	}

	// Insert verified media asset
	var aid pgtype.UUID
	_ = aid.Scan(assetID.String())

	var uid pgtype.UUID
	_ = uid.Scan(userID.String())

	var verifiedAtPg pgtype.Timestamptz
	_ = verifiedAtPg.Scan(now)

	asset, err := s.db.CreateMediaAsset(ctx, mediadb.CreateMediaAssetParams{
		TenantID:         tid,
		StorageKey:       finalKey,
		OriginalName:     upload.OriginalName,
		MimeType:         upload.ExpectedMimeType,
		SizeBytes:        head.Size,
		AccessClass:      upload.AccessClass,
		ValidationStatus: "verified",
		UploadedBy:       uid,
		VerifiedAt:       verifiedAtPg,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to record media asset: %w", err)
	}

	var parsedAssetID uuid.UUID
	copy(parsedAssetID[:], asset.ID.Bytes[:])

	return &MediaAssetOutput{
		ID:               parsedAssetID,
		StorageKey:       asset.StorageKey,
		OriginalName:     asset.OriginalName,
		MimeType:         asset.MimeType,
		SizeBytes:        asset.SizeBytes,
		AccessClass:      asset.AccessClass,
		ValidationStatus: asset.ValidationStatus,
		CreatedAt:        asset.CreatedAt.Time,
	}, nil
}

func (s *Service) GetPublicStreamURL(ctx context.Context, tenantID, assetID uuid.UUID) (*StreamURLOutput, error) {
	var tid pgtype.UUID
	_ = tid.Scan(tenantID.String())

	var aid pgtype.UUID
	_ = aid.Scan(assetID.String())

	asset, err := s.db.GetMediaAssetByID(ctx, mediadb.GetMediaAssetByIDParams{
		TenantID: tid,
		ID:       aid,
	})
	if err != nil {
		return nil, ErrMediaNotFound
	}

	if asset.ValidationStatus != "verified" {
		return nil, ErrMediaNotVerified
	}

	if asset.AccessClass != "public" {
		return nil, ErrMediaNotPublic
	}

	streamURL, err := s.storage.PresignGetURL(ctx, asset.StorageKey, StreamURLTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate stream url: %w", err)
	}

	return &StreamURLOutput{
		StreamURL:        streamURL,
		ExpiresInSeconds: int(StreamURLTTL.Seconds()),
	}, nil
}

func (s *Service) ListMedia(ctx context.Context, tenantID uuid.UUID) (*MediaListOutput, error) {
	var tid pgtype.UUID
	_ = tid.Scan(tenantID.String())

	assets, err := s.db.ListMediaAssetsByTenant(ctx, tid)
	if err != nil {
		return nil, fmt.Errorf("failed to list media assets: %w", err)
	}

	now := time.Now().UTC()
	var nowPg pgtype.Timestamptz
	_ = nowPg.Scan(now)

	totalUsed, err := s.db.GetTotalMediaUsageBytes(ctx, mediadb.GetTotalMediaUsageBytesParams{
		TenantID:  tid,
		ExpiresAt: nowPg,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to calculate quota: %w", err)
	}

	items := make([]MediaAssetOutput, len(assets))
	for i, a := range assets {
		var id uuid.UUID
		copy(id[:], a.ID.Bytes[:])
		items[i] = MediaAssetOutput{
			ID:               id,
			StorageKey:       a.StorageKey,
			OriginalName:     a.OriginalName,
			MimeType:         a.MimeType,
			SizeBytes:        a.SizeBytes,
			AccessClass:      a.AccessClass,
			ValidationStatus: a.ValidationStatus,
			CreatedAt:        a.CreatedAt.Time,
		}
	}

	return &MediaListOutput{
		Items:           items,
		QuotaUsedBytes:  totalUsed,
		QuotaTotalBytes: TenantQuotaLimit,
	}, nil
}

func (s *Service) PurgeExpiredUploads(ctx context.Context, limit int32) (int, error) {
	now := time.Now().UTC()
	var nowPg pgtype.Timestamptz
	_ = nowPg.Scan(now)

	expired, err := s.db.ListExpiredMediaUploads(ctx, mediadb.ListExpiredMediaUploadsParams{
		ExpiresAt: nowPg,
		Limit:     limit,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list expired uploads: %w", err)
	}

	purged := 0
	for _, up := range expired {
		// Clean up staging file in storage
		_ = s.storage.DeleteObject(ctx, up.StagingKey)

		// Mark status as expired in database
		if err := s.db.MarkMediaUploadExpired(ctx, mediadb.MarkMediaUploadExpiredParams{
			TenantID: up.TenantID,
			ID:       up.ID,
		}); err == nil {
			purged++
		}
	}

	return purged, nil
}
