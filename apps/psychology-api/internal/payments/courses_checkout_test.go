package payments_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"psychology/api/internal/courses/coursesdb"
	"psychology/api/internal/payments"
	"psychology/api/internal/payments/paymentsdb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type mockCoursePaymentQuerier struct {
	mu           sync.Mutex
	buyers       map[string]coursesdb.BuyerAccount
	courses      map[string]coursesdb.Course
	entitlements map[string]coursesdb.CourseEntitlement
}

func newMockCoursePaymentQuerier() *mockCoursePaymentQuerier {
	return &mockCoursePaymentQuerier{
		buyers:       make(map[string]coursesdb.BuyerAccount),
		courses:      make(map[string]coursesdb.Course),
		entitlements: make(map[string]coursesdb.CourseEntitlement),
	}
}

func (m *mockCoursePaymentQuerier) GetBuyerAccountByID(ctx context.Context, arg coursesdb.GetBuyerAccountByIDParams) (coursesdb.BuyerAccount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	b, ok := m.buyers[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(b.TenantID) != fromPgUUID(arg.TenantID) {
		return coursesdb.BuyerAccount{}, pgx.ErrNoRows
	}
	return b, nil
}

func (m *mockCoursePaymentQuerier) GetCourseByID(ctx context.Context, arg coursesdb.GetCourseByIDParams) (coursesdb.Course, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.courses[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(c.TenantID) != fromPgUUID(arg.TenantID) {
		return coursesdb.Course{}, pgx.ErrNoRows
	}
	return c, nil
}

func (m *mockCoursePaymentQuerier) GetCourseByIDForUpdate(ctx context.Context, arg coursesdb.GetCourseByIDForUpdateParams) (coursesdb.Course, error) {
	return m.GetCourseByID(ctx, coursesdb.GetCourseByIDParams{
		TenantID: arg.TenantID,
		ID:       arg.ID,
	})
}

func (m *mockCoursePaymentQuerier) GetCourseBySlug(ctx context.Context, arg coursesdb.GetCourseBySlugParams) (coursesdb.Course, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, c := range m.courses {
		if fromPgUUID(c.TenantID) == fromPgUUID(arg.TenantID) && c.Slug == arg.Slug {
			return c, nil
		}
	}
	return coursesdb.Course{}, pgx.ErrNoRows
}

func (m *mockCoursePaymentQuerier) GetActiveCourseEntitlement(ctx context.Context, arg coursesdb.GetActiveCourseEntitlementParams) (coursesdb.CourseEntitlement, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ent := range m.entitlements {
		if fromPgUUID(ent.TenantID) == fromPgUUID(arg.TenantID) &&
			fromPgUUID(ent.BuyerID) == fromPgUUID(arg.BuyerID) &&
			fromPgUUID(ent.CourseID) == fromPgUUID(arg.CourseID) &&
			ent.Status == "active" {
			return ent, nil
		}
	}
	return coursesdb.CourseEntitlement{}, pgx.ErrNoRows
}

func (m *mockCoursePaymentQuerier) GetCourseEntitlementByPurchaseID(ctx context.Context, arg coursesdb.GetCourseEntitlementByPurchaseIDParams) (coursesdb.CourseEntitlement, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ent := range m.entitlements {
		if fromPgUUID(ent.TenantID) == fromPgUUID(arg.TenantID) &&
			fromPgUUID(ent.PurchaseID) == fromPgUUID(arg.PurchaseID) {
			return ent, nil
		}
	}
	return coursesdb.CourseEntitlement{}, pgx.ErrNoRows
}

func (m *mockCoursePaymentQuerier) CreateCourseEntitlement(ctx context.Context, arg coursesdb.CreateCourseEntitlementParams) (coursesdb.CourseEntitlement, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New()
	ent := coursesdb.CourseEntitlement{
		ID:                  toPgUUID(id),
		TenantID:            arg.TenantID,
		BuyerID:             arg.BuyerID,
		CourseID:            arg.CourseID,
		PurchaseID:          arg.PurchaseID,
		Status:              arg.Status,
		CourseTitleSnapshot: arg.CourseTitleSnapshot,
		PriceRialsSnapshot:  arg.PriceRialsSnapshot,
		GrantedAt:           arg.GrantedAt,
		CreatedAt:           pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:           pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.entitlements[id.String()] = ent
	return ent, nil
}

func (m *mockCoursePaymentQuerier) ActivateCourseEntitlement(ctx context.Context, arg coursesdb.ActivateCourseEntitlementParams) (coursesdb.CourseEntitlement, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	idStr := fromPgUUID(arg.ID).String()
	ent, ok := m.entitlements[idStr]
	if !ok || fromPgUUID(ent.TenantID) != fromPgUUID(arg.TenantID) {
		return coursesdb.CourseEntitlement{}, pgx.ErrNoRows
	}
	ent.Status = "active"
	ent.GrantedAt = arg.GrantedAt
	ent.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.entitlements[idStr] = ent
	return ent, nil
}

func (m *mockCoursePaymentQuerier) RevokeCourseEntitlementByPurchaseID(ctx context.Context, arg coursesdb.RevokeCourseEntitlementByPurchaseIDParams) (coursesdb.CourseEntitlement, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for idStr, ent := range m.entitlements {
		if fromPgUUID(ent.TenantID) == fromPgUUID(arg.TenantID) &&
			fromPgUUID(ent.PurchaseID) == fromPgUUID(arg.PurchaseID) &&
			ent.Status == "active" {
			ent.Status = "revoked"
			ent.RevokedAt = arg.RevokedAt
			ent.RevocationReason = arg.RevocationReason
			ent.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
			m.entitlements[idStr] = ent
			return ent, nil
		}
	}
	return coursesdb.CourseEntitlement{}, pgx.ErrNoRows
}

func TestCoursePayment_Initiate_SnapshotAndIdempotency(t *testing.T) {
	tenantID := uuid.New()
	buyerID := uuid.New()
	courseID := uuid.New()

	pq := newMockPaymentsQuerier()
	cq := newMockCoursePaymentQuerier()
	provider := payments.NewMockPaymentProvider()

	// Provision active buyer
	cq.buyers[buyerID.String()] = coursesdb.BuyerAccount{
		ID:       toPgUUID(buyerID),
		TenantID: toPgUUID(tenantID),
		Phone:    "09123456789",
		FullName: "مریم حسینی",
		Status:   "active",
	}

	// Provision published course with Rials price
	cq.courses[courseID.String()] = coursesdb.Course{
		ID:             toPgUUID(courseID),
		TenantID:       toPgUUID(tenantID),
		Title:          "دوره جامع مدیریت اضطراب",
		Slug:           "anxiety-management",
		PriceRials:     12500000, // 1,250,000 Tomans
		Status:         "published",
		InstructorName: "دکتر مریم احمدی",
	}

	svc := payments.NewService(pq, provider, nil)
	svc.SetCourseQueries(cq)

	ctx := context.Background()
	now := time.Now()

	// 1. Successful Initiation
	res, err := svc.InitiateCoursePayment(ctx, payments.InitiateCoursePaymentParams{
		TenantID:       tenantID,
		BuyerID:        buyerID,
		CourseID:       courseID,
		IdempotencyKey: "chk_key_101",
		CallbackURL:    "https://clinic.example.com/callbacks/payments/zarinpal",
		Now:            now,
	})
	if err != nil {
		t.Fatalf("unexpected error initiating course payment: %v", err)
	}

	if res.AmountRials != 12500000 {
		t.Errorf("expected amount 12500000 Rials, got: %d", res.AmountRials)
	}
	if res.Authority == "" {
		t.Error("expected non-empty authority")
	}

	// Verify purchase snapshot
	purchase, err := pq.GetPurchaseByID(ctx, paymentsdb.GetPurchaseByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(res.PurchaseID),
	})
	if err != nil {
		t.Fatalf("failed to retrieve created purchase: %v", err)
	}
	if purchase.ItemType != "course" {
		t.Errorf("expected item_type 'course', got: %s", purchase.ItemType)
	}
	if purchase.AmountRials != 12500000 {
		t.Errorf("expected snapshot amount 12500000 Rials, got: %d", purchase.AmountRials)
	}
	if purchase.TitleSnapshot != "دوره جامع مدیریت اضطراب" {
		t.Errorf("expected snapshot title 'دوره جامع مدیریت اضطراب', got: %s", purchase.TitleSnapshot)
	}
	if purchase.Status != "initiated" {
		t.Errorf("expected purchase status 'initiated', got: %s", purchase.Status)
	}

	// Verify pending entitlement record
	ent, err := cq.GetCourseEntitlementByPurchaseID(ctx, coursesdb.GetCourseEntitlementByPurchaseIDParams{
		TenantID:   toPgUUID(tenantID),
		PurchaseID: toPgUUID(res.PurchaseID),
	})
	if err != nil {
		t.Fatalf("failed to retrieve created entitlement: %v", err)
	}
	if ent.Status != "pending" {
		t.Errorf("expected entitlement status 'pending', got: %s", ent.Status)
	}
	if ent.GrantedAt.Valid {
		t.Errorf("expected GrantedAt to be null until verified, got valid: %v", ent.GrantedAt.Time)
	}

	// 2. Idempotency test: repeating with same key returns identical attempt
	idempRes, err := svc.InitiateCoursePayment(ctx, payments.InitiateCoursePaymentParams{
		TenantID:       tenantID,
		BuyerID:        buyerID,
		CourseID:       courseID,
		IdempotencyKey: "chk_key_101",
		CallbackURL:    "https://clinic.example.com/callbacks/payments/zarinpal",
		Now:            now,
	})
	if err != nil {
		t.Fatalf("idempotency retry failed: %v", err)
	}
	if idempRes.PaymentAttemptID != res.PaymentAttemptID || idempRes.Authority != res.Authority {
		t.Errorf("idempotency returned mismatching attempt info: got %+v, want %+v", idempRes, res)
	}

	// 3. Block initiation if buyer already holds an active entitlement
	cq.entitlements[uuid.New().String()] = coursesdb.CourseEntitlement{
		ID:       toPgUUID(uuid.New()),
		TenantID: toPgUUID(tenantID),
		BuyerID:  toPgUUID(buyerID),
		CourseID: toPgUUID(courseID),
		Status:   "active",
	}

	_, err = svc.InitiateCoursePayment(ctx, payments.InitiateCoursePaymentParams{
		TenantID:       tenantID,
		BuyerID:        buyerID,
		CourseID:       courseID,
		IdempotencyKey: "chk_key_diff_102",
		CallbackURL:    "https://clinic.example.com/callbacks/payments/zarinpal",
		Now:            now,
	})
	if err == nil {
		t.Error("expected error when buyer already holds an active entitlement, got nil")
	}
}

func TestCoursePayment_Callback_SuccessfulVerification(t *testing.T) {
	tenantID := uuid.New()
	buyerID := uuid.New()
	courseID := uuid.New()

	pq := newMockPaymentsQuerier()
	cq := newMockCoursePaymentQuerier()
	provider := payments.NewMockPaymentProvider()

	cq.buyers[buyerID.String()] = coursesdb.BuyerAccount{
		ID:       toPgUUID(buyerID),
		TenantID: toPgUUID(tenantID),
		Phone:    "09123456789",
		FullName: "سارا احمدی",
		Status:   "active",
	}
	cq.courses[courseID.String()] = coursesdb.Course{
		ID:         toPgUUID(courseID),
		TenantID:   toPgUUID(tenantID),
		Title:      "دوره درمان شناختی رفتاری",
		Slug:       "cbt-course",
		PriceRials: 18000000,
		Status:     "published",
	}

	svc := payments.NewService(pq, provider, nil)
	svc.SetCourseQueries(cq)

	ctx := context.Background()
	now := time.Now()

	initRes, err := svc.InitiateCoursePayment(ctx, payments.InitiateCoursePaymentParams{
		TenantID:       tenantID,
		BuyerID:        buyerID,
		CourseID:       courseID,
		IdempotencyKey: "chk_verify_001",
		CallbackURL:    "https://clinic.example.com/callbacks/payments/zarinpal",
		Now:            now,
	})
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	// Gateway returns with OK
	cbRes, err := svc.HandleCallbackAndVerify(ctx, tenantID, initRes.Authority, "OK", now)
	if err != nil {
		t.Fatalf("unexpected callback error: %v", err)
	}

	if cbRes.Status != "confirmed" {
		t.Errorf("expected confirmed status, got: %s", cbRes.Status)
	}
	if cbRes.CourseID != courseID {
		t.Errorf("expected CourseID %s, got: %s", courseID, cbRes.CourseID)
	}

	// Verify atomic activation of entitlement
	ent, err := cq.GetCourseEntitlementByPurchaseID(ctx, coursesdb.GetCourseEntitlementByPurchaseIDParams{
		TenantID:   toPgUUID(tenantID),
		PurchaseID: toPgUUID(initRes.PurchaseID),
	})
	if err != nil {
		t.Fatalf("failed to retrieve entitlement: %v", err)
	}
	if ent.Status != "active" {
		t.Errorf("expected entitlement status 'active', got: %s", ent.Status)
	}
	if !ent.GrantedAt.Valid {
		t.Error("expected GrantedAt to be set upon verification")
	}

	// Verify purchase status completed
	purchase, err := pq.GetPurchaseByID(ctx, paymentsdb.GetPurchaseByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(initRes.PurchaseID),
	})
	if err != nil {
		t.Fatalf("failed to retrieve purchase: %v", err)
	}
	if purchase.Status != "completed" {
		t.Errorf("expected purchase status 'completed', got: %s", purchase.Status)
	}

	// Verify idempotent duplicate callback
	cbRetry, err := svc.HandleCallbackAndVerify(ctx, tenantID, initRes.Authority, "OK", now)
	if err != nil {
		t.Fatalf("callback retry failed: %v", err)
	}
	if cbRetry.Status != "confirmed" {
		t.Errorf("expected retry status confirmed, got: %s", cbRetry.Status)
	}
}

func TestCoursePayment_Callback_DuplicatePaymentMovesToRefundRequired(t *testing.T) {
	tenantID := uuid.New()
	buyerID := uuid.New()
	courseID := uuid.New()

	pq := newMockPaymentsQuerier()
	cq := newMockCoursePaymentQuerier()
	provider := payments.NewMockPaymentProvider()

	cq.buyers[buyerID.String()] = coursesdb.BuyerAccount{
		ID:       toPgUUID(buyerID),
		TenantID: toPgUUID(tenantID),
		Phone:    "09123456789",
		FullName: "علی رضایی",
		Status:   "active",
	}
	cq.courses[courseID.String()] = coursesdb.Course{
		ID:         toPgUUID(courseID),
		TenantID:   toPgUUID(tenantID),
		Title:      "دوره زوج درمانی",
		Slug:       "couples-therapy",
		PriceRials: 15000000,
		Status:     "published",
	}

	svc := payments.NewService(pq, provider, nil)
	svc.SetCourseQueries(cq)

	ctx := context.Background()
	now := time.Now()

	// Simulate concurrent payment 1
	init1, err := svc.InitiateCoursePayment(ctx, payments.InitiateCoursePaymentParams{
		TenantID:       tenantID,
		BuyerID:        buyerID,
		CourseID:       courseID,
		IdempotencyKey: "concurrent_chk_1",
		CallbackURL:    "https://clinic.example.com/callbacks",
		Now:            now,
	})
	if err != nil {
		t.Fatalf("init 1 failed: %v", err)
	}

	// Simulate concurrent payment 2 (initiated right before 1 finished verifying)
	init2, err := svc.InitiateCoursePayment(ctx, payments.InitiateCoursePaymentParams{
		TenantID:       tenantID,
		BuyerID:        buyerID,
		CourseID:       courseID,
		IdempotencyKey: "concurrent_chk_2",
		CallbackURL:    "https://clinic.example.com/callbacks",
		Now:            now,
	})
	if err != nil {
		t.Fatalf("init 2 failed: %v", err)
	}

	// Payment 1 verifies first -> Success, Entitlement Active
	cb1, err := svc.HandleCallbackAndVerify(ctx, tenantID, init1.Authority, "OK", now)
	if err != nil || cb1.Status != "confirmed" {
		t.Fatalf("cb1 failed: status=%s, err=%v", cb1.Status, err)
	}

	// Payment 2 callback arrives -> user already has active entitlement!
	// "تست پرداخت تکراری (پرداخت دوم هم‌زمان به refund_required می‌رود و entitlement تکراری ساخته نمی‌شود)"
	cb2, err := svc.HandleCallbackAndVerify(ctx, tenantID, init2.Authority, "OK", now)
	if err != nil {
		t.Fatalf("cb2 unexpected error: %v", err)
	}

	if cb2.Status != "refund_required" {
		t.Errorf("expected duplicate concurrent payment status 'refund_required', got: %s", cb2.Status)
	}

	// Verify purchase 2 moved to refund_required
	purchase2, err := pq.GetPurchaseByID(ctx, paymentsdb.GetPurchaseByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(init2.PurchaseID),
	})
	if err != nil {
		t.Fatalf("failed to retrieve purchase 2: %v", err)
	}
	if purchase2.Status != "refund_required" {
		t.Errorf("expected purchase 2 status 'refund_required', got: %s", purchase2.Status)
	}

	// Verify entitlement 2 was NOT activated (remains pending or not active)
	ent2, err := cq.GetCourseEntitlementByPurchaseID(ctx, coursesdb.GetCourseEntitlementByPurchaseIDParams{
		TenantID:   toPgUUID(tenantID),
		PurchaseID: toPgUUID(init2.PurchaseID),
	})
	if err != nil {
		t.Fatalf("failed to retrieve entitlement 2: %v", err)
	}
	if ent2.Status == "active" {
		t.Error("duplicate payment must NOT activate duplicate entitlement!")
	}
}

