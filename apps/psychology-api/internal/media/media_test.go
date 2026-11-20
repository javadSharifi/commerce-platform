package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"psychology/api/internal/auth"
	"psychology/api/internal/auth/authdb"
	"psychology/api/internal/domain"
	"psychology/api/internal/domain/domaindb"
	"psychology/api/internal/media/mediadb"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func toPgUUID(u uuid.UUID) pgtype.UUID {
	var pu pgtype.UUID
	_ = pu.Scan(u.String())
	return pu
}

func fromPgUUID(pu pgtype.UUID) uuid.UUID {
	var u uuid.UUID
	copy(u[:], pu.Bytes[:])
	return u
}

type mockMediaQuerier struct {
	mu      sync.Mutex
	uploads map[string]mediadb.MediaUpload
	assets  map[string]mediadb.MediaAsset
}

func newMockMediaQuerier() *mockMediaQuerier {
	return &mockMediaQuerier{
		uploads: make(map[string]mediadb.MediaUpload),
		assets:  make(map[string]mediadb.MediaAsset),
	}
}

func (m *mockMediaQuerier) CreateMediaUpload(ctx context.Context, arg mediadb.CreateMediaUploadParams) (mediadb.MediaUpload, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fromPgUUID(arg.ID)
	if id == uuid.Nil {
		id = uuid.New()
	}
	tID := fromPgUUID(arg.TenantID)
	uID := fromPgUUID(arg.InitiatedBy)

	upload := mediadb.MediaUpload{
		ID:                  toPgUUID(id),
		TenantID:            arg.TenantID,
		StagingKey:          arg.StagingKey,
		OriginalName:        arg.OriginalName,
		AccessClass:         arg.AccessClass,
		ExpectedMimeType:    arg.ExpectedMimeType,
		ExpectedSizeBytes:   arg.ExpectedSizeBytes,
		ExpiresAt:           arg.ExpiresAt,
		Status:              "reserved",
		InitiatedBy:         toPgUUID(uID),
		CreatedAt:           pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}

	key := fmt.Sprintf("%s:%s", tID.String(), id.String())
	m.uploads[key] = upload
	return upload, nil
}

func (m *mockMediaQuerier) GetMediaUploadByID(ctx context.Context, arg mediadb.GetMediaUploadByIDParams) (mediadb.MediaUpload, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tID := fromPgUUID(arg.TenantID)
	uID := fromPgUUID(arg.ID)
	key := fmt.Sprintf("%s:%s", tID.String(), uID.String())

	upload, ok := m.uploads[key]
	if !ok {
		return mediadb.MediaUpload{}, pgx.ErrNoRows
	}
	return upload, nil
}

func (m *mockMediaQuerier) GetMediaUploadByStagingKey(ctx context.Context, arg mediadb.GetMediaUploadByStagingKeyParams) (mediadb.MediaUpload, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tID := fromPgUUID(arg.TenantID)
	for _, u := range m.uploads {
		if fromPgUUID(u.TenantID) == tID && u.StagingKey == arg.StagingKey {
			return u, nil
		}
	}
	return mediadb.MediaUpload{}, pgx.ErrNoRows
}

func (m *mockMediaQuerier) MarkMediaUploadCompleted(ctx context.Context, arg mediadb.MarkMediaUploadCompletedParams) (mediadb.MediaUpload, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tID := fromPgUUID(arg.TenantID)
	uID := fromPgUUID(arg.ID)
	key := fmt.Sprintf("%s:%s", tID.String(), uID.String())

	upload, ok := m.uploads[key]
	if !ok || upload.Status != "reserved" {
		return mediadb.MediaUpload{}, errors.New("upload not reserved")
	}

	upload.Status = "completed"
	m.uploads[key] = upload
	return upload, nil
}

func (m *mockMediaQuerier) MarkMediaUploadExpired(ctx context.Context, arg mediadb.MarkMediaUploadExpiredParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tID := fromPgUUID(arg.TenantID)
	uID := fromPgUUID(arg.ID)
	key := fmt.Sprintf("%s:%s", tID.String(), uID.String())

	upload, ok := m.uploads[key]
	if !ok {
		return pgx.ErrNoRows
	}

	upload.Status = "expired"
	m.uploads[key] = upload
	return nil
}

