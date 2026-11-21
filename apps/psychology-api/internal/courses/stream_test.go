package courses_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"psychology/api/internal/courses"
	"psychology/api/internal/courses/coursesdb"
	"psychology/api/internal/domain"
	"psychology/api/internal/media"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type mockMediaStreamSigner struct {
	signFunc func(ctx context.Context, tenantID, assetID uuid.UUID) (*media.StreamURLOutput, error)
}

func (m *mockMediaStreamSigner) GetCoursePrivateStreamURL(ctx context.Context, tenantID, assetID uuid.UUID) (*media.StreamURLOutput, error) {
	if m.signFunc != nil {
		return m.signFunc(ctx, tenantID, assetID)
	}
	return &media.StreamURLOutput{
		StreamURL:        "https://storage.example.com/assets/" + assetID.String() + "?token=sig_sec_12345",
		ExpiresInSeconds: 300, // 5 minutes
	}, nil
}

func setupStreamTestEnvironment(t *testing.T) (
	*courses.BuyerService,
	*courses.AdminCourseService,
	*domain.Resolver,
	*mockCoursesQuerier,
	*mockMediaStreamSigner,
	uuid.UUID, // tenantA
	uuid.UUID, // tenantB
	uuid.UUID, // buyerA
	uuid.UUID, // buyerB
	string,    // tokenBuyerA
	string,    // tokenBuyerB
	uuid.UUID, // courseID
	uuid.UUID, // lessonID
) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	buyerA := uuid.New()
	buyerB := uuid.New()
	courseID := uuid.New()
	lessonID := uuid.New()
	mediaAssetID := uuid.New()

	q := newMockCoursesQuerier()
	domQ := &mockDomainQuerier{
		domains: map[string]uuid.UUID{
			"clinic-a.example.com": tenantA,
			"clinic-b.example.com": tenantB,
		},
	}
	resolver := domain.NewResolver(domQ)

	signer := &mockMediaStreamSigner{}
	buyerSvc := courses.NewBuyerService(q, courses.NewMockOTPSender(), signer)
	adminSvc := courses.NewAdminCourseService(q, nil)

	// Seed Buyer A on Tenant A
	q.accounts[buyerA.String()] = coursesdb.BuyerAccount{
		ID:        toPgUUID(buyerA),
		TenantID:  toPgUUID(tenantA),
		Phone:     "09121111111",
		FullName:  "سارا احمدی",
		Status:    "active",
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	rawTokenA := "token_buyer_a_secret_111"
	q.sessions[courses.HashToken(rawTokenA)] = coursesdb.BuyerSession{
		ID:        toPgUUID(uuid.New()),
		TenantID:  toPgUUID(tenantA),
		BuyerID:   toPgUUID(buyerA),
		TokenHash: courses.HashToken(rawTokenA),
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	}

	// Seed Buyer B on Tenant A (another account in same clinic)
	q.accounts[buyerB.String()] = coursesdb.BuyerAccount{
		ID:        toPgUUID(buyerB),
		TenantID:  toPgUUID(tenantA),
		Phone:     "09122222222",
		FullName:  "علی رضایی",
		Status:    "active",
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	rawTokenB := "token_buyer_b_secret_222"
	q.sessions[courses.HashToken(rawTokenB)] = coursesdb.BuyerSession{
		ID:        toPgUUID(uuid.New()),
		TenantID:  toPgUUID(tenantA),
		BuyerID:   toPgUUID(buyerB),
		TokenHash: courses.HashToken(rawTokenB),
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	}

	// Seed Published Course on Tenant A
	q.courses[courseID.String()] = coursesdb.Course{
		ID:             toPgUUID(courseID),
		TenantID:       toPgUUID(tenantA),
		Title:          "دوره تنظیم هیجان",
		Slug:           "emotion-regulation",
		Description:    "روش‌های علمی مهار هیجان",
		InstructorName: "دکتر مریم علوی",
		PriceRials:     15000000,
		Status:         "published",
		PublishedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		CreatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	// Seed Lesson on Tenant A
	q.lessons[lessonID.String()] = coursesdb.CourseLesson{
		ID:              toPgUUID(lessonID),
		TenantID:        toPgUUID(tenantA),
		CourseID:        toPgUUID(courseID),
		Title:           "جلسه ۱: مبانی پردازش هیجانی",
		Description:     "فایل خصوصی جلسه اول",
		DisplayOrder:    1,
		MediaAssetID:    toPgUUID(mediaAssetID),
		DurationSeconds: 1500,
		IsFreePreview:   false,
		CreatedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	return buyerSvc, adminSvc, resolver, q, signer, tenantA, tenantB, buyerA, buyerB, rawTokenA, rawTokenB, courseID, lessonID
}

func TestLessonStreaming_Success_And_TTL(t *testing.T) {
	buyerSvc, adminSvc, resolver, q, _, tenantA, _, buyerA, _, tokenA, _, courseID, lessonID := setupStreamTestEnvironment(t)

	// Grant active entitlement to Buyer A
	entID := uuid.New()
	q.entitlements[entID.String()] = coursesdb.CourseEntitlement{
		ID:                  toPgUUID(entID),
		TenantID:            toPgUUID(tenantA),
		BuyerID:             toPgUUID(buyerA),
		CourseID:            toPgUUID(courseID),
		Status:              "active",
		CourseTitleSnapshot: "دوره تنظیم هیجان",
		PriceRialsSnapshot:  15000000,
		GrantedAt:           pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	handler := courses.NewHandler(buyerSvc, adminSvc, resolver, nil)
	r := chi.NewRouter()
	handler.MountRoutes(r)

	req := httptest.NewRequest("GET", "/account/courses/"+courseID.String()+"/lessons/"+lessonID.String()+"/stream", nil)
	req.Host = "clinic-a.example.com"
	req.AddCookie(&http.Cookie{
		Name:  "buyer_session_token",
		Value: tokenA,
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d (%s)", rec.Code, rec.Body.String())
	}

	// Verify Cache-Control header
	cacheCtrl := rec.Header().Get("Cache-Control")
	if cacheCtrl == "" || !bytes.Contains([]byte(cacheCtrl), []byte("no-store")) {
		t.Errorf("expected no-store Cache-Control header for private stream, got: %s", cacheCtrl)
	}

	var streamOut courses.LessonStreamOutput
	if err := json.NewDecoder(rec.Body).Decode(&streamOut); err != nil {
		t.Fatalf("failed to decode stream response: %v", err)
	}

	if streamOut.StreamURL == "" {
		t.Error("expected non-empty stream URL")
	}

	// Verify TTL max 5 minutes (300 seconds)
	now := time.Now()
	ttl := streamOut.ExpiresAt.Sub(now)
	if ttl <= 0 || ttl > (5*time.Minute+5*time.Second) {
		t.Errorf("expected stream URL TTL around 5 minutes, got: %v", ttl)
	}
	if streamOut.DurationSeconds != 1500 {
		t.Errorf("expected duration 1500 seconds, got: %d", streamOut.DurationSeconds)
	}
}

func TestLessonStreaming_UnpurchasedBlocked(t *testing.T) {
	buyerSvc, adminSvc, resolver, _, _, _, _, _, _, tokenA, _, courseID, lessonID := setupStreamTestEnvironment(t)

	// Buyer A has NOT purchased courseID (zero entitlements)
	handler := courses.NewHandler(buyerSvc, adminSvc, resolver, nil)
	r := chi.NewRouter()
	handler.MountRoutes(r)

	// 1. Authenticated buyer without entitlement -> 403 Forbidden
	req := httptest.NewRequest("GET", "/account/courses/"+courseID.String()+"/lessons/"+lessonID.String()+"/stream", nil)
	req.Host = "clinic-a.example.com"
	req.AddCookie(&http.Cookie{
		Name:  "buyer_session_token",
		Value: tokenA,
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for unpurchased course, got: %d (%s)", rec.Code, rec.Body.String())
	}

	// 2. Unauthenticated guest -> 401 Unauthorized
	reqGuest := httptest.NewRequest("GET", "/account/courses/"+courseID.String()+"/lessons/"+lessonID.String()+"/stream", nil)
	reqGuest.Host = "clinic-a.example.com"
	recGuest := httptest.NewRecorder()
	r.ServeHTTP(recGuest, reqGuest)

	if recGuest.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for guest stream request, got: %d", recGuest.Code)
	}
}

func TestLessonStreaming_OtherAccountBlocked(t *testing.T) {
	buyerSvc, adminSvc, resolver, q, _, tenantA, _, buyerA, _, _, tokenB, courseID, lessonID := setupStreamTestEnvironment(t)

	// Buyer A purchased the course
	entID := uuid.New()
	q.entitlements[entID.String()] = coursesdb.CourseEntitlement{
		ID:       toPgUUID(entID),
		TenantID: toPgUUID(tenantA),
		BuyerID:  toPgUUID(buyerA),
		CourseID: toPgUUID(courseID),
		Status:   "active",
	}

	handler := courses.NewHandler(buyerSvc, adminSvc, resolver, nil)
	r := chi.NewRouter()
	handler.MountRoutes(r)

	// Buyer B requests stream -> 403 Forbidden
	req := httptest.NewRequest("GET", "/account/courses/"+courseID.String()+"/lessons/"+lessonID.String()+"/stream", nil)
	req.Host = "clinic-a.example.com"
	req.AddCookie(&http.Cookie{
		Name:  "buyer_session_token",
		Value: tokenB,
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for other buyer account, got: %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestLessonStreaming_CrossTenantIsolation(t *testing.T) {
	buyerSvc, adminSvc, resolver, q, _, tenantA, _, buyerA, _, tokenA, _, courseID, lessonID := setupStreamTestEnvironment(t)

	// Buyer A has entitlement on Tenant A
	entID := uuid.New()
	q.entitlements[entID.String()] = coursesdb.CourseEntitlement{
		ID:       toPgUUID(entID),
		TenantID: toPgUUID(tenantA),
		BuyerID:  toPgUUID(buyerA),
		CourseID: toPgUUID(courseID),
		Status:   "active",
	}

	handler := courses.NewHandler(buyerSvc, adminSvc, resolver, nil)
	r := chi.NewRouter()
	handler.MountRoutes(r)

	// Request arrives targeted at Clinic B host domain with Buyer A token
	req := httptest.NewRequest("GET", "/account/courses/"+courseID.String()+"/lessons/"+lessonID.String()+"/stream", nil)
	req.Host = "clinic-b.example.com"
	req.AddCookie(&http.Cookie{
		Name:  "buyer_session_token",
		Value: tokenA,
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Tenant session check or course lookup must strictly fail
	if rec.Code == http.StatusOK {
		t.Errorf("cross-tenant request must NOT return 200 OK, got: %d", rec.Code)
	}
}

func TestLessonStreaming_RevokedEntitlementBlocked(t *testing.T) {
	buyerSvc, adminSvc, resolver, q, _, tenantA, _, buyerA, _, tokenA, _, courseID, lessonID := setupStreamTestEnvironment(t)

	// Entitlement is revoked (e.g. refund issued)
	entID := uuid.New()
	q.entitlements[entID.String()] = coursesdb.CourseEntitlement{
		ID:                  toPgUUID(entID),
		TenantID:            toPgUUID(tenantA),
		BuyerID:             toPgUUID(buyerA),
		CourseID:            toPgUUID(courseID),
		Status:              "revoked",
		RevocationReason:    pgtype.Text{String: "استرداد وجه به درخواست خریدار", Valid: true},
		RevokedAt:           pgtype.Timestamptz{Time: time.Now(), Valid: true},
		CourseTitleSnapshot: "دوره تنظیم هیجان",
	}

	handler := courses.NewHandler(buyerSvc, adminSvc, resolver, nil)
	r := chi.NewRouter()
	handler.MountRoutes(r)

	req := httptest.NewRequest("GET", "/account/courses/"+courseID.String()+"/lessons/"+lessonID.String()+"/stream", nil)
	req.Host = "clinic-a.example.com"
	req.AddCookie(&http.Cookie{
		Name:  "buyer_session_token",
		Value: tokenA,
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for revoked entitlement, got: %d (%s)", rec.Code, rec.Body.String())
	}

	var errBody map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&errBody)
	if errBody["code"] != "ENTITLEMENT_REVOKED" {
		t.Errorf("expected code ENTITLEMENT_REVOKED, got: %s", errBody["code"])
	}
}

func TestLessonStreaming_ArchivedCourse_ExistingBuyerKeepsAccess(t *testing.T) {
	buyerSvc, adminSvc, resolver, q, _, tenantA, _, buyerA, _, tokenA, _, courseID, lessonID := setupStreamTestEnvironment(t)

	// Buyer A already purchased the course
	entID := uuid.New()
	q.entitlements[entID.String()] = coursesdb.CourseEntitlement{
		ID:       toPgUUID(entID),
		TenantID: toPgUUID(tenantA),
		BuyerID:  toPgUUID(buyerA),
		CourseID: toPgUUID(courseID),
		Status:   "active",
	}

	// Clinic owner archives the course ("آرشیو دوره خرید تازه را می‌بندد و خریدار قبلی به جلسات موجود دسترسی دارد")
	c := q.courses[courseID.String()]
	c.Status = "archived"
	q.courses[courseID.String()] = c

	handler := courses.NewHandler(buyerSvc, adminSvc, resolver, nil)
	r := chi.NewRouter()
	handler.MountRoutes(r)

	req := httptest.NewRequest("GET", "/account/courses/"+courseID.String()+"/lessons/"+lessonID.String()+"/stream", nil)
	req.Host = "clinic-a.example.com"
	req.AddCookie(&http.Cookie{
		Name:  "buyer_session_token",
		Value: tokenA,
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected existing buyer to retain access to archived course lessons, got: %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestLessonStreaming_FreePreview_PublicAccess(t *testing.T) {
	buyerSvc, adminSvc, resolver, q, _, tenantA, _, _, _, _, _, courseID, _ := setupStreamTestEnvironment(t)

	// Lesson 1: Free Preview
	freeLessonID := uuid.New()
	q.lessons[freeLessonID.String()] = coursesdb.CourseLesson{
		ID:              toPgUUID(freeLessonID),
		TenantID:        toPgUUID(tenantA),
		CourseID:        toPgUUID(courseID),
		Title:           "جلسه پیش‌نمایش رایگان",
		DisplayOrder:    1,
		MediaAssetID:    toPgUUID(uuid.New()),
		DurationSeconds: 600,
		IsFreePreview:   true,
	}

	// Lesson 2: Locked
	lockedLessonID := uuid.New()
	q.lessons[lockedLessonID.String()] = coursesdb.CourseLesson{
		ID:              toPgUUID(lockedLessonID),
		TenantID:        toPgUUID(tenantA),
		CourseID:        toPgUUID(courseID),
		Title:           "جلسه قفل‌شده اصلی",
		DisplayOrder:    2,
		MediaAssetID:    toPgUUID(uuid.New()),
		DurationSeconds: 1200,
		IsFreePreview:   false,
	}

	handler := courses.NewHandler(buyerSvc, adminSvc, resolver, nil)
	r := chi.NewRouter()
	handler.MountRoutes(r)

	// 1. Request free preview -> 200 OK without any cookies/auth
	req1 := httptest.NewRequest("GET", "/public/courses/emotion-regulation/lessons/"+freeLessonID.String()+"/preview-stream", nil)
	req1.Host = "clinic-a.example.com"
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for free preview, got: %d (%s)", rec1.Code, rec1.Body.String())
	}

	var previewOut courses.LessonStreamOutput
	if err := json.NewDecoder(rec1.Body).Decode(&previewOut); err != nil {
		t.Fatalf("failed to decode preview stream: %v", err)
	}
	if previewOut.StreamURL == "" {
		t.Error("expected non-empty preview stream URL")
	}

	// 2. Request locked lesson via public preview endpoint -> 403 Forbidden
	req2 := httptest.NewRequest("GET", "/public/courses/emotion-regulation/lessons/"+lockedLessonID.String()+"/preview-stream", nil)
	req2.Host = "clinic-a.example.com"
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for locked lesson on preview endpoint, got: %d", rec2.Code)
	}
}
