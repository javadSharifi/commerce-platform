package testsupport_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"psychology/api/internal/auth"
	"psychology/api/internal/auth/authdb"
	"psychology/api/internal/courses"
	"psychology/api/internal/courses/coursesdb"
	"psychology/api/internal/domain"
	"psychology/api/internal/domain/domaindb"
	"psychology/api/internal/media"
	"psychology/api/internal/media/mediadb"
	"psychology/api/internal/payments"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type pilotMediaValidatorAdapter struct {
	store *pilotMediaStore
}

func (a *pilotMediaValidatorAdapter) GetMediaAsset(ctx context.Context, tenantID, mediaAssetID uuid.UUID) (*courses.MediaAssetInfo, error) {
	asset, err := a.store.GetMediaAssetByID(ctx, mediadb.GetMediaAssetByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(mediaAssetID),
	})
	if err != nil {
		return nil, err
	}
	return &courses.MediaAssetInfo{
		ID:               fromPgUUID(asset.ID),
		TenantID:         fromPgUUID(asset.TenantID),
		AccessClass:      asset.AccessClass,
		ValidationStatus: asset.ValidationStatus,
		MimeType:         asset.MimeType,
		SizeBytes:        asset.SizeBytes,
	}, nil
}

type pilotPhase3Fixture struct {
	tenantA uuid.UUID // کلینیک امید (omid-psych.ir)
	tenantB uuid.UUID // مرکز مشاوره ماهان (mahan-clinic.ir)

	hostA string
	hostB string

	ownerIDA uuid.UUID
	ownerIDB uuid.UUID

	tokenOwnerA string
	tokenOwnerB string

	router http.Handler

	coursesStore  *courses.MockCoursesQuerier
	paymentsStore *pilotPaymentsStore
	mediaStore    *pilotMediaStore
	storage       *media.MockStorageProvider
	gateway       *payments.MockPaymentProvider
	otpSender     *courses.MockOTPSender

	coursesHandler  *courses.Handler
	paymentsHandler *payments.Handler
	mediaHandler    *media.Handler

	buyerSvc *courses.BuyerService
	adminSvc *courses.AdminCourseService
	paySvc   *payments.Service
	mediaSvc *media.Service
}