func (m *mockMediaQuerier) ListExpiredMediaUploads(ctx context.Context, arg mediadb.ListExpiredMediaUploadsParams) ([]mediadb.MediaUpload, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []mediadb.MediaUpload
	for _, u := range m.uploads {
		if u.Status == "reserved" && u.ExpiresAt.Time.Before(arg.ExpiresAt.Time) {
			result = append(result, u)
			if int32(len(result)) >= arg.Limit {
				break
			}
		}
	}
	return result, nil
}

func (m *mockMediaQuerier) GetTotalMediaUsageBytes(ctx context.Context, arg mediadb.GetTotalMediaUsageBytesParams) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tID := fromPgUUID(arg.TenantID)
	var total int64

	for _, a := range m.assets {
		if fromPgUUID(a.TenantID) == tID && a.ValidationStatus == "verified" {
			total += a.SizeBytes
		}
	}

	for _, u := range m.uploads {
		if fromPgUUID(u.TenantID) == tID && u.Status == "reserved" && u.ExpiresAt.Time.After(arg.ExpiresAt.Time) {
			total += u.ExpectedSizeBytes
		}
	}

	return total, nil
}

func (m *mockMediaQuerier) CreateMediaAsset(ctx context.Context, arg mediadb.CreateMediaAssetParams) (mediadb.MediaAsset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New()
	tID := fromPgUUID(arg.TenantID)

	for _, a := range m.assets {
		if a.StorageKey == arg.StorageKey {
			return mediadb.MediaAsset{}, errors.New("unique violation: storage_key exists")
		}
	}

	asset := mediadb.MediaAsset{
		ID:               toPgUUID(id),
		TenantID:         arg.TenantID,
		StorageKey:       arg.StorageKey,
		OriginalName:     arg.OriginalName,
		MimeType:         arg.MimeType,
		SizeBytes:        arg.SizeBytes,
		AccessClass:      arg.AccessClass,
		ValidationStatus: arg.ValidationStatus,
		UploadedBy:       arg.UploadedBy,
		VerifiedAt:       arg.VerifiedAt,
		CreatedAt:        pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		UpdatedAt:        pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}

	key := fmt.Sprintf("%s:%s", tID.String(), id.String())
	m.assets[key] = asset
	return asset, nil
}

func (m *mockMediaQuerier) GetMediaAssetByID(ctx context.Context, arg mediadb.GetMediaAssetByIDParams) (mediadb.MediaAsset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tID := fromPgUUID(arg.TenantID)
	aID := fromPgUUID(arg.ID)
	key := fmt.Sprintf("%s:%s", tID.String(), aID.String())

	asset, ok := m.assets[key]
	if !ok {
		return mediadb.MediaAsset{}, pgx.ErrNoRows
	}
	return asset, nil
}

func (m *mockMediaQuerier) GetMediaAssetByStorageKey(ctx context.Context, arg mediadb.GetMediaAssetByStorageKeyParams) (mediadb.MediaAsset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tID := fromPgUUID(arg.TenantID)
	for _, a := range m.assets {
		if fromPgUUID(a.TenantID) == tID && a.StorageKey == arg.StorageKey {
			return a, nil
		}
	}
	return mediadb.MediaAsset{}, pgx.ErrNoRows
}

func (m *mockMediaQuerier) ListMediaAssetsByTenant(ctx context.Context, tenantID pgtype.UUID) ([]mediadb.MediaAsset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tID := fromPgUUID(tenantID)
	var list []mediadb.MediaAsset
	for _, a := range m.assets {
		if fromPgUUID(a.TenantID) == tID {
			list = append(list, a)
		}
	}
	return list, nil
}

type mockDomainQuerier struct {
	domains map[string]domaindb.Domain
}

func (m *mockDomainQuerier) GetDomainByHost(ctx context.Context, domainName string) (domaindb.Domain, error) {
	d, ok := m.domains[domainName]
	if !ok {
		return domaindb.Domain{}, pgx.ErrNoRows
	}
	return d, nil
}

type mockAuthQuerier struct {
	sessions    map[string]authdb.PanelSession
	users       map[string]authdb.PanelIdentity
	memberships map[string]authdb.Membership
}

