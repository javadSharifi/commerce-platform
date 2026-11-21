package courses_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"psychology/api/internal/auth"
	"psychology/api/internal/auth/authdb"
	"psychology/api/internal/courses"
	"psychology/api/internal/domain"
	"psychology/api/internal/platform/tenantctx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type mockMediaValidator struct {
	assets map[string]*courses.MediaAssetInfo
}

func newMockMediaValidator() *mockMediaValidator {
	return &mockMediaValidator{
		assets: make(map[string]*courses.MediaAssetInfo),
	}
}

func (m *mockMediaValidator) GetMediaAsset(ctx context.Context, tenantID, mediaAssetID uuid.UUID) (*courses.MediaAssetInfo, error) {
	asset, ok := m.assets[mediaAssetID.String()]
	if !ok {
		return nil, courses.ErrLessonNotFound
	}
	return asset, nil
}

func (m *mockMediaValidator) addAsset(id, tenantID uuid.UUID, accessClass, validationStatus, mimeType string, sizeBytes int64) {
	m.assets[id.String()] = &courses.MediaAssetInfo{
		ID:               id,
		TenantID:         tenantID,
		AccessClass:      accessClass,
		ValidationStatus: validationStatus,
		MimeType:         mimeType,
		SizeBytes:        sizeBytes,
	}
}

type mockAuthQuerier struct {
	identities  map[string]authdb.PanelIdentity
	sessions    map[string]authdb.PanelSession
	memberships map[string]authdb.Membership
}

func newMockAuthQuerier() *mockAuthQuerier {
	return &mockAuthQuerier{
		identities:  make(map[string]authdb.PanelIdentity),
		sessions:    make(map[string]authdb.PanelSession),
		memberships: make(map[string]authdb.Membership),
	}
}

func (m *mockAuthQuerier) GetPanelIdentityByEmail(ctx context.Context, email string) (authdb.PanelIdentity, error) {
	for _, id := range m.identities {
		if id.Email == email {
			return id, nil
		}
	}
	return authdb.PanelIdentity{}, pgx.ErrNoRows
}

func (m *mockAuthQuerier) GetPanelIdentityByID(ctx context.Context, id pgtype.UUID) (authdb.PanelIdentity, error) {
	var u uuid.UUID
	copy(u[:], id.Bytes[:])
	idObj, ok := m.identities[u.String()]
	if !ok {
		return authdb.PanelIdentity{}, pgx.ErrNoRows
	}
	return idObj, nil
}

func (m *mockAuthQuerier) CreatePanelIdentity(ctx context.Context, arg authdb.CreatePanelIdentityParams) (authdb.PanelIdentity, error) {
	return authdb.PanelIdentity{}, nil
}

func (m *mockAuthQuerier) CreatePanelSession(ctx context.Context, arg authdb.CreatePanelSessionParams) (authdb.PanelSession, error) {
	m.sessions[arg.TokenHash] = authdb.PanelSession{
		ID:        arg.ID,
		UserID:    arg.UserID,
		TokenHash: arg.TokenHash,
		ExpiresAt: arg.ExpiresAt,
	}
	return m.sessions[arg.TokenHash], nil
}

func (m *mockAuthQuerier) GetPanelSessionByTokenHash(ctx context.Context, tokenHash string) (authdb.PanelSession, error) {
	s, ok := m.sessions[tokenHash]
	if !ok || s.ExpiresAt.Time.Before(time.Now()) {
		return authdb.PanelSession{}, pgx.ErrNoRows
	}
	return s, nil
}

func (m *mockAuthQuerier) DeletePanelSession(ctx context.Context, tokenHash string) error {
	delete(m.sessions, tokenHash)
	return nil
}

func (m *mockAuthQuerier) ListMembershipsByUserID(ctx context.Context, userID pgtype.UUID) ([]authdb.ListMembershipsByUserIDRow, error) {
	return nil, nil
}

func (m *mockAuthQuerier) GetMembershipByUserAndTenant(ctx context.Context, arg authdb.GetMembershipByUserAndTenantParams) (authdb.Membership, error) {
	for _, mem := range m.memberships {
		if mem.TenantID == arg.TenantID && mem.UserID == arg.UserID {
			return mem, nil
		}
	}
	return authdb.Membership{}, pgx.ErrNoRows
}