func TestCoursePayment_Refund_RevokesEntitlement(t *testing.T) {
	tenantID := uuid.New()
	buyerID := uuid.New()
	courseID := uuid.New()
	ownerAdminID := uuid.New()

	pq := newMockPaymentsQuerier()
	cq := newMockCoursePaymentQuerier()
	provider := payments.NewMockPaymentProvider()

	cq.buyers[buyerID.String()] = coursesdb.BuyerAccount{
		ID:       toPgUUID(buyerID),
		TenantID: toPgUUID(tenantID),
		Phone:    "09123456789",
		FullName: "نرگس محمدی",
		Status:   "active",
	}
	cq.courses[courseID.String()] = coursesdb.Course{
		ID:         toPgUUID(courseID),
		TenantID:   toPgUUID(tenantID),
		Title:      "دوره پرورش ذهن آگاهی",
		Slug:       "mindfulness",
		PriceRials: 10000000,
		Status:     "published",
	}

	svc := payments.NewService(pq, provider, nil)
	svc.SetCourseQueries(cq)

	ctx := context.Background()
	now := time.Now()

	// 1. Buy and verify course
	initRes, err := svc.InitiateCoursePayment(ctx, payments.InitiateCoursePaymentParams{
		TenantID:       tenantID,
		BuyerID:        buyerID,
		CourseID:       courseID,
		IdempotencyKey: "chk_refund_001",
		CallbackURL:    "https://clinic.example.com/callbacks",
		Now:            now,
	})
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	_, err = svc.HandleCallbackAndVerify(ctx, tenantID, initRes.Authority, "OK", now)
	if err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	// Verify entitlement active before refund
	entBefore, err := cq.GetCourseEntitlementByPurchaseID(ctx, coursesdb.GetCourseEntitlementByPurchaseIDParams{
		TenantID:   toPgUUID(tenantID),
		PurchaseID: toPgUUID(initRes.PurchaseID),
	})
	if err != nil || entBefore.Status != "active" {
		t.Fatalf("expected active entitlement before refund, got status=%s, err=%v", entBefore.Status, err)
	}

	// 2. Process Refund
	refundRes, err := svc.RecordRefund(ctx, payments.RecordRefundParams{
		TenantID:        tenantID,
		PurchaseID:      initRes.PurchaseID,
		ReferenceNumber: "BANK_REF_998877",
		Reason:          "انصراف خریدار از دوره طبق گارانتی بازگشت وجه",
		AmountRials:     10000000,
		RegisteredBy:    ownerAdminID,
		Now:             now,
	})
	if err != nil {
		t.Fatalf("RecordRefund failed: %v", err)
	}
	if refundRes.ReferenceNumber != "BANK_REF_998877" {
		t.Errorf("expected reference number BANK_REF_998877, got: %s", refundRes.ReferenceNumber)
	}

	// 3. Verify entitlement revoked
	// "بازپرداخت ثبت‌شده entitlement همان خرید را لغو می‌کند."
	entAfter, err := cq.GetCourseEntitlementByPurchaseID(ctx, coursesdb.GetCourseEntitlementByPurchaseIDParams{
		TenantID:   toPgUUID(tenantID),
		PurchaseID: toPgUUID(initRes.PurchaseID),
	})
	if err != nil {
		t.Fatalf("failed to retrieve entitlement after refund: %v", err)
	}
	if entAfter.Status != "revoked" {
		t.Errorf("expected entitlement status 'revoked', got: %s", entAfter.Status)
	}
	if !entAfter.RevokedAt.Valid {
		t.Error("expected RevokedAt to be set upon refund")
	}
	if !entAfter.RevocationReason.Valid || entAfter.RevocationReason.String != "انصراف خریدار از دوره طبق گارانتی بازگشت وجه" {
		t.Errorf("unexpected revocation reason: %v", entAfter.RevocationReason.String)
	}

	// 4. Verify idempotent retry of refund
	idempRefund, err := svc.RecordRefund(ctx, payments.RecordRefundParams{
		TenantID:        tenantID,
		PurchaseID:      initRes.PurchaseID,
		ReferenceNumber: "BANK_REF_998877",
		Reason:          "انصراف خریدار",
		AmountRials:     10000000,
		RegisteredBy:    ownerAdminID,
		Now:             now,
	})
	if err != nil {
		t.Fatalf("idempotent refund retry failed: %v", err)
	}
	if idempRefund.RefundID != refundRes.RefundID {
		t.Errorf("idempotent refund returned mismatching refund ID: got %s, want %s", idempRefund.RefundID, refundRes.RefundID)
	}
}