func newMockAuthQuerier() *mockAuthQuerier {
	return &mockAuthQuerier{
		sessions:    make(map[string]authdb.PanelSession),
		users:       make(map[string]authdb.PanelIdentity),
		memberships: make(map[string]authdb.Membership),
	}
}

func (m *mockAuthQuerier) GetPanelSessionByTokenHash(ctx context.Context, tokenHash string) (authdb.PanelSession, error) {
	s, ok := m.sessions[tokenHash]
	if !ok {
		return authdb.PanelSession{}, pgx.ErrNoRows
	}
	return s, nil
}

func (m *mockAuthQuerier) GetPanelIdentityByID(ctx context.Context, id pgtype.UUID) (authdb.PanelIdentity, error) {
	var u uuid.UUID
	copy(u[:], id.Bytes[:])
	ident, ok := m.users[u.String()]
	if !ok {
		return authdb.PanelIdentity{}, pgx.ErrNoRows
	}
	return ident, nil
}

func (m *mockAuthQuerier) GetPanelIdentityByEmail(ctx context.Context, email string) (authdb.PanelIdentity, error) {
	return authdb.PanelIdentity{}, pgx.ErrNoRows
}
func (m *mockAuthQuerier) CreatePanelIdentity(ctx context.Context, arg authdb.CreatePanelIdentityParams) (authdb.PanelIdentity, error) {
	return authdb.PanelIdentity{}, nil
}
func (m *mockAuthQuerier) CreatePanelSession(ctx context.Context, arg authdb.CreatePanelSessionParams) (authdb.PanelSession, error) {
	return authdb.PanelSession{}, nil
}
func (m *mockAuthQuerier) DeletePanelSession(ctx context.Context, tokenHash string) error {
	delete(m.sessions, tokenHash)
	return nil
}
func (m *mockAuthQuerier) ListMembershipsByUserID(ctx context.Context, userID pgtype.UUID) ([]authdb.ListMembershipsByUserIDRow, error) {
	return nil, nil
}
func (m *mockAuthQuerier) GetMembershipByUserAndTenant(ctx context.Context, arg authdb.GetMembershipByUserAndTenantParams) (authdb.Membership, error) {
	var u, t uuid.UUID
	copy(u[:], arg.UserID.Bytes[:])
	copy(t[:], arg.TenantID.Bytes[:])
	key := u.String() + ":" + t.String()
	mem, ok := m.memberships[key]
	if !ok {
		return authdb.Membership{}, pgx.ErrNoRows
	}
	return mem, nil
}
func (m *mockAuthQuerier) CreateMembership(ctx context.Context, arg authdb.CreateMembershipParams) (authdb.Membership, error) {
	return authdb.Membership{}, nil
}

// ----------------- TESTS -----------------