func (m *mockAuthQuerier) CreateMembership(ctx context.Context, arg authdb.CreateMembershipParams) (authdb.Membership, error) {
	return authdb.Membership{}, nil
}

func TestCourseLifecycle_Draft_Publish_Archive(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	q := newMockCoursesQuerier()
	mv := newMockMediaValidator()
	svc := courses.NewAdminCourseService(q, mv)

	// 1. Create Course (initial status should be draft)
	course, err := svc.CreateCourse(ctx, tenantID, courses.CreateCourseInput{
		Title:          "دوره جامع مدیریت استرس",
		Slug:           "stress-management",
		Description:    "آموزش کامل تکنیک‌های کنترل استرس و اضطراب",
		InstructorName: "دکتر مریم علوی",
		PriceRials:     25000000,
	})
	if err != nil {
		t.Fatalf("CreateCourse failed: %v", err)
	}
	if course.Status != "draft" {
		t.Fatalf("Expected initial status draft, got: %s", course.Status)
	}
	if course.LessonsCount != 0 {
		t.Fatalf("Expected 0 lessons initially, got: %d", course.LessonsCount)
	}

	// 2. Attempt to publish course with 0 lessons -> MUST fail
	_, err = svc.PublishCourse(ctx, tenantID, course.ID)
	if err != courses.ErrCannotPublishWithoutLessons {
		t.Fatalf("Expected ErrCannotPublishWithoutLessons, got: %v", err)
	}

	// 3. Add a valid course lesson with verified course_private media
	mediaID := uuid.New()
	mv.addAsset(mediaID, tenantID, "course_private", "verified", "video/mp4", 150*1024*1024)

	lesson, err := svc.CreateLesson(ctx, tenantID, course.ID, courses.CreateLessonInput{
		Title:           "درس اول: مقدمات فیزیولوژی استرس",
		Description:     "معرفی سیستم عصبی خودمختار",
		DisplayOrder:    1,
		MediaAssetID:    mediaID,
		DurationSeconds: 1800,
		IsFreePreview:   true,
	})
	if err != nil {
		t.Fatalf("CreateLesson failed: %v", err)
	}
	if lesson.MediaAssetID != mediaID {
		t.Fatalf("Unexpected lesson media ID: %s vs %s", lesson.MediaAssetID, mediaID)
	}

	// 4. Publish Course with lesson -> MUST succeed
	publishedCourse, err := svc.PublishCourse(ctx, tenantID, course.ID)
	if err != nil {
		t.Fatalf("PublishCourse failed: %v", err)
	}
	if publishedCourse.Status != "published" {
		t.Fatalf("Expected status published, got: %s", publishedCourse.Status)
	}
	if publishedCourse.PublishedAt == nil {
		t.Fatal("Expected non-nil PublishedAt timestamp after publishing")
	}
	if publishedCourse.LessonsCount != 1 {
		t.Fatalf("Expected 1 lesson, got: %d", publishedCourse.LessonsCount)
	}

	// 5. Archive Course -> MUST succeed
	archivedCourse, err := svc.ArchiveCourse(ctx, tenantID, course.ID)
	if err != nil {
		t.Fatalf("ArchiveCourse failed: %v", err)
	}
	if archivedCourse.Status != "archived" {
		t.Fatalf("Expected status archived, got: %s", archivedCourse.Status)
	}

	// 6. Attempt to publish archived course -> MUST fail
	_, err = svc.PublishCourse(ctx, tenantID, course.ID)
	if err != courses.ErrCourseArchived {
		t.Fatalf("Expected ErrCourseArchived, got: %v", err)
	}
}

