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
	"psychology/api/internal/payments"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func toPgUUID(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

type mockCoursePaymentService struct {
	initiateFunc func(ctx context.Context, p payments.InitiateCoursePaymentParams) (*payments.InitiatePaymentResult, error)
}

func (m *mockCoursePaymentService) InitiateCoursePayment(ctx context.Context, p payments.InitiateCoursePaymentParams) (*payments.InitiatePaymentResult, error) {
	if m.initiateFunc != nil {
		return m.initiateFunc(ctx, p)
	}
	return &payments.InitiatePaymentResult{
		PaymentAttemptID: uuid.New(),
		PurchaseID:       uuid.New(),
		Authority:        "A0000000000000000000000000000001",
		PaymentURL:       "https://payment.zarinpal.com/pg/StartPay/A0000000000000000000000000000001",
		AmountRials:      12000000,
		Status:           "initiated",
	}, nil
}

func TestPublicCoursesAndCheckout_HTTP(t *testing.T) {
	tenantID := uuid.New()
	buyerID := uuid.New()
	courseID := uuid.New()

	q := newMockCoursesQuerier()
	domQ := &mockDomainQuerier{
		domains: map[string]uuid.UUID{
			"clinic.example.com": tenantID,
		},
	}
	resolver := domain.NewResolver(domQ)
	buyerSvc := courses.NewBuyerService(q, courses.NewMockOTPSender())
	adminSvc := courses.NewAdminCourseService(q, nil)

	// Seed buyer
	q.accounts[buyerID.String()] = coursesdb.BuyerAccount{
		ID:        toPgUUID(buyerID),
		TenantID:  toPgUUID(tenantID),
		Phone:     "09123456789",
		FullName:  "سارا عباسی",
		Status:    "active",
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	// Seed buyer session
	rawToken := "valid_buyer_session_token_123"
	tokenHash := courses.HashToken(rawToken)
	q.sessions[tokenHash] = coursesdb.BuyerSession{
		ID:        toPgUUID(uuid.New()),
		TenantID:  toPgUUID(tenantID),
		BuyerID:   toPgUUID(buyerID),
		TokenHash: tokenHash,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	}

	// Seed published course
	q.courses[courseID.String()] = coursesdb.Course{
		ID:             toPgUUID(courseID),
		TenantID:       toPgUUID(tenantID),
		Title:          "دوره مدیریت استرس",
		Slug:           "stress-management",
		Description:    "راهنمای کاربردی کنترل استرس",
		InstructorName: "دکتر احمدی",
		PriceRials:     12000000,
		Status:         "published",
		PublishedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		CreatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	// Seed a public lesson
	lessonID := uuid.New()
	q.lessons[lessonID.String()] = coursesdb.CourseLesson{
		ID:              toPgUUID(lessonID),
		TenantID:        toPgUUID(tenantID),
		CourseID:        toPgUUID(courseID),
		Title:           "مقدمه و تعریف استرس",
		Description:     "فایل مقدماتی",
		DisplayOrder:    1,
		DurationSeconds: 1200,
		IsFreePreview:   true,
		CreatedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	mockPaymentSvc := &mockCoursePaymentService{}
	handler := courses.NewHandler(buyerSvc, adminSvc, resolver, nil, mockPaymentSvc)

	r := chi.NewRouter()
	handler.MountRoutes(r)

	// 1. GET /public/courses -> Returns list with published course
	req1 := httptest.NewRequest("GET", "/public/courses", nil)
	req1.Host = "clinic.example.com"
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /public/courses, got: %d", rec1.Code)
	}

	var publicCourses []courses.PublicCourseItem
	if err := json.NewDecoder(rec1.Body).Decode(&publicCourses); err != nil {
		t.Fatalf("failed to decode public courses: %v", err)
	}
	if len(publicCourses) != 1 || publicCourses[0].Slug != "stress-management" {
		t.Errorf("expected 1 published course 'stress-management', got: %+v", publicCourses)
	}
	if publicCourses[0].PriceRials != 12000000 {
		t.Errorf("expected price 12000000 Rials, got: %d", publicCourses[0].PriceRials)
	}

	// 2. GET /public/courses/{slug} -> Returns course detail and syllabus
	req2 := httptest.NewRequest("GET", "/public/courses/stress-management", nil)
	req2.Host = "clinic.example.com"
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /public/courses/stress-management, got: %d", rec2.Code)
	}

	var detail courses.PublicCourseDetail
	if err := json.NewDecoder(rec2.Body).Decode(&detail); err != nil {
		t.Fatalf("failed to decode course detail: %v", err)
	}
	if detail.Title != "دوره مدیریت استرس" || len(detail.Lessons) != 1 {
		t.Errorf("unexpected course detail: %+v", detail)
	}
	if !detail.Lessons[0].IsFreePreview {
		t.Errorf("expected lesson to be free preview")
	}

	// 3. POST /public/courses/{slug}/checkout without auth -> 401 Unauthorized
	bodyCheckout, _ := json.Marshal(map[string]any{
		"idempotency_key": "chk_http_001",
		"callback_url":    "https://clinic.example.com/checkout/callback",
	})
	req3 := httptest.NewRequest("POST", "/public/courses/stress-management/checkout", bytes.NewReader(bodyCheckout))
	req3.Host = "clinic.example.com"
	req3.Header.Set("Content-Type", "application/json")
	rec3 := httptest.NewRecorder()
	r.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for unauthenticated checkout, got: %d", rec3.Code)
	}

	// 4. POST /public/courses/{slug}/checkout WITH valid buyer session cookie -> 200 OK
	req4 := httptest.NewRequest("POST", "/public/courses/stress-management/checkout", bytes.NewReader(bodyCheckout))
	req4.Host = "clinic.example.com"
	req4.Header.Set("Content-Type", "application/json")
	req4.AddCookie(&http.Cookie{
		Name:  "buyer_session_token",
		Value: rawToken,
	})
	rec4 := httptest.NewRecorder()
	r.ServeHTTP(rec4, req4)

	if rec4.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for authenticated checkout, got: %d (%s)", rec4.Code, rec4.Body.String())
	}

	var checkoutRes payments.InitiatePaymentResult
	if err := json.NewDecoder(rec4.Body).Decode(&checkoutRes); err != nil {
		t.Fatalf("failed to decode checkout result: %v", err)
	}
	if checkoutRes.PaymentURL == "" || checkoutRes.Authority == "" {
		t.Errorf("expected valid payment url and authority, got: %+v", checkoutRes)
	}
}