// Setup Phase 3 Fixture with two isolated tenants
func setupPilotPhase3Fixture(t *testing.T) *pilotPhase3Fixture {
	tenantA := uuid.New()
	tenantB := uuid.New()

	hostA := "omid-psych.ir"
	hostB := "mahan-clinic.ir"

	domainStore := &pilotDomainStore{
		domains: map[string]domaindb.Domain{
			hostA: {TenantID: toPgUUID(tenantA), Host: hostA, IsVerified: true},
			hostB: {TenantID: toPgUUID(tenantB), Host: hostB, IsVerified: true},
		},
	}
	authStore := &pilotAuthStore{
		identities: make(map[string]authdb.PanelIdentity),
		sessions:   make(map[string]authdb.PanelSession),
		members:    make(map[string]authdb.Membership),
	}

	ownerAUser, _ := authStore.CreateUser("owner@omid.ir", "omidPass123", "دکتر امیدی (مالک)")
	authStore.AddMembership(ownerAUser.ID, tenantA, "owner")
	tokenOwnerA, _ := authStore.CreateSession(ownerAUser.ID)

	ownerBUser, _ := authStore.CreateUser("owner@mahan.ir", "mahanPass123", "دکتر ماهانی (مالک)")
	authStore.AddMembership(ownerBUser.ID, tenantB, "owner")
	tokenOwnerB, _ := authStore.CreateSession(ownerBUser.ID)

	domResolver := domain.NewResolver(domainStore)
	authSvc := auth.NewService(authStore)

	storage := media.NewMockStorageProvider()
	mediaStore := newPilotMediaStore()
	mediaSvc := media.NewService(mediaStore, storage)
	mediaValidator := &pilotMediaValidatorAdapter{store: mediaStore}

	coursesStore := courses.NewMockCoursesQuerier()
	otpSender := courses.NewMockOTPSender()
	buyerSvc := courses.NewBuyerService(coursesStore, otpSender, mediaSvc)
	adminSvc := courses.NewAdminCourseService(coursesStore, mediaValidator)

	gateway := payments.NewMockPaymentProvider()
	paymentsStore := newPilotPaymentsStore()
	workshopsStore := newPilotWorkshopsStore()
	paySvc := payments.NewService(paymentsStore, gateway, workshopsStore, coursesStore)

	coursesHandler := courses.NewHandler(buyerSvc, adminSvc, domResolver, authSvc)
	coursesHandler.SetPaymentService(paySvc)
	paymentsHandler := payments.NewHandler(paySvc, domResolver, authSvc)
	mediaHandler := media.NewHandler(mediaSvc, domResolver, authSvc)
	authHandler := auth.NewHandler(authSvc)

	r := chi.NewRouter()

	// Mount Routes
	authHandler.MountRoutes(r)
	coursesHandler.MountRoutes(r)
	paymentsHandler.MountRoutes(r)
	mediaHandler.MountRoutes(r)

	return &pilotPhase3Fixture{
		tenantA:         tenantA,
		tenantB:         tenantB,
		hostA:           hostA,
		hostB:           hostB,
		ownerIDA:        fromPgUUID(ownerAUser.ID),
		ownerIDB:        fromPgUUID(ownerBUser.ID),
		tokenOwnerA:     tokenOwnerA,
		tokenOwnerB:     tokenOwnerB,
		router:          r,
		coursesStore:    coursesStore,
		paymentsStore:   paymentsStore,
		mediaStore:      mediaStore,
		storage:         storage,
		gateway:         gateway,
		otpSender:       otpSender,
		coursesHandler:  coursesHandler,
		paymentsHandler: paymentsHandler,
		mediaHandler:    mediaHandler,
		buyerSvc:        buyerSvc,
		adminSvc:        adminSvc,
		paySvc:          paySvc,
		mediaSvc:        mediaSvc,
	}
}