func TestLessonMedia_StrictSeparationFromBlogMedia(t *testing.T) {
	ctx := context.Background()
	tenantA := uuid.New()
	tenantB := uuid.New()

	q := newMockCoursesQuerier()
	mv := newMockMediaValidator()
	svc := courses.NewAdminCourseService(q, mv)

	course, err := svc.CreateCourse(ctx, tenantA, courses.CreateCourseInput{
		Title:          "دوره فرزندپروری",
		Slug:           "parenting",
		Description:    "روش‌های ارتباط موثر با کودک",
		InstructorName: "دکتر کامران راد",
		PriceRials:     18000000,
	})
	if err != nil {
		t.Fatalf("CreateCourse failed: %v", err)
	}

	// Case 1: Attempt to attach a blog media asset (access_class = 'public') to course lesson
	publicBlogMediaID := uuid.New()
	mv.addAsset(publicBlogMediaID, tenantA, "public", "verified", "video/mp4", 50*1024*1024)

	_, err = svc.CreateLesson(ctx, tenantA, course.ID, courses.CreateLessonInput{
		Title:           "تلاش برای استفاده از رسانه بلاگ در درس",
		MediaAssetID:    publicBlogMediaID,
		DurationSeconds: 600,
	})
	if err != courses.ErrDisallowedMediaAccessClass {
		t.Fatalf("Expected ErrDisallowedMediaAccessClass for public blog media, got: %v", err)
	}

	// Case 2: Attempt to attach unverified media asset (validation_status = 'pending')
	unverifiedMediaID := uuid.New()
	mv.addAsset(unverifiedMediaID, tenantA, "course_private", "pending", "video/mp4", 100*1024*1024)

	_, err = svc.CreateLesson(ctx, tenantA, course.ID, courses.CreateLessonInput{
		Title:           "تلاش برای استفاده از فایل تایید نشده",
		MediaAssetID:    unverifiedMediaID,
		DurationSeconds: 1200,
	})
	if err != courses.ErrMediaNotVerified {
		t.Fatalf("Expected ErrMediaNotVerified, got: %v", err)
	}

	// Case 3: Attempt cross-tenant linking (Tenant B private course video attached to Tenant A course)
	tenantBMediaID := uuid.New()
	mv.addAsset(tenantBMediaID, tenantB, "course_private", "verified", "video/mp4", 120*1024*1024)

	_, err = svc.CreateLesson(ctx, tenantA, course.ID, courses.CreateLessonInput{
		Title:           "تلاش برای استفاده از فایل کلینیک دیگر",
		MediaAssetID:    tenantBMediaID,
		DurationSeconds: 900,
	})
	if err != courses.ErrMediaCrossTenantForbidden {
		t.Fatalf("Expected ErrMediaCrossTenantForbidden for cross-tenant media asset, got: %v", err)
	}

	// Case 4: Legitimate verified course_private media asset of the same tenant -> MUST succeed
	validPrivateMediaID := uuid.New()
	mv.addAsset(validPrivateMediaID, tenantA, "course_private", "verified", "video/mp4", 300*1024*1024)

	lesson, err := svc.CreateLesson(ctx, tenantA, course.ID, courses.CreateLessonInput{
		Title:           "درس اول: ارتباط غیرکلامی با کودک",
		MediaAssetID:    validPrivateMediaID,
		DurationSeconds: 1500,
		IsFreePreview:   false,
	})
	if err != nil {
		t.Fatalf("Valid lesson creation failed: %v", err)
	}
	if lesson.MediaAssetID != validPrivateMediaID {
		t.Fatalf("Unexpected media ID: %s", lesson.MediaAssetID)
	}
}