func TestValidateFileSpecs(t *testing.T) {
	tests := []struct {
		name        string
		accessClass string
		mimeType    string
		sizeBytes   int64
		wantErr     error
	}{
		{"Valid JPEG image", "public", "image/jpeg", 2 * 1024 * 1024, nil},
		{"Valid PNG image", "public", "image/png", 5 * 1024 * 1024, nil},
		{"Valid WebP image", "public", "image/webp", 4 * 1024 * 1024, nil},
		{"Image exceeding 5 MiB", "public", "image/png", 5*1024*1024 + 1, ErrFileSizeExceeded},
		{"Disallowed executable image", "public", "image/svg+xml", 1024, ErrInvalidMimeType},

		{"Valid Blog MP3", "public", "audio/mpeg", 20 * 1024 * 1024, nil},
		{"Blog MP3 exceeding 32 MiB", "public", "audio/mpeg", 32*1024*1024 + 1, ErrFileSizeExceeded},

		{"Valid Blog Video MP4", "public", "video/mp4", 100 * 1024 * 1024, nil},
		{"Blog Video MP4 exceeding 128 MiB", "public", "video/mp4", 128*1024*1024 + 1, ErrFileSizeExceeded},

		{"Valid Course Audio MP3", "course_private", "audio/mpeg", 100 * 1024 * 1024, nil},
		{"Course Audio MP3 exceeding 128 MiB", "course_private", "audio/mpeg", 128*1024*1024 + 1, ErrFileSizeExceeded},

		{"Valid Course Video MP4", "course_private", "video/mp4", 450 * 1024 * 1024, nil},
		{"Course Video MP4 exceeding 512 MiB", "course_private", "video/mp4", 512*1024*1024 + 1, ErrFileSizeExceeded},

		{"Disallowed image in course_private", "course_private", "image/png", 1024, ErrInvalidMimeType},
		{"Zero byte file", "public", "image/png", 0, ErrInvalidFileSize},
		{"Negative byte file", "public", "image/png", -10, ErrInvalidFileSize},
		{"Unknown access class", "confidential", "image/png", 1024, ErrInvalidAccessClass},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFileSpecs(tt.accessClass, tt.mimeType, tt.sizeBytes)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestStorageQuotaEnforcement(t *testing.T) {
	db := newMockMediaQuerier()
	storage := NewMockStorageProvider()
	svc := NewService(db, storage)

	tenantID := uuid.New()
	userID := uuid.New()

	// Seed existing assets totaling 9.95 GiB (50 MiB remaining)
	db.assets["asset-1"] = mediadb.MediaAsset{
		ID:               toPgUUID(uuid.New()),
		TenantID:         toPgUUID(tenantID),
		StorageKey:       "assets/tenant/bigfile.mp4",
		SizeBytes:        10*1024*1024*1024 - 50*1024*1024, // 50 MiB remaining
		ValidationStatus: "verified",
	}

	// 1. Requesting 30 MiB should succeed
	res, err := svc.RequestStagingUpload(context.Background(), tenantID, userID, RequestUploadInput{
		OriginalName: "sample.mp3",
		MimeType:     "audio/mpeg",
		SizeBytes:    30 * 1024 * 1024, // 30 MiB
		AccessClass:  "public",
	})
	if err != nil {
		t.Fatalf("expected upload reservation to succeed, got: %v", err)
	}
	if res.UploadID == uuid.Nil || res.UploadURL == "" {
		t.Fatalf("expected valid upload output")
	}

	// 2. Requesting another 30 MiB should fail because 9.95 GiB + 30 MiB (reserved) + 30 MiB > 10 GiB
	_, err = svc.RequestStagingUpload(context.Background(), tenantID, userID, RequestUploadInput{
		OriginalName: "podcast.mp3",
		MimeType:     "audio/mpeg",
		SizeBytes:    30 * 1024 * 1024,
		AccessClass:  "public",
	})
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded, got: %v", err)
	}
}

func TestStagingUploadAndFinalization(t *testing.T) {
	db := newMockMediaQuerier()
	storage := NewMockStorageProvider()
	svc := NewService(db, storage)

	tenantA := uuid.New()
	userA := uuid.New()

	// 1. Request staging upload
	reqRes, err := svc.RequestStagingUpload(context.Background(), tenantA, userA, RequestUploadInput{
		OriginalName: "dr_speech.mp4",
		MimeType:     "video/mp4",
		SizeBytes:    50 * 1024 * 1024,
		AccessClass:  "public",
	})
	if err != nil {
		t.Fatalf("failed to request staging upload: %v", err)
	}

	// Simulate browser PUT into storage
	storage.PutMockObject(reqRes.StagingKey, &ObjectMeta{
		Key:         reqRes.StagingKey,
		Size:        50 * 1024 * 1024,
		ContentType: "video/mp4",
	})

	// 2. Finalize upload
	asset, err := svc.FinalizeUpload(context.Background(), tenantA, userA, FinalizeUploadInput{
		UploadID: reqRes.UploadID,
	})
	if err != nil {
		t.Fatalf("failed to finalize upload: %v", err)
	}

	if asset.ValidationStatus != "verified" {
		t.Errorf("expected validation_status 'verified', got '%s'", asset.ValidationStatus)
	}
	if asset.OriginalName != "dr_speech.mp4" {
		t.Errorf("expected original name 'dr_speech.mp4', got '%s'", asset.OriginalName)
	}

	// Staging object must be deleted from storage
	if _, exists := storage.GetMockObject(reqRes.StagingKey); exists {
		t.Errorf("expected staging object to be deleted after finalization")
	}

	// Final asset must exist in storage
	if _, exists := storage.GetMockObject(asset.StorageKey); !exists {
		t.Errorf("expected final asset to exist in storage at %s", asset.StorageKey)
	}

	// Double finalization must fail
	_, err = svc.FinalizeUpload(context.Background(), tenantA, userA, FinalizeUploadInput{
		UploadID: reqRes.UploadID,
	})
	if !errors.Is(err, ErrUploadNotReserved) {
		t.Errorf("expected ErrUploadNotReserved on re-finalization, got: %v", err)
	}
}

func TestFileOverwritePreventionAndImmutability(t *testing.T) {
	db := newMockMediaQuerier()
	storage := NewMockStorageProvider()
	svc := NewService(db, storage)

	tenantA := uuid.New()
	userA := uuid.New()

	// Upload first file with name "profile.png"
	req1, _ := svc.RequestStagingUpload(context.Background(), tenantA, userA, RequestUploadInput{
		OriginalName: "profile.png",
		MimeType:     "image/png",
		SizeBytes:    1024,
		AccessClass:  "public",
	})
	storage.PutMockObject(req1.StagingKey, &ObjectMeta{
		Key:         req1.StagingKey,
		Size:        1024,
		ContentType: "image/png",
	})
	asset1, err := svc.FinalizeUpload(context.Background(), tenantA, userA, FinalizeUploadInput{
		UploadID: req1.UploadID,
	})
	if err != nil {
		t.Fatalf("finalize 1 failed: %v", err)
	}

	// Upload second file with the exact SAME name "profile.png"
	req2, _ := svc.RequestStagingUpload(context.Background(), tenantA, userA, RequestUploadInput{
		OriginalName: "profile.png",
		MimeType:     "image/png",
		SizeBytes:    2048,
		AccessClass:  "public",
	})
	storage.PutMockObject(req2.StagingKey, &ObjectMeta{
		Key:         req2.StagingKey,
		Size:        2048,
		ContentType: "image/png",
	})
	asset2, err := svc.FinalizeUpload(context.Background(), tenantA, userA, FinalizeUploadInput{
		UploadID: req2.UploadID,
	})
	if err != nil {
		t.Fatalf("finalize 2 failed: %v", err)
	}

	// The two assets MUST have distinct storage keys and IDs (immutable storage)
	if asset1.ID == asset2.ID {
		t.Errorf("expected different asset IDs")
	}
	if asset1.StorageKey == asset2.StorageKey {
		t.Errorf("expected immutable separate storage keys, got same key %s", asset1.StorageKey)
	}

	// Both must exist independently in storage
	obj1, ok1 := storage.GetMockObject(asset1.StorageKey)
	obj2, ok2 := storage.GetMockObject(asset2.StorageKey)
	if !ok1 || !ok2 || obj1.Size != 1024 || obj2.Size != 2048 {
		t.Errorf("storage corruption or overwrite detected")
	}
}

func TestCrossTenantIsolation(t *testing.T) {
	db := newMockMediaQuerier()
	storage := NewMockStorageProvider()
	svc := NewService(db, storage)

	tenantA := uuid.New()
	tenantB := uuid.New()
	userA := uuid.New()
	userB := uuid.New()

	// Tenant A starts upload
	reqA, err := svc.RequestStagingUpload(context.Background(), tenantA, userA, RequestUploadInput{
		OriginalName: "confidential_interview.mp4",
		MimeType:     "video/mp4",
		SizeBytes:    30 * 1024 * 1024,
		AccessClass:  "course_private",
	})
	if err != nil {
		t.Fatalf("tenant A upload reservation failed: %v", err)
	}

	storage.PutMockObject(reqA.StagingKey, &ObjectMeta{
		Key:         reqA.StagingKey,
		Size:        30 * 1024 * 1024,
		ContentType: "video/mp4",
	})

	// 1. Tenant B tries to finalize Tenant A's upload -> MUST fail
	_, err = svc.FinalizeUpload(context.Background(), tenantB, userB, FinalizeUploadInput{
		UploadID: reqA.UploadID,
	})
	if !errors.Is(err, ErrUploadNotFound) {
		t.Errorf("expected ErrUploadNotFound when tenant B tries to finalize tenant A's upload, got: %v", err)
	}

	// Tenant A finalizes its own upload
	assetA, err := svc.FinalizeUpload(context.Background(), tenantA, userA, FinalizeUploadInput{
		UploadID: reqA.UploadID,
	})
	if err != nil {
		t.Fatalf("tenant A finalize failed: %v", err)
	}

	// 2. Tenant B tries to get stream URL for Tenant A's asset -> MUST fail
	_, err = svc.GetPublicStreamURL(context.Background(), tenantB, assetA.ID)
	if !errors.Is(err, ErrMediaNotFound) {
		t.Errorf("expected ErrMediaNotFound when tenant B requests tenant A's media, got: %v", err)
	}

	// 3. Tenant B lists media -> Tenant A's asset MUST NOT be present
	listB, err := svc.ListMedia(context.Background(), tenantB)
	if err != nil {
		t.Fatalf("tenant B list failed: %v", err)
	}
	if len(listB.Items) != 0 {
		t.Errorf("tenant B should see 0 items, found %d", len(listB.Items))
	}
}

func TestExpiredStagingCleanup(t *testing.T) {
	db := newMockMediaQuerier()
	storage := NewMockStorageProvider()
	svc := NewService(db, storage)

	tenantID := uuid.New()
	userID := uuid.New()

	// Insert an expired reserved upload in DB
	expiredID := uuid.New()
	stagingKey := fmt.Sprintf("staging/%s/%s_old.mp3", tenantID, expiredID)
	storage.PutMockObject(stagingKey, &ObjectMeta{
		Key:         stagingKey,
		Size:        10 * 1024 * 1024,
		ContentType: "audio/mpeg",
	})

	db.uploads[fmt.Sprintf("%s:%s", tenantID, expiredID)] = mediadb.MediaUpload{
		ID:                  toPgUUID(expiredID),
		TenantID:            toPgUUID(tenantID),
		StagingKey:          stagingKey,
		ExpectedSizeBytes:   10 * 1024 * 1024,
		ExpiresAt:           pgtype.Timestamptz{Time: time.Now().UTC().Add(-1 * time.Hour), Valid: true},
		Status:              "reserved",
		InitiatedBy:         toPgUUID(userID),
	}

	// Try to finalize expired upload -> fails
	_, err := svc.FinalizeUpload(context.Background(), tenantID, userID, FinalizeUploadInput{
		UploadID: expiredID,
	})
	if !errors.Is(err, ErrUploadExpired) {
		t.Errorf("expected ErrUploadExpired, got %v", err)
	}

	// Run purge worker
	purged, err := svc.PurgeExpiredUploads(context.Background(), 10)
	if err != nil {
		t.Fatalf("purge failed: %v", err)
	}
	if purged != 1 {
		t.Errorf("expected 1 purged item, got %d", purged)
	}

	// Storage object should be deleted
	if _, ok := storage.GetMockObject(stagingKey); ok {
		t.Errorf("expected staging file in storage to be purged")
	}

	// Database record should be marked expired
	up := db.uploads[fmt.Sprintf("%s:%s", tenantID, expiredID)]
	if up.Status != "expired" {
		t.Errorf("expected status 'expired', got %s", up.Status)
	}
}

func TestPublicMediaStreamingEndpoint(t *testing.T) {
	db := newMockMediaQuerier()
	storage := NewMockStorageProvider()
	svc := NewService(db, storage)

	tenantID := uuid.New()
	domainResolver := domain.NewResolver(&mockDomainQuerier{
		domains: map[string]domaindb.Domain{
			"clinic.ir": {
				ID:         toPgUUID(uuid.New()),
				TenantID:   toPgUUID(tenantID),
				Host:       "clinic.ir",
				IsVerified: true,
			},
		},
	})

	handler := NewHandler(svc, domainResolver, nil)
	r := chi.NewRouter()
	handler.MountRoutes(r)

	// Seed public verified media
	publicAssetID := uuid.New()
	storageKey := fmt.Sprintf("assets/%s/%s_intro.mp4", tenantID, publicAssetID)
	storage.PutMockObject(storageKey, &ObjectMeta{
		Key:         storageKey,
		Size:        5 * 1024 * 1024,
		ContentType: "video/mp4",
	})
	db.assets[fmt.Sprintf("%s:%s", tenantID, publicAssetID)] = mediadb.MediaAsset{
		ID:               toPgUUID(publicAssetID),
		TenantID:         toPgUUID(tenantID),
		StorageKey:       storageKey,
		OriginalName:     "intro.mp4",
		MimeType:         "video/mp4",
		SizeBytes:        5 * 1024 * 1024,
		AccessClass:      "public",
		ValidationStatus: "verified",
	}

	// Seed course_private verified media
	privateAssetID := uuid.New()
	privKey := fmt.Sprintf("assets/%s/%s_lesson.mp4", tenantID, privateAssetID)
	storage.PutMockObject(privKey, &ObjectMeta{
		Key:         privKey,
		Size:        50 * 1024 * 1024,
		ContentType: "video/mp4",
	})
	db.assets[fmt.Sprintf("%s:%s", tenantID, privateAssetID)] = mediadb.MediaAsset{
		ID:               toPgUUID(privateAssetID),
		TenantID:         toPgUUID(tenantID),
		StorageKey:       privKey,
		OriginalName:     "lesson.mp4",
		MimeType:         "video/mp4",
		SizeBytes:        50 * 1024 * 1024,
		AccessClass:      "course_private",
		ValidationStatus: "verified",
	}

	// 1. Request public stream URL
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/public/media/%s/stream-url", publicAssetID), nil)
	req.Host = "clinic.ir"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for public media stream, got %d: %s", rec.Code, rec.Body.String())
	}

	var streamRes StreamURLOutput
	if err := json.Unmarshal(rec.Body.Bytes(), &streamRes); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if streamRes.StreamURL == "" || streamRes.ExpiresInSeconds != 300 {
		t.Errorf("unexpected stream response: %+v", streamRes)
	}

	// 2. Request course_private media stream from public endpoint -> must be 403 Forbidden
	reqPriv := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/public/media/%s/stream-url", privateAssetID), nil)
	reqPriv.Host = "clinic.ir"
	recPriv := httptest.NewRecorder()
	r.ServeHTTP(recPriv, reqPriv)

	if recPriv.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for course_private media from public endpoint, got %d", recPriv.Code)
	}

	// 3. Request unknown media ID -> 404
	reqUnknown := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/public/media/%s/stream-url", uuid.New()), nil)
	reqUnknown.Host = "clinic.ir"
	recUnknown := httptest.NewRecorder()
	r.ServeHTTP(recUnknown, reqUnknown)

	if recUnknown.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for non-existent media, got %d", recUnknown.Code)
	}
}