// -------------------------------------------------------------------------------------------------
// Acceptance Test PSY-A08:
// خریدار پس از Verify یک دسترسی همان دوره می‌گیرد. Retry اثر تکراری ندارد.
// آرشیو جلوی فروش تازه را می‌گیرد و دسترسی خرید قبلی برقرار می‌ماند. بازپرداخت صدور URL تازه را متوقف می‌کند.
// -------------------------------------------------------------------------------------------------
func TestAcceptance_PSY_A08_CoursePurchaseToStreamingAndArchive(t *testing.T) {
	f := setupPilotPhase3Fixture(t)

	// Step 1: Admin in Tenant A uploads verified course_private media
	assetID := uuid.New()
	storageKey := fmt.Sprintf("assets/%s/%s_lesson1.mp4", f.tenantA.String(), assetID.String())
	f.mediaStore.assets[assetID.String()] = mediadb.MediaAsset{
		ID:               toPgUUID(assetID),
		TenantID:         toPgUUID(f.tenantA),
		StorageKey:       storageKey,
		OriginalName:     "cbt_stress_session1.mp4",
		MimeType:         "video/mp4",
		SizeBytes:        80 * 1024 * 1024,
		AccessClass:      "course_private",
		ValidationStatus: "verified",
	}
	f.storage.PutMockObject(storageKey, &media.ObjectMeta{
		Key:         storageKey,
		Size:        80 * 1024 * 1024,
		ContentType: "video/mp4",
	})

	// Step 2: Admin creates course and adds lesson
	courseID := uuid.New()
	f.coursesStore.Courses[courseID.String()] = coursesdb.Course{
		ID:             toPgUUID(courseID),
		TenantID:       toPgUUID(f.tenantA),
		Title:          "دوره مدیریت استرس و اضطراب",
		Slug:           "stress-management",
		Description:    "آموزش کاربردی درمان شناختی رفتاری",
		InstructorName: "دکتر مریم علوی",
		PriceRials:     5000000, // 500,000 Tomans
		Status:         "draft",
		CreatedAt:      pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}

	lessonID := uuid.New()
	f.coursesStore.Lessons[lessonID.String()] = coursesdb.CourseLesson{
		ID:              toPgUUID(lessonID),
		TenantID:        toPgUUID(f.tenantA),
		CourseID:        toPgUUID(courseID),
		Title:           "جلسه ۱: مکانیسم استرس و نشانه بالینی",
		DisplayOrder:    1,
		MediaAssetID:    toPgUUID(assetID),
		DurationSeconds: 1800,
		IsFreePreview:   false,
	}

	// Step 3: Admin publishes the course (requires >= 1 lesson)
	pubReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/courses/%s/publish", courseID.String()), nil)
	pubReq.Host = f.hostA
	pubReq.Header.Set("Authorization", "Bearer "+f.tokenOwnerA)
	pubReq.Header.Set("X-Tenant-ID", f.tenantA.String())
	pubReq.Header.Set("Sec-Fetch-Site", "same-origin")
	pubReq.Header.Set("Content-Type", "application/json")
	pubReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	pubRec := httptest.NewRecorder()
	f.router.ServeHTTP(pubRec, pubReq)
	if pubRec.Code != http.StatusOK {
		t.Fatalf("failed to publish course: %d (%s)", pubRec.Code, pubRec.Body.String())
	}

	// Step 4: Buyer logs in via OTP in Tenant A
	buyerPhone := "09121112233"
	_, err := f.buyerSvc.SendOTP(context.Background(), f.tenantA, buyerPhone)
	if err != nil {
		t.Fatalf("failed to send OTP: %v", err)
	}
	rawCodeA := f.otpSender.GetLastCode(buyerPhone)

	verifyRes, err := f.buyerSvc.VerifyOTP(context.Background(), f.tenantA, buyerPhone, rawCodeA, "سارا احمدی")
	if err != nil {
		t.Fatalf("failed to verify OTP: %v", err)
	}
	buyerToken := verifyRes.SessionToken
	buyerID := verifyRes.Account.ID

	// Step 5: Buyer initiates checkout for course
	checkoutBody, _ := json.Marshal(map[string]string{
		"idempotency_key": "chk_phase3_" + uuid.New().String()[:8],
		"callback_url":    "https://" + f.hostA + "/callbacks/payments/zarinpal",
	})
	chkReq := httptest.NewRequest(http.MethodPost, "/public/courses/stress-management/checkout", bytes.NewReader(checkoutBody))
	chkReq.Host = f.hostA
	chkReq.Header.Set("Content-Type", "application/json")
	chkReq.AddCookie(&http.Cookie{
		Name:  "buyer_session_token",
		Value: buyerToken,
	})
	chkRec := httptest.NewRecorder()
	f.router.ServeHTTP(chkRec, chkReq)

	if chkRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for checkout, got %d: %s", chkRec.Code, chkRec.Body.String())
	}
	var chkOut payments.InitiatePaymentResult
	_ = json.Unmarshal(chkRec.Body.Bytes(), &chkOut)
	if chkOut.PaymentURL == "" {
		t.Fatal("expected non-empty payment URL")
	}

	// Step 6: Bank callback verification -> In same transaction, entitlement becomes active
	callbackReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/callbacks/payments/zarinpal?Authority=%s&Status=OK", chkOut.Authority), nil)
	callbackReq.Host = f.hostA
	callbackRec := httptest.NewRecorder()
	f.router.ServeHTTP(callbackRec, callbackReq)

	if callbackRec.Code != http.StatusFound {
		t.Fatalf("expected 302 Found for payment callback verify, got %d: %s", callbackRec.Code, callbackRec.Body.String())
	}

	// Verify buyer now has active entitlement
	activeEnt, err := f.coursesStore.GetActiveCourseEntitlement(context.Background(), coursesdb.GetActiveCourseEntitlementParams{
		TenantID: toPgUUID(f.tenantA),
		BuyerID:  toPgUUID(buyerID),
		CourseID: toPgUUID(courseID),
	})
	if err != nil || activeEnt.Status != "active" {
		t.Fatalf("expected active entitlement after verify, err: %v", err)
	}

	// Verify buyer can stream private lesson
	streamReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/account/courses/%s/lessons/%s/stream", courseID.String(), lessonID.String()), nil)
	streamReq.Host = f.hostA
	streamReq.AddCookie(&http.Cookie{
		Name:  "buyer_session_token",
		Value: buyerToken,
	})
	streamRec := httptest.NewRecorder()
	f.router.ServeHTTP(streamRec, streamReq)

	if streamRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for streaming after verify, got %d: %s", streamRec.Code, streamRec.Body.String())
	}
	var streamOut courses.LessonStreamOutput
	_ = json.Unmarshal(streamRec.Body.Bytes(), &streamOut)
	if streamOut.StreamURL == "" || !strings.Contains(streamOut.StreamURL, "ttl=300") {
		t.Fatalf("expected signed streaming URL with 5min TTL, got: %s", streamOut.StreamURL)
	}
	// Verify cache control header
	cc := streamRec.Header().Get("Cache-Control")
	if !strings.Contains(cc, "no-store") || !strings.Contains(cc, "private") {
		t.Errorf("expected private no-store cache control on streaming, got %s", cc)
	}

	// Step 7: Duplicate callback (Retry) has NO duplicate effect
	callbackRetryReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/callbacks/payments/zarinpal?Authority=%s&Status=OK", chkOut.Authority), nil)
	callbackRetryReq.Host = f.hostA
	callbackRetryRec := httptest.NewRecorder()
	f.router.ServeHTTP(callbackRetryRec, callbackRetryReq)

	if callbackRetryRec.Code != http.StatusFound {
		t.Errorf("expected 302 Found for duplicate callback retry, got %d", callbackRetryRec.Code)
	}
	// Verify count of entitlements for this buyer is still exactly 1
	entList, _ := f.coursesStore.ListBuyerCourseEntitlements(context.Background(), coursesdb.ListBuyerCourseEntitlementsParams{
		TenantID: toPgUUID(f.tenantA),
		BuyerID:  toPgUUID(buyerID),
	})
	if len(entList) != 1 {
		t.Fatalf("expected exactly 1 entitlement for buyer, got %d", len(entList))
	}

	// Step 8: Concurrent second purchase for already-entitled buyer is rejected
	secondCheckoutBody, _ := json.Marshal(map[string]string{
		"idempotency_key": "chk_second_" + uuid.New().String()[:8],
		"callback_url":    "https://" + f.hostA + "/callbacks/payments/zarinpal",
	})
	secondChkReq := httptest.NewRequest(http.MethodPost, "/public/courses/stress-management/checkout", bytes.NewReader(secondCheckoutBody))
	secondChkReq.Host = f.hostA
	secondChkReq.Header.Set("Content-Type", "application/json")
	secondChkReq.AddCookie(&http.Cookie{
		Name:  "buyer_session_token",
		Value: buyerToken,
	})
	secondChkRec := httptest.NewRecorder()
	f.router.ServeHTTP(secondChkRec, secondChkReq)

	if secondChkRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict when buying already-entitled course, got %d: %s", secondChkRec.Code, secondChkRec.Body.String())
	}

	// Step 9: Clinic owner archives the course
	arcReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/courses/%s/archive", courseID.String()), nil)
	arcReq.Host = f.hostA
	arcReq.Header.Set("Authorization", "Bearer "+f.tokenOwnerA)
	arcReq.Header.Set("X-Tenant-ID", f.tenantA.String())
	arcReq.Header.Set("Sec-Fetch-Site", "same-origin")
	arcReq.Header.Set("Content-Type", "application/json")
	arcReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	arcRec := httptest.NewRecorder()
	f.router.ServeHTTP(arcRec, arcReq)

	if arcRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for course archive, got %d", arcRec.Code)
	}

	// Verify new buyer cannot purchase archived course
	buyer2Phone := "09129998877"
	_, _ = f.buyerSvc.SendOTP(context.Background(), f.tenantA, buyer2Phone)
	rawCode2 := f.otpSender.GetLastCode(buyer2Phone)
	b2Verify, _ := f.buyerSvc.VerifyOTP(context.Background(), f.tenantA, buyer2Phone, rawCode2, "خریدار جدید")

	newBuyerCheckoutBody, _ := json.Marshal(map[string]string{
		"idempotency_key": "chk_new_buyer_" + uuid.New().String()[:8],
		"callback_url":    "https://" + f.hostA + "/callbacks/payments/zarinpal",
	})
	newBuyerChkReq := httptest.NewRequest(http.MethodPost, "/public/courses/stress-management/checkout", bytes.NewReader(newBuyerCheckoutBody))
	newBuyerChkReq.Host = f.hostA
	newBuyerChkReq.Header.Set("Content-Type", "application/json")
	newBuyerChkReq.AddCookie(&http.Cookie{
		Name:  "buyer_session_token",
		Value: b2Verify.SessionToken,
	})
	newBuyerChkRec := httptest.NewRecorder()
	f.router.ServeHTTP(newBuyerChkRec, newBuyerChkReq)

	if newBuyerChkRec.Code == http.StatusOK {
		t.Fatalf("expected purchase of archived course to be blocked, but got 200 OK")
	}

	// Verify existing buyer retains streaming access to archived course
	arcStreamReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/account/courses/%s/lessons/%s/stream", courseID.String(), lessonID.String()), nil)
	arcStreamReq.Host = f.hostA
	arcStreamReq.AddCookie(&http.Cookie{
		Name:  "buyer_session_token",
		Value: buyerToken,
	})
	arcStreamRec := httptest.NewRecorder()
	f.router.ServeHTTP(arcStreamRec, arcStreamReq)

	if arcStreamRec.Code != http.StatusOK {
		t.Fatalf("expected existing buyer to retain streaming access after course is archived, got %d", arcStreamRec.Code)
	}

	// Step 10: Refund recorded -> Entitlement is revoked and stops issuing new URLs
	purchaseID := fromPgUUID(activeEnt.PurchaseID)
	_, err = f.paySvc.RecordRefund(context.Background(), payments.RecordRefundParams{
		TenantID:        f.tenantA,
		PurchaseID:      purchaseID,
		ReferenceNumber: "ref_test_12345",
		Reason:          "استرداد وجه به درخواست خریدار",
		AmountRials:     5000000,
		RegisteredBy:    f.ownerIDA,
		Now:             time.Now(),
	})
	if err != nil {
		t.Fatalf("RecordRefund failed: %v", err)
	}

	// Attempting to stream now fails with 403 ENTITLEMENT_REVOKED
	revStreamReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/account/courses/%s/lessons/%s/stream", courseID.String(), lessonID.String()), nil)
	revStreamReq.Host = f.hostA
	revStreamReq.AddCookie(&http.Cookie{
		Name:  "buyer_session_token",
		Value: buyerToken,
	})
	revStreamRec := httptest.NewRecorder()
	f.router.ServeHTTP(revStreamRec, revStreamReq)

	if revStreamRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden after refund/revocation, got %d (%s)", revStreamRec.Code, revStreamRec.Body.String())
	}
	var revErr map[string]string
	_ = json.Unmarshal(revStreamRec.Body.Bytes(), &revErr)
	if revErr["code"] != "ENTITLEMENT_REVOKED" {
		t.Errorf("expected code ENTITLEMENT_REVOKED, got %s", revErr["code"])
	}
}