func TestAdminCourses_HTTP_Lifecycle_And_RBAC(t *testing.T) {
	tenantID := uuid.New()
	host := "clinic-test.ir"

	q := newMockCoursesQuerier()
	mv := newMockMediaValidator()
	svc := courses.NewAdminCourseService(q, mv)
	buyerSvc := courses.NewBuyerService(q, nil)

	// Auth Mock Setup
	authQ := newMockAuthQuerier()
	authSvc := auth.NewService(authQ)

	// 1. Create staff user & membership (role = owner)
	userID := uuid.New()
	userEmail := "admin@clinic.ir"
	authQ.identities[userID.String()] = authdb.PanelIdentity{
		ID:           pgtype.UUID{Bytes: userID, Valid: true},
		Email:        userEmail,
		PasswordHash: "hashed",
		FullName:     "مدیر کلینیک",
		Status:       "active",
	}
	memID := uuid.New()
	authQ.memberships[memID.String()] = authdb.Membership{
		ID:       pgtype.UUID{Bytes: memID, Valid: true},
		TenantID: pgtype.UUID{Bytes: tenantID, Valid: true},
		UserID:   pgtype.UUID{Bytes: userID, Valid: true},
		Role:     "owner",
	}

	// Session for user
	sessionToken := "admin-secret-token"
	sessionTokenHash := auth.HashToken(sessionToken)
	authQ.sessions[sessionTokenHash] = authdb.PanelSession{
		ID:        pgtype.UUID{Bytes: uuid.New(), Valid: true},
		UserID:    pgtype.UUID{Bytes: userID, Valid: true},
		TokenHash: sessionTokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	}

	domQ := &mockDomainQuerier{
		domains: map[string]uuid.UUID{
			host: tenantID,
		},
	}
	resolver := domain.NewResolver(domQ)

	handler := courses.NewHandler(buyerSvc, svc, resolver, authSvc)

	r := chi.NewRouter()
	// Middleware simulating tenant context attached by domain or membership
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := tenantctx.With(req.Context(), tenantID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	handler.MountRoutes(r)

	// 1. Unauthenticated request to /admin/courses -> MUST return 401
	unauthReq := httptest.NewRequest("GET", "/admin/courses", nil)
	unauthRec := httptest.NewRecorder()
	r.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 for unauthenticated request, got: %d", unauthRec.Code)
	}

	// 2. Create Course via HTTP POST with valid auth
	createBody, _ := json.Marshal(courses.CreateCourseInput{
		Title:          "دوره آزمایشی وبینار",
		Slug:           "webinar-course",
		Description:    "توضیحات دوره آزمایشی",
		InstructorName: "دکتر آزمایشی",
		PriceRials:     10000000,
	})
	req := httptest.NewRequest("POST", "/admin/courses", bytes.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+sessionToken)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /admin/courses code = %d, body = %s", rec.Code, rec.Body.String())
	}

	var created courses.AdminCourseItem
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("Parse created course response: %v", err)
	}
	if created.Status != "draft" {
		t.Fatalf("Expected draft status, got: %s", created.Status)
	}

	// 3. Add Lesson via HTTP POST
	mediaID := uuid.New()
	mv.addAsset(mediaID, tenantID, "course_private", "verified", "audio/mp3", 20*1024*1024)

	lessonBody, _ := json.Marshal(courses.CreateLessonInput{
		Title:           "جلسه صوتی اول",
		Description:     "فایل صوتی مقدمه",
		DisplayOrder:    1,
		MediaAssetID:    mediaID,
		DurationSeconds: 1200,
		IsFreePreview:   true,
	})
	req = httptest.NewRequest("POST", "/admin/courses/"+created.ID.String()+"/lessons", bytes.NewReader(lessonBody))
	req.Header.Set("Authorization", "Bearer "+sessionToken)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /admin/courses/:id/lessons code = %d, body = %s", rec.Code, rec.Body.String())
	}

	// 4. Publish Course via HTTP POST
	req = httptest.NewRequest("POST", "/admin/courses/"+created.ID.String()+"/publish", nil)
	req.Header.Set("Authorization", "Bearer "+sessionToken)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /admin/courses/:id/publish code = %d, body = %s", rec.Code, rec.Body.String())
	}

	var published courses.AdminCourseItem
	_ = json.Unmarshal(rec.Body.Bytes(), &published)
	if published.Status != "published" {
		t.Fatalf("Expected published status, got: %s", published.Status)
	}

	// 5. List Courses via HTTP GET
	req = httptest.NewRequest("GET", "/admin/courses", nil)
	req.Header.Set("Authorization", "Bearer "+sessionToken)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/courses code = %d, body = %s", rec.Code, rec.Body.String())
	}
	var list []courses.AdminCourseItem
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0].LessonsCount != 1 {
		t.Fatalf("Unexpected courses list response: %+v", list)
	}
}