func TestAdminMediaEndpointsAndRoles(t *testing.T) {
	db := newMockMediaQuerier()
	storage := NewMockStorageProvider()
	svc := NewService(db, storage)

	tenantID := uuid.New()
	ownerUserID := uuid.New()
	receptionistUserID := uuid.New()

	authQ := newMockAuthQuerier()
	authSvc := auth.NewService(authQ)

	// Owner session
	ownerToken := "session_owner_token"
	authQ.sessions[auth.HashToken(ownerToken)] = authdb.PanelSession{
		UserID:    toPgUUID(ownerUserID),
		TokenHash: auth.HashToken(ownerToken),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(time.Hour), Valid: true},
	}
	authQ.users[ownerUserID.String()] = authdb.PanelIdentity{
		ID:       toPgUUID(ownerUserID),
		FullName: "Dr Owner",
		Email:    "owner@clinic.ir",
		Status:   "active",
	}
	authQ.memberships[ownerUserID.String()+":"+tenantID.String()] = authdb.Membership{
		TenantID: toPgUUID(tenantID),
		UserID:   toPgUUID(ownerUserID),
		Role:     "owner",
	}

	// Receptionist session
	recepToken := "session_receptionist_token"
	authQ.sessions[auth.HashToken(recepToken)] = authdb.PanelSession{
		UserID:    toPgUUID(receptionistUserID),
		TokenHash: auth.HashToken(recepToken),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(time.Hour), Valid: true},
	}
	authQ.users[receptionistUserID.String()] = authdb.PanelIdentity{
		ID:       toPgUUID(receptionistUserID),
		FullName: "Receptionist",
		Email:    "recep@clinic.ir",
		Status:   "active",
	}
	authQ.memberships[receptionistUserID.String()+":"+tenantID.String()] = authdb.Membership{
		TenantID: toPgUUID(tenantID),
		UserID:   toPgUUID(receptionistUserID),
		Role:     "receptionist",
	}

	handler := NewHandler(svc, nil, authSvc)
	r := chi.NewRouter()
	handler.MountRoutes(r)

	// 1. Receptionist tries to request upload URL -> 403 Forbidden
	payload := map[string]any{
		"original_name": "poster.png",
		"mime_type":     "image/png",
		"size_bytes":    1024 * 1024,
		"access_class":  "public",
	}
	bodyBytes, _ := json.Marshal(payload)

	reqRecep := httptest.NewRequest(http.MethodPost, "/admin/media/upload-url", bytes.NewReader(bodyBytes))
	reqRecep.Header.Set("Content-Type", "application/json")
	reqRecep.Header.Set("Authorization", "Bearer "+recepToken)
	reqRecep.Header.Set("X-Tenant-ID", tenantID.String())
	reqRecep.Header.Set("X-Requested-With", "XMLHttpRequest")
	recRecep := httptest.NewRecorder()
	r.ServeHTTP(recRecep, reqRecep)

	if recRecep.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for receptionist, got %d", recRecep.Code)
	}

	// 2. Owner requests upload URL with cross-site header -> 403 Forbidden
	reqCsrf := httptest.NewRequest(http.MethodPost, "/admin/media/upload-url", bytes.NewReader(bodyBytes))
	reqCsrf.Header.Set("Content-Type", "application/json")
	reqCsrf.Header.Set("Authorization", "Bearer "+ownerToken)
	reqCsrf.Header.Set("X-Tenant-ID", tenantID.String())
	reqCsrf.Header.Set("Sec-Fetch-Site", "cross-site")
	recCsrf := httptest.NewRecorder()
	r.ServeHTTP(recCsrf, reqCsrf)

	if recCsrf.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden with Sec-Fetch-Site cross-site, got %d", recCsrf.Code)
	}

	// 3. Owner requests upload URL with CSRF header -> 201 Created
	reqOwner := httptest.NewRequest(http.MethodPost, "/admin/media/upload-url", bytes.NewReader(bodyBytes))
	reqOwner.Header.Set("Content-Type", "application/json")
	reqOwner.Header.Set("Authorization", "Bearer "+ownerToken)
	reqOwner.Header.Set("X-Tenant-ID", tenantID.String())
	reqOwner.Header.Set("X-Requested-With", "XMLHttpRequest")
	recOwner := httptest.NewRecorder()
	r.ServeHTTP(recOwner, reqOwner)

	if recOwner.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for owner, got %d: %s", recOwner.Code, recOwner.Body.String())
	}

	var uploadRes RequestUploadOutput
	_ = json.Unmarshal(recOwner.Body.Bytes(), &uploadRes)

	// Simulate upload
	storage.PutMockObject(uploadRes.StagingKey, &ObjectMeta{
		Key:         uploadRes.StagingKey,
		Size:        1024 * 1024,
		ContentType: "image/png",
	})

	// 4. Owner finalizes upload -> 200 OK
	finPayload := map[string]any{
		"upload_id": uploadRes.UploadID.String(),
	}
	finBytes, _ := json.Marshal(finPayload)
	reqFin := httptest.NewRequest(http.MethodPost, "/admin/media/finalize", bytes.NewReader(finBytes))
	reqFin.Header.Set("Content-Type", "application/json")
	reqFin.Header.Set("Authorization", "Bearer "+ownerToken)
	reqFin.Header.Set("X-Tenant-ID", tenantID.String())
	reqFin.Header.Set("X-Requested-With", "XMLHttpRequest")
	recFin := httptest.NewRecorder()
	r.ServeHTTP(recFin, reqFin)

	if recFin.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for finalize, got %d: %s", recFin.Code, recFin.Body.String())
	}

	// 5. Owner lists media -> 200 OK
	reqList := httptest.NewRequest(http.MethodGet, "/admin/media", nil)
	reqList.Header.Set("Authorization", "Bearer "+ownerToken)
	reqList.Header.Set("X-Tenant-ID", tenantID.String())
	recList := httptest.NewRecorder()
	r.ServeHTTP(recList, reqList)

	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for list media, got %d", recList.Code)
	}

	var listRes MediaListOutput
	_ = json.Unmarshal(recList.Body.Bytes(), &listRes)
	if len(listRes.Items) != 1 {
		t.Errorf("expected 1 media item, got %d", len(listRes.Items))
	}
	if listRes.QuotaUsedBytes != 1024*1024 {
		t.Errorf("expected 1048576 used bytes, got %d", listRes.QuotaUsedBytes)
	}
}