// -------------------------------------------------------------------------------------------------
// Acceptance Test PSY-A09:
// کاربر مهمان، حساب دیگر و Tenant دیگر نمی‌توانند URL یا فایل خصوصی درس را بگیرند.
// URL کوتاه‌عمر با دسترسی معتبر تمدید می‌شود. مصرف خارج از سهمیه، URL مستقیم بدون امضا
// و استفادهٔ دوباره از کد بازیابی رد می‌شوند.
// -------------------------------------------------------------------------------------------------
func TestAcceptance_PSY_A09_StreamSecurityIsolationAndTokenRenewal(t *testing.T) {
	f := setupPilotPhase3Fixture(t)

	// Seed Course with Lesson in Tenant A
	assetID := uuid.New()
	storageKey := fmt.Sprintf("assets/%s/%s_lesson.mp4", f.tenantA.String(), assetID.String())
	f.mediaStore.assets[assetID.String()] = mediadb.MediaAsset{
		ID:               toPgUUID(assetID),
		TenantID:         toPgUUID(f.tenantA),
		StorageKey:       storageKey,
		OriginalName:     "cbt_lesson.mp4",
		MimeType:         "video/mp4",
		SizeBytes:        50 * 1024 * 1024,
		AccessClass:      "course_private",
		ValidationStatus: "verified",
	}
	f.storage.PutMockObject(storageKey, &media.ObjectMeta{
		Key:         storageKey,
		Size:        50 * 1024 * 1024,
		ContentType: "video/mp4",
	})

	courseID := uuid.New()
	f.coursesStore.Courses[courseID.String()] = coursesdb.Course{
		ID:             toPgUUID(courseID),
		TenantID:       toPgUUID(f.tenantA),
		Title:          "دوره تنظیم هیجان پیشرفته",
		Slug:           "emotion-regulation-adv",
		Description:    "مهارت‌های پیشرفته تنظیم هیجان",
		InstructorName: "دکتر مریم علوی",
		PriceRials:     8000000,
		Status:         "published",
		CreatedAt:      pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}

	// Lesson 1: Free Preview
	previewLessonID := uuid.New()
	f.coursesStore.Lessons[previewLessonID.String()] = coursesdb.CourseLesson{
		ID:              toPgUUID(previewLessonID),
		TenantID:        toPgUUID(f.tenantA),
		CourseID:        toPgUUID(courseID),
		Title:           "جلسه پیش‌نمایش رایگان",
		DisplayOrder:    1,
		MediaAssetID:    toPgUUID(assetID),
		DurationSeconds: 600,
		IsFreePreview:   true,
	}

	// Lesson 2: Private Locked Lesson
	privateLessonID := uuid.New()
	f.coursesStore.Lessons[privateLessonID.String()] = coursesdb.CourseLesson{
		ID:              toPgUUID(privateLessonID),
		TenantID:        toPgUUID(f.tenantA),
		CourseID:        toPgUUID(courseID),
		Title:           "جلسه دوم تخصصی",
		DisplayOrder:    2,
		MediaAssetID:    toPgUUID(assetID),
		DurationSeconds: 1500,
		IsFreePreview:   false,
	}

	// 1. Guest (unauthenticated) attempting to stream private lesson -> 401 Unauthorized
	guestReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/account/courses/%s/lessons/%s/stream", courseID.String(), privateLessonID.String()), nil)
	guestReq.Host = f.hostA
	guestRec := httptest.NewRecorder()
	f.router.ServeHTTP(guestRec, guestReq)

	if guestRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for guest streaming private lesson, got %d", guestRec.Code)
	}

	// 2. Guest accessing free preview lesson -> 200 OK without login
	prevReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/public/courses/emotion-regulation-adv/lessons/%s/preview-stream", previewLessonID.String()), nil)
	prevReq.Host = f.hostA
	prevRec := httptest.NewRecorder()
	f.router.ServeHTTP(prevRec, prevReq)

	if prevRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for free preview, got %d: %s", prevRec.Code, prevRec.Body.String())
	}
	var prevOut courses.LessonStreamOutput
	_ = json.Unmarshal(prevRec.Body.Bytes(), &prevOut)
	if prevOut.StreamURL == "" {
		t.Fatal("expected valid signed URL for free preview")
	}

	// 3. Guest attempting to access locked lesson on preview endpoint -> 403 Forbidden
	lockedReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/public/courses/emotion-regulation-adv/lessons/%s/preview-stream", privateLessonID.String()), nil)
	lockedReq.Host = f.hostA
	lockedRec := httptest.NewRecorder()
	f.router.ServeHTTP(lockedRec, lockedReq)

	if lockedRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when requesting locked lesson via preview-stream, got %d", lockedRec.Code)
	}

	// 4. Other buyer in Tenant A who has NOT purchased the course -> 403 NOT_PURCHASED
	phoneUnpurchased := "09123334455"
	_, _ = f.buyerSvc.SendOTP(context.Background(), f.tenantA, phoneUnpurchased)
	rawCodeUnpurchased := f.otpSender.GetLastCode(phoneUnpurchased)
	vUnpurchased, err := f.buyerSvc.VerifyOTP(context.Background(), f.tenantA, phoneUnpurchased, rawCodeUnpurchased, "کاربر بدون خرید")
	if err != nil || vUnpurchased == nil {
		t.Fatalf("failed to verify OTP for unpurchased buyer: %v", err)
	}

	noBuyReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/account/courses/%s/lessons/%s/stream", courseID.String(), privateLessonID.String()), nil)
	noBuyReq.Host = f.hostA
	noBuyReq.AddCookie(&http.Cookie{
		Name:  "buyer_session_token",
		Value: vUnpurchased.SessionToken,
	})
	noBuyRec := httptest.NewRecorder()
	f.router.ServeHTTP(noBuyRec, noBuyReq)

	if noBuyRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for non-purchased buyer, got %d: %s", noBuyRec.Code, noBuyRec.Body.String())
	}
	var errNoBuy map[string]string
	_ = json.Unmarshal(noBuyRec.Body.Bytes(), &errNoBuy)
	if errNoBuy["code"] != "FORBIDDEN" {
		t.Errorf("expected FORBIDDEN code, got %s", errNoBuy["code"])
	}

	// 5. Cross-Tenant Isolation: Tenant B attempting to access Tenant A's courses/lessons
	crossTenantReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/account/courses/%s/lessons/%s/stream", courseID.String(), privateLessonID.String()), nil)
	crossTenantReq.Host = f.hostB // Sending Tenant A course ID under Tenant B domain
	crossTenantReq.AddCookie(&http.Cookie{
		Name:  "buyer_session_token",
		Value: vUnpurchased.SessionToken,
	})
	crossTenantRec := httptest.NewRecorder()
	f.router.ServeHTTP(crossTenantRec, crossTenantReq)

	if crossTenantRec.Code == http.StatusOK {
		t.Fatalf("expected Cross-Tenant access to be blocked, but got 200 OK")
	}

	// 6. Registered Buyer with active purchase tests TTL and Renewal
	phoneBuyer := "09127778899"
	_, _ = f.buyerSvc.SendOTP(context.Background(), f.tenantA, phoneBuyer)
	rawCodeBuyer := f.otpSender.GetLastCode(phoneBuyer)
	vBuyer, err := f.buyerSvc.VerifyOTP(context.Background(), f.tenantA, phoneBuyer, rawCodeBuyer, "خریدار قطعی")
	if err != nil || vBuyer == nil {
		t.Fatalf("failed to verify OTP for buyer: %v", err)
	}

	// Grant active entitlement
	entID := uuid.New()
	f.coursesStore.Entitlements[entID.String()] = coursesdb.CourseEntitlement{
		ID:                  toPgUUID(entID),
		TenantID:            toPgUUID(f.tenantA),
		BuyerID:             toPgUUID(vBuyer.Account.ID),
		CourseID:            toPgUUID(courseID),
		Status:              "active",
		GrantedAt:           pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		CourseTitleSnapshot: "دوره تنظیم هیجان پیشرفته",
		PriceRialsSnapshot:  8000000,
	}

	// Issue first signed URL
	streamReq1 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/account/courses/%s/lessons/%s/stream", courseID.String(), privateLessonID.String()), nil)
	streamReq1.Host = f.hostA
	streamReq1.AddCookie(&http.Cookie{
		Name:  "buyer_session_token",
		Value: vBuyer.SessionToken,
	})
	streamRec1 := httptest.NewRecorder()
	f.router.ServeHTTP(streamRec1, streamReq1)

	if streamRec1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for entitled buyer stream, got %d", streamRec1.Code)
	}
	var streamOut1 courses.LessonStreamOutput
	_ = json.Unmarshal(streamRec1.Body.Bytes(), &streamOut1)

	// Verify short-lived TTL: exactly 300 seconds (5 minutes)
	if streamOut1.ExpiresInSeconds != 300 {
		t.Errorf("expected 300 seconds TTL, got %d", streamOut1.ExpiresInSeconds)
	}

	// Renewal: After URL expiration or seek, client requests fresh stream URL
	renewReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/account/courses/%s/lessons/%s/stream", courseID.String(), privateLessonID.String()), nil)
	renewReq.Host = f.hostA
	renewReq.AddCookie(&http.Cookie{
		Name:  "buyer_session_token",
		Value: vBuyer.SessionToken,
	})
	renewRec := httptest.NewRecorder()
	f.router.ServeHTTP(renewRec, renewReq)

	if renewRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for stream renewal, got %d", renewRec.Code)
	}
	var streamOut2 courses.LessonStreamOutput
	_ = json.Unmarshal(renewRec.Body.Bytes(), &streamOut2)
	if streamOut2.StreamURL == "" {
		t.Fatal("expected valid stream URL on renewal")
	}

	// 7. Single-Use OTP (Replay Prevention):
	// Using the already-used OTP code must be rejected with invalid OTP
	_, errReplay := f.buyerSvc.VerifyOTP(context.Background(), f.tenantA, phoneBuyer, rawCodeBuyer, "خریدار قطعی")
	if errReplay == nil {
		t.Fatalf("expected replay of used OTP code to be rejected, but succeeded")
	}

	// 8. Rejection of playlists (m3u8) and incompatible containers (mkv) on target bucket
	errPlaylist := media.ValidateMediaFile("course_private", "stream.m3u8", "video/mp4", 10*1024*1024)
	if errPlaylist != media.ErrDisallowedPlaylist {
		t.Errorf("expected ErrDisallowedPlaylist for .m3u8, got %v", errPlaylist)
	}

	errContainer := media.ValidateMediaFile("course_private", "recording.mkv", "video/mp4", 10*1024*1024)
	if errContainer != media.ErrIncompatibleMediaContainer {
		t.Errorf("expected ErrIncompatibleMediaContainer for .mkv, got %v", errContainer)
	}

	// 9. Quota Check: attempting upload that exceeds limits is rejected
	_, errQuota := f.mediaSvc.RequestStagingUpload(context.Background(), f.tenantA, uuid.New(), media.RequestUploadInput{
		OriginalName: "huge_video.mp4",
		MimeType:     "video/mp4",
		SizeBytes:    11 * 1024 * 1024 * 1024,
		AccessClass:  "course_private",
	})
	if errQuota == nil {
		t.Errorf("expected upload exceeding limits to return error")
	}
}
