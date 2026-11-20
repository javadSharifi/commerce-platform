package payments_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"psychology/api/internal/auth"
	"psychology/api/internal/auth/authdb"
	"psychology/api/internal/domain"
	"psychology/api/internal/domain/domaindb"
	"psychology/api/internal/payments"
	"psychology/api/internal/payments/paymentsdb"
	"psychology/api/internal/workshops/workshopsdb"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type mockPaymentsQuerier struct {
	mu              sync.Mutex
	purchases       map[string]paymentsdb.Purchase
	paymentAttempts map[string]paymentsdb.PaymentAttempt
	refunds         map[string]paymentsdb.Refund
}

func newMockPaymentsQuerier() *mockPaymentsQuerier {
	return &mockPaymentsQuerier{
		purchases:       make(map[string]paymentsdb.Purchase),
		paymentAttempts: make(map[string]paymentsdb.PaymentAttempt),
		refunds:         make(map[string]paymentsdb.Refund),
	}
}

func toPgUUID(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

func fromPgUUID(p pgtype.UUID) uuid.UUID {
	var u uuid.UUID
	copy(u[:], p.Bytes[:])
	return u
}

func (m *mockPaymentsQuerier) CreatePurchase(ctx context.Context, arg paymentsdb.CreatePurchaseParams) (paymentsdb.Purchase, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New()
	now := time.Now()
	p := paymentsdb.Purchase{
		ID:            toPgUUID(id),
		TenantID:      arg.TenantID,
		ItemType:      arg.ItemType,
		ItemID:        arg.ItemID,
		AmountRials:   arg.AmountRials,
		TitleSnapshot: arg.TitleSnapshot,
		Status:        arg.Status,
		CreatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
	}
	m.purchases[id.String()] = p
	return p, nil
}

func (m *mockPaymentsQuerier) GetPurchaseByID(ctx context.Context, arg paymentsdb.GetPurchaseByIDParams) (paymentsdb.Purchase, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.purchases[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(p.TenantID) != fromPgUUID(arg.TenantID) {
		return paymentsdb.Purchase{}, pgx.ErrNoRows
	}
	return p, nil
}

func (m *mockPaymentsQuerier) GetPurchaseByIDForUpdate(ctx context.Context, arg paymentsdb.GetPurchaseByIDForUpdateParams) (paymentsdb.Purchase, error) {
	return m.GetPurchaseByID(ctx, paymentsdb.GetPurchaseByIDParams{
		TenantID: arg.TenantID,
		ID:       arg.ID,
	})
}

func (m *mockPaymentsQuerier) UpdatePurchaseStatus(ctx context.Context, arg paymentsdb.UpdatePurchaseStatusParams) (paymentsdb.Purchase, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.purchases[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(p.TenantID) != fromPgUUID(arg.TenantID) {
		return paymentsdb.Purchase{}, pgx.ErrNoRows
	}
	p.Status = arg.Status
	p.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.purchases[fromPgUUID(arg.ID).String()] = p
	return p, nil
}

func (m *mockPaymentsQuerier) ListPurchases(ctx context.Context, arg paymentsdb.ListPurchasesParams) ([]paymentsdb.Purchase, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var res []paymentsdb.Purchase
	for _, p := range m.purchases {
		if fromPgUUID(p.TenantID) == fromPgUUID(arg.TenantID) {
			if !arg.Status.Valid || p.Status == arg.Status.String {
				res = append(res, p)
			}
		}
	}
	return res, nil
}

func (m *mockPaymentsQuerier) CreatePaymentAttempt(ctx context.Context, arg paymentsdb.CreatePaymentAttemptParams) (paymentsdb.PaymentAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New()
	now := time.Now()
	pa := paymentsdb.PaymentAttempt{
		ID:             toPgUUID(id),
		TenantID:       arg.TenantID,
		PurchaseID:     arg.PurchaseID,
		Provider:       arg.Provider,
		Authority:      arg.Authority,
		AmountRials:    arg.AmountRials,
		Status:         arg.Status,
		IdempotencyKey: arg.IdempotencyKey,
		CreatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
	}
	m.paymentAttempts[id.String()] = pa
	return pa, nil
}

func (m *mockPaymentsQuerier) GetPaymentAttemptByAuthority(ctx context.Context, arg paymentsdb.GetPaymentAttemptByAuthorityParams) (paymentsdb.PaymentAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, pa := range m.paymentAttempts {
		if fromPgUUID(pa.TenantID) == fromPgUUID(arg.TenantID) && pa.Authority == arg.Authority {
			return pa, nil
		}
	}
	return paymentsdb.PaymentAttempt{}, pgx.ErrNoRows
}

func (m *mockPaymentsQuerier) GetPaymentAttemptByIdempotencyKey(ctx context.Context, arg paymentsdb.GetPaymentAttemptByIdempotencyKeyParams) (paymentsdb.PaymentAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, pa := range m.paymentAttempts {
		if fromPgUUID(pa.TenantID) == fromPgUUID(arg.TenantID) && pa.IdempotencyKey == arg.IdempotencyKey {
			return pa, nil
		}
	}
	return paymentsdb.PaymentAttempt{}, pgx.ErrNoRows
}

func (m *mockPaymentsQuerier) UpdatePaymentAttemptStatus(ctx context.Context, arg paymentsdb.UpdatePaymentAttemptStatusParams) (paymentsdb.PaymentAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pa, ok := m.paymentAttempts[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(pa.TenantID) != fromPgUUID(arg.TenantID) {
		return paymentsdb.PaymentAttempt{}, pgx.ErrNoRows
	}
	pa.Status = arg.Status
	if arg.TrackingCode.Valid {
		pa.TrackingCode = arg.TrackingCode
	}
	if arg.GatewayRefID.Valid {
		pa.GatewayRefID = arg.GatewayRefID
	}
	if arg.ErrorMessage.Valid {
		pa.ErrorMessage = arg.ErrorMessage
	}
	pa.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.paymentAttempts[fromPgUUID(arg.ID).String()] = pa
	return pa, nil
}

func (m *mockPaymentsQuerier) GetActivePaymentAttemptByPurchase(ctx context.Context, arg paymentsdb.GetActivePaymentAttemptByPurchaseParams) (paymentsdb.PaymentAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, pa := range m.paymentAttempts {
		if fromPgUUID(pa.TenantID) == fromPgUUID(arg.TenantID) &&
			fromPgUUID(pa.PurchaseID) == fromPgUUID(arg.PurchaseID) &&
			pa.Status == "initiated" {
			return pa, nil
		}
	}
	return paymentsdb.PaymentAttempt{}, pgx.ErrNoRows
}

func (m *mockPaymentsQuerier) ListPaymentAttemptsByPurchase(ctx context.Context, arg paymentsdb.ListPaymentAttemptsByPurchaseParams) ([]paymentsdb.PaymentAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var res []paymentsdb.PaymentAttempt
	for _, pa := range m.paymentAttempts {
		if fromPgUUID(pa.TenantID) == fromPgUUID(arg.TenantID) && fromPgUUID(pa.PurchaseID) == fromPgUUID(arg.PurchaseID) {
			res = append(res, pa)
		}
	}
	return res, nil
}

func (m *mockPaymentsQuerier) CreateRefund(ctx context.Context, arg paymentsdb.CreateRefundParams) (paymentsdb.Refund, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New()
	ref := paymentsdb.Refund{
		ID:               toPgUUID(id),
		TenantID:         arg.TenantID,
		PurchaseID:       arg.PurchaseID,
		PaymentAttemptID: arg.PaymentAttemptID,
		AmountRials:      arg.AmountRials,
		Reason:           arg.Reason,
		ReferenceNumber:  arg.ReferenceNumber,
		RegisteredBy:     arg.RegisteredBy,
		RegisteredAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
		CreatedAt:        pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.refunds[id.String()] = ref
	return ref, nil
}

func (m *mockPaymentsQuerier) GetRefundByReference(ctx context.Context, arg paymentsdb.GetRefundByReferenceParams) (paymentsdb.Refund, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, r := range m.refunds {
		if fromPgUUID(r.TenantID) == fromPgUUID(arg.TenantID) && r.ReferenceNumber == arg.ReferenceNumber {
			return r, nil
		}
	}
	return paymentsdb.Refund{}, pgx.ErrNoRows
}

func (m *mockPaymentsQuerier) ListRefundsByPurchase(ctx context.Context, arg paymentsdb.ListRefundsByPurchaseParams) ([]paymentsdb.Refund, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var res []paymentsdb.Refund
	for _, r := range m.refunds {
		if fromPgUUID(r.TenantID) == fromPgUUID(arg.TenantID) && fromPgUUID(r.PurchaseID) == fromPgUUID(arg.PurchaseID) {
			res = append(res, r)
		}
	}
	return res, nil
}

func (m *mockPaymentsQuerier) GetVerifiedPaymentAttemptByPurchase(ctx context.Context, arg paymentsdb.GetVerifiedPaymentAttemptByPurchaseParams) (paymentsdb.PaymentAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, pa := range m.paymentAttempts {
		if fromPgUUID(pa.TenantID) == fromPgUUID(arg.TenantID) &&
			fromPgUUID(pa.PurchaseID) == fromPgUUID(arg.PurchaseID) &&
			pa.Status == "verified" {
			return pa, nil
		}
	}
	return paymentsdb.PaymentAttempt{}, pgx.ErrNoRows
}

func (m *mockPaymentsQuerier) ListStalePaymentAttempts(ctx context.Context, arg paymentsdb.ListStalePaymentAttemptsParams) ([]paymentsdb.PaymentAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var res []paymentsdb.PaymentAttempt
	for _, pa := range m.paymentAttempts {
		if fromPgUUID(pa.TenantID) == fromPgUUID(arg.TenantID) &&
			pa.Status == "initiated" &&
			(pa.CreatedAt.Time.Before(arg.CreatedAt.Time) || pa.CreatedAt.Time.Equal(arg.CreatedAt.Time)) {
			res = append(res, pa)
			if int32(len(res)) >= arg.Limit {
				break
			}
		}
	}
	return res, nil
}

// ----------------------------------------------------
// Mock Workshop Querier for Payment Integration
// ----------------------------------------------------

type mockWorkshopQuerier struct {
	mu            sync.Mutex
	workshops     map[string]workshopsdb.Workshop
	registrations map[string]workshopsdb.WorkshopRegistration
}

func newMockWorkshopQuerier() *mockWorkshopQuerier {
	return &mockWorkshopQuerier{
		workshops:     make(map[string]workshopsdb.Workshop),
		registrations: make(map[string]workshopsdb.WorkshopRegistration),
	}
}

func (m *mockWorkshopQuerier) GetRegistrationByID(ctx context.Context, arg workshopsdb.GetRegistrationByIDParams) (workshopsdb.WorkshopRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.registrations[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(r.TenantID) != fromPgUUID(arg.TenantID) {
		return workshopsdb.WorkshopRegistration{}, pgx.ErrNoRows
	}
	return r, nil
}

func (m *mockWorkshopQuerier) GetRegistrationByIDForUpdate(ctx context.Context, arg workshopsdb.GetRegistrationByIDForUpdateParams) (workshopsdb.WorkshopRegistration, error) {
	return m.GetRegistrationByID(ctx, workshopsdb.GetRegistrationByIDParams{
		TenantID: arg.TenantID,
		ID:       arg.ID,
	})
}

func (m *mockWorkshopQuerier) GetRegistrationByPurchaseID(ctx context.Context, arg workshopsdb.GetRegistrationByPurchaseIDParams) (workshopsdb.WorkshopRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, r := range m.registrations {
		if fromPgUUID(r.TenantID) == fromPgUUID(arg.TenantID) && r.PurchaseID.Valid && fromPgUUID(r.PurchaseID) == fromPgUUID(arg.PurchaseID) {
			return r, nil
		}
	}
	return workshopsdb.WorkshopRegistration{}, pgx.ErrNoRows
}

func (m *mockWorkshopQuerier) GetRegistrationByPurchaseIDForUpdate(ctx context.Context, arg workshopsdb.GetRegistrationByPurchaseIDForUpdateParams) (workshopsdb.WorkshopRegistration, error) {
	return m.GetRegistrationByPurchaseID(ctx, workshopsdb.GetRegistrationByPurchaseIDParams{
		TenantID:   arg.TenantID,
		PurchaseID: arg.PurchaseID,
	})
}

func (m *mockWorkshopQuerier) GetWorkshopByID(ctx context.Context, arg workshopsdb.GetWorkshopByIDParams) (workshopsdb.Workshop, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	w, ok := m.workshops[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(w.TenantID) != fromPgUUID(arg.TenantID) {
		return workshopsdb.Workshop{}, pgx.ErrNoRows
	}
	return w, nil
}

func (m *mockWorkshopQuerier) GetWorkshopByIDForUpdate(ctx context.Context, arg workshopsdb.GetWorkshopByIDForUpdateParams) (workshopsdb.Workshop, error) {
	return m.GetWorkshopByID(ctx, workshopsdb.GetWorkshopByIDParams{
		TenantID: arg.TenantID,
		ID:       arg.ID,
	})
}

func (m *mockWorkshopQuerier) UpdateRegistrationStatus(ctx context.Context, arg workshopsdb.UpdateRegistrationStatusParams) (workshopsdb.WorkshopRegistration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.registrations[fromPgUUID(arg.ID).String()]
	if !ok || fromPgUUID(r.TenantID) != fromPgUUID(arg.TenantID) {
		return workshopsdb.WorkshopRegistration{}, pgx.ErrNoRows
	}
	r.Status = arg.Status
	if arg.PurchaseID.Valid {
		r.PurchaseID = arg.PurchaseID
	}
	r.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.registrations[fromPgUUID(arg.ID).String()] = r
	return r, nil
}

func (m *mockWorkshopQuerier) ExpirePendingRegistrations(ctx context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var count int64
	now := time.Now()
	for k, r := range m.registrations {
		if r.Status == "pending_payment" && r.ExpiresAt.Valid && r.ExpiresAt.Time.Before(now) {
			r.Status = "expired"
			r.UpdatedAt = pgtype.Timestamptz{Time: now, Valid: true}
			m.registrations[k] = r
			count++
		}
	}
	return count, nil
}

// ----------------------------------------------------
// Mock Auth Querier for Admin Authorization
// ----------------------------------------------------

type mockAuthQuerier struct {
	memberships map[string]authdb.Membership
	sessions    map[string]authdb.PanelSession
	identities  map[string]authdb.PanelIdentity
}

func (m *mockAuthQuerier) GetPanelIdentityByEmail(ctx context.Context, email string) (authdb.PanelIdentity, error) {
	return authdb.PanelIdentity{}, pgx.ErrNoRows
}
func (m *mockAuthQuerier) GetPanelIdentityByID(ctx context.Context, id pgtype.UUID) (authdb.PanelIdentity, error) {
	i, ok := m.identities[fromPgUUID(id).String()]
	if !ok {
		return authdb.PanelIdentity{}, pgx.ErrNoRows
	}
	return i, nil
}
func (m *mockAuthQuerier) CreatePanelIdentity(ctx context.Context, arg authdb.CreatePanelIdentityParams) (authdb.PanelIdentity, error) {
	return authdb.PanelIdentity{}, nil
}
func (m *mockAuthQuerier) CreatePanelSession(ctx context.Context, arg authdb.CreatePanelSessionParams) (authdb.PanelSession, error) {
	return authdb.PanelSession{}, nil
}
func (m *mockAuthQuerier) GetPanelSessionByTokenHash(ctx context.Context, tokenHash string) (authdb.PanelSession, error) {
	s, ok := m.sessions[tokenHash]
	if !ok {
		return authdb.PanelSession{}, pgx.ErrNoRows
	}
	return s, nil
}
func (m *mockAuthQuerier) DeletePanelSession(ctx context.Context, tokenHash string) error {
	return nil
}
func (m *mockAuthQuerier) ListMembershipsByUserID(ctx context.Context, userID pgtype.UUID) ([]authdb.ListMembershipsByUserIDRow, error) {
	return nil, nil
}
func (m *mockAuthQuerier) GetMembershipByUserAndTenant(ctx context.Context, arg authdb.GetMembershipByUserAndTenantParams) (authdb.Membership, error) {
	key := fromPgUUID(arg.UserID).String() + ":" + fromPgUUID(arg.TenantID).String()
	mem, ok := m.memberships[key]
	if !ok {
		return authdb.Membership{}, pgx.ErrNoRows
	}
	return mem, nil
}
func (m *mockAuthQuerier) CreateMembership(ctx context.Context, arg authdb.CreateMembershipParams) (authdb.Membership, error) {
	return authdb.Membership{}, nil
}

// ----------------------------------------------------
// TESTS
// ----------------------------------------------------

func TestMockPaymentProvider_Basic(t *testing.T) {
	mockProv := payments.NewMockPaymentProvider()
	if mockProv.Name() != "mock" {
		t.Fatalf("expected provider name 'mock', got %s", mockProv.Name())
	}

	ctx := context.Background()
	req := payments.PaymentRequest{
		AmountRials: 10000000,
		Description: "تست کارگاه",
		CallbackURL: "https://clinic.ir/callbacks/payments/zarinpal",
		Mobile:      "09121111111",
	}

	resp, err := mockProv.RequestPayment(ctx, req)
	if err != nil {
		t.Fatalf("mock RequestPayment failed: %v", err)
	}
	if resp.Authority == "" || resp.PaymentURL == "" {
		t.Fatalf("expected valid authority and payment url, got authority=%s url=%s", resp.Authority, resp.PaymentURL)
	}

	vReq := payments.VerifyRequest{
		Authority:   resp.Authority,
		AmountRials: 10000000,
	}
	vResp, err := mockProv.VerifyPayment(ctx, vReq)
	if err != nil {
		t.Fatalf("mock VerifyPayment failed: %v", err)
	}
	if vResp.Code != 100 || vResp.RefID == "" {
		t.Fatalf("expected code 100 and non-empty refID, got code=%d refID=%s", vResp.Code, vResp.RefID)
	}
}

func TestZarinPalProvider_RequestPayment_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pg/v4/payment/request.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var reqBody map[string]any
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		if reqBody["amount"].(float64) != 25000000 {
			t.Errorf("expected amount 25000000, got %v", reqBody["amount"])
		}

		resp := map[string]any{
			"data": map[string]any{
				"code":      100,
				"message":   "Success",
				"authority": "A00000000000000000000000000000000001",
				"fee_type":  "Merchant",
				"fee":       0,
			},
			"errors": []any{},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	zp := payments.NewZarinPalProvider("test-merchant-id", true, server.Client(), payments.WithBaseURL(server.URL))
	ctx := context.Background()
	req := payments.PaymentRequest{
		AmountRials: 25000000,
		Description: "کارگاه تربیت کودک",
		CallbackURL: "https://clinic.ir/callbacks/payments/zarinpal",
		Mobile:      "09121112233",
	}

	resp, err := zp.RequestPayment(ctx, req)
	if err != nil {
		t.Fatalf("zarinpal RequestPayment failed: %v", err)
	}
	if resp.Authority != "A00000000000000000000000000000000001" {
		t.Errorf("expected authority A00000000000000000000000000000000001, got %s", resp.Authority)
	}
	expectedURL := server.URL + "/pg/StartPay/A00000000000000000000000000000000001"
	if resp.PaymentURL != expectedURL {
		t.Errorf("expected payment URL %s, got %s", expectedURL, resp.PaymentURL)
	}
}

func TestZarinPalProvider_RequestPayment_GatewayRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []any{},
			"errors": map[string]any{
				"code":    -9,
				"message": "Validation error: Invalid merchant",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	zp := payments.NewZarinPalProvider("bad-merchant", true, server.Client(), payments.WithBaseURL(server.URL))
	ctx := context.Background()
	_, err := zp.RequestPayment(ctx, payments.PaymentRequest{
		AmountRials: 10000000,
		CallbackURL: "https://clinic.ir/cb",
	})
	if !errors.Is(err, payments.ErrGatewayRejected) {
		t.Fatalf("expected ErrGatewayRejected, got: %v", err)
	}
}

func TestZarinPalProvider_RequestPayment_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := server.Client()
	client.Timeout = 20 * time.Millisecond
	zp := payments.NewZarinPalProvider("merchant", true, client, payments.WithBaseURL(server.URL))

	ctx := context.Background()
	_, err := zp.RequestPayment(ctx, payments.PaymentRequest{
		AmountRials: 10000000,
		CallbackURL: "https://clinic.ir/cb",
	})
	if !errors.Is(err, payments.ErrGatewayTimeout) {
		t.Fatalf("expected ErrGatewayTimeout, got: %v", err)
	}
}

func TestInitiatePayment_RialsSnapshot(t *testing.T) {
	ctx := context.Background()
	payMock := newMockPaymentsQuerier()
	wsMock := newMockWorkshopQuerier()
	prov := payments.NewMockPaymentProvider()
	svc := payments.NewService(payMock, prov, wsMock)

	tenantID := uuid.New()
	wsID := uuid.New()
	regID := uuid.New()
	now := time.Now()

	// 1. Create a Workshop with 5,000,000 Rials (500,000 Tomans)
	workshopPrice := int64(5000000)
	workshopTitle := "کارگاه روان‌شناسی بالینی"
	wsMock.workshops[wsID.String()] = workshopsdb.Workshop{
		ID:         toPgUUID(wsID),
		TenantID:   toPgUUID(tenantID),
		Title:      workshopTitle,
		PriceRials: workshopPrice,
		Status:     "published",
	}

	// 2. Create a pending registration hold
	wsMock.registrations[regID.String()] = workshopsdb.WorkshopRegistration{
		ID:         toPgUUID(regID),
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(wsID),
		GuestName:  "مریم نادری",
		GuestPhone: "09123334455",
		Status:     "pending_payment",
		ExpiresAt:  pgtype.Timestamptz{Time: now.Add(15 * time.Minute), Valid: true},
	}

	// 3. Initiate payment
	result, err := svc.InitiateWorkshopPayment(ctx, payments.InitiateWorkshopPaymentParams{
		TenantID:       tenantID,
		RegistrationID: regID,
		IdempotencyKey: "reg_key_001",
		CallbackURL:    "https://clinic.ir/callbacks/payments/zarinpal",
		Now:            now,
	})
	if err != nil {
		t.Fatalf("initiate payment failed: %v", err)
	}

	if result.AmountRials != workshopPrice {
		t.Errorf("expected amount %d Rials, got %d", workshopPrice, result.AmountRials)
	}
	if result.Authority == "" || result.PaymentURL == "" {
		t.Errorf("expected valid authority and payment url, got authority=%s url=%s", result.Authority, result.PaymentURL)
	}

	// 4. Verify Snapshot in purchases table:
	purchase, err := payMock.GetPurchaseByID(ctx, paymentsdb.GetPurchaseByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(result.PurchaseID),
	})
	if err != nil {
		t.Fatalf("failed to get purchase snapshot: %v", err)
	}
	if purchase.AmountRials != workshopPrice {
		t.Errorf("snapshot amount must be %d, got %d", workshopPrice, purchase.AmountRials)
	}
	if purchase.TitleSnapshot != workshopTitle {
		t.Errorf("snapshot title must be %q, got %q", workshopTitle, purchase.TitleSnapshot)
	}
	if purchase.Status != "pending" {
		t.Errorf("expected purchase status 'pending', got %s", purchase.Status)
	}

	// 5. Subsequent alteration to parent workshop MUST NOT alter the purchase snapshot!
	wsMod := wsMock.workshops[wsID.String()]
	wsMod.PriceRials = 99999999
	wsMod.Title = "عنوان تغییریافته"
	wsMock.workshops[wsID.String()] = wsMod

	pCheck, _ := payMock.GetPurchaseByID(ctx, paymentsdb.GetPurchaseByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(result.PurchaseID),
	})
	if pCheck.AmountRials != workshopPrice || pCheck.TitleSnapshot != workshopTitle {
		t.Fatalf("immutable snapshot violated! expected price %d and title %q, got price %d and title %q",
			workshopPrice, workshopTitle, pCheck.AmountRials, pCheck.TitleSnapshot)
	}
}

func TestInitiatePayment_UncertainResultAndTimeout(t *testing.T) {
	ctx := context.Background()
	payMock := newMockPaymentsQuerier()
	wsMock := newMockWorkshopQuerier()
	prov := payments.NewMockPaymentProvider()
	svc := payments.NewService(payMock, prov, wsMock)

	tenantID := uuid.New()
	wsID := uuid.New()
	regID := uuid.New()
	now := time.Now()

	wsMock.workshops[wsID.String()] = workshopsdb.Workshop{
		ID:         toPgUUID(wsID),
		TenantID:   toPgUUID(tenantID),
		Title:      "کارگاه تست تایم‌اوت",
		PriceRials: 3000000,
		Status:     "published",
	}
	wsMock.registrations[regID.String()] = workshopsdb.WorkshopRegistration{
		ID:         toPgUUID(regID),
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(wsID),
		GuestName:  "علی حسنی",
		GuestPhone: "09121112233",
		Status:     "pending_payment",
		ExpiresAt:  pgtype.Timestamptz{Time: now.Add(15 * time.Minute), Valid: true},
	}

	// 1. Simulate network timeout during gateway request ("مسیر شروع با نتیجهٔ نامعلوم")
	prov.SimulateTimeout = true
	idempotencyKey := "key_timeout_test"

	_, err := svc.InitiateWorkshopPayment(ctx, payments.InitiateWorkshopPaymentParams{
		TenantID:       tenantID,
		RegistrationID: regID,
		IdempotencyKey: idempotencyKey,
		CallbackURL:    "https://clinic.ir/callbacks/payments/zarinpal",
		Now:            now,
	})
	if !errors.Is(err, payments.ErrGatewayTimeout) {
		t.Fatalf("expected ErrGatewayTimeout, got: %v", err)
	}

	// Verify that NO bogus payment_attempts was saved with empty authority
	_, err = payMock.GetPaymentAttemptByIdempotencyKey(ctx, paymentsdb.GetPaymentAttemptByIdempotencyKeyParams{
		TenantID:       toPgUUID(tenantID),
		IdempotencyKey: idempotencyKey,
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("no payment attempt should exist after timeout, got: %v", err)
	}

	// Verify purchase exists in 'pending' status
	purchasesList, _ := payMock.ListPurchases(ctx, paymentsdb.ListPurchasesParams{TenantID: toPgUUID(tenantID)})
	if len(purchasesList) != 1 || purchasesList[0].Status != "pending" {
		t.Fatalf("expected 1 pending purchase, got: %+v", purchasesList)
	}

	// 2. Gateway recovers: client retries with the SAME idempotency key
	prov.SimulateTimeout = false

	retryResult, err := svc.InitiateWorkshopPayment(ctx, payments.InitiateWorkshopPaymentParams{
		TenantID:       tenantID,
		RegistrationID: regID,
		IdempotencyKey: idempotencyKey,
		CallbackURL:    "https://clinic.ir/callbacks/payments/zarinpal",
		Now:            now.Add(10 * time.Second),
	})
	if err != nil {
		t.Fatalf("retry after gateway recovery failed: %v", err)
	}

	if retryResult.Authority == "" || retryResult.PaymentURL == "" {
		t.Fatalf("expected successful authority and payment url on retry, got: %+v", retryResult)
	}

	// Now payment_attempt exists with recorded authority and status 'initiated'
	recordedAttempt, err := payMock.GetPaymentAttemptByIdempotencyKey(ctx, paymentsdb.GetPaymentAttemptByIdempotencyKeyParams{
		TenantID:       toPgUUID(tenantID),
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		t.Fatalf("expected recorded attempt in DB: %v", err)
	}
	if recordedAttempt.Status != "initiated" || recordedAttempt.Authority != retryResult.Authority {
		t.Errorf("mismatched recorded attempt: %+v", recordedAttempt)
	}
}

func TestInitiatePayment_IdempotencyWithSameKey(t *testing.T) {
	ctx := context.Background()
	payMock := newMockPaymentsQuerier()
	wsMock := newMockWorkshopQuerier()
	prov := payments.NewMockPaymentProvider()
	svc := payments.NewService(payMock, prov, wsMock)

	tenantID := uuid.New()
	wsID := uuid.New()
	regID := uuid.New()
	now := time.Now()

	wsMock.workshops[wsID.String()] = workshopsdb.Workshop{
		ID:         toPgUUID(wsID),
		TenantID:   toPgUUID(tenantID),
		Title:      "کارگاه آیدمپوتنسی",
		PriceRials: 4000000,
		Status:     "published",
	}
	wsMock.registrations[regID.String()] = workshopsdb.WorkshopRegistration{
		ID:         toPgUUID(regID),
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(wsID),
		GuestName:  "رضا کریمی",
		GuestPhone: "09128889900",
		Status:     "pending_payment",
		ExpiresAt:  pgtype.Timestamptz{Time: now.Add(15 * time.Minute), Valid: true},
	}

	idempotencyKey := "idemp_same_key_test"
	res1, err := svc.InitiateWorkshopPayment(ctx, payments.InitiateWorkshopPaymentParams{
		TenantID:       tenantID,
		RegistrationID: regID,
		IdempotencyKey: idempotencyKey,
		CallbackURL:    "https://clinic.ir/cb",
		Now:            now,
	})
	if err != nil {
		t.Fatalf("first initiate failed: %v", err)
	}

	callCountAfterFirst := len(prov.RequestCalls)
	if callCountAfterFirst != 1 {
		t.Fatalf("expected exactly 1 provider call, got %d", callCountAfterFirst)
	}

	// Second initiate call with IDENTICAL idempotency key
	res2, err := svc.InitiateWorkshopPayment(ctx, payments.InitiateWorkshopPaymentParams{
		TenantID:       tenantID,
		RegistrationID: regID,
		IdempotencyKey: idempotencyKey,
		CallbackURL:    "https://clinic.ir/cb",
		Now:            now.Add(5 * time.Second),
	})
	if err != nil {
		t.Fatalf("second initiate failed: %v", err)
	}

	// Must NOT call payment gateway a second time!
	if len(prov.RequestCalls) != callCountAfterFirst {
		t.Fatalf("idempotency violated: gateway called %d times instead of %d", len(prov.RequestCalls), callCountAfterFirst)
	}

	if res2.Authority != res1.Authority || res2.PaymentAttemptID != res1.PaymentAttemptID {
		t.Fatalf("expected identical result on idempotent retry, got res1=%+v res2=%+v", res1, res2)
	}
}

func TestInitiatePayment_ActiveAttemptInProgressBlocked(t *testing.T) {
	ctx := context.Background()
	payMock := newMockPaymentsQuerier()
	wsMock := newMockWorkshopQuerier()
	prov := payments.NewMockPaymentProvider()
	svc := payments.NewService(payMock, prov, wsMock)

	tenantID := uuid.New()
	wsID := uuid.New()
	regID := uuid.New()
	now := time.Now()

	wsMock.workshops[wsID.String()] = workshopsdb.Workshop{
		ID:         toPgUUID(wsID),
		TenantID:   toPgUUID(tenantID),
		Title:      "کارگاه هم‌روندی پرداخت",
		PriceRials: 1500000,
		Status:     "published",
	}
	wsMock.registrations[regID.String()] = workshopsdb.WorkshopRegistration{
		ID:         toPgUUID(regID),
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(wsID),
		GuestName:  "نرگس افشار",
		GuestPhone: "09127776655",
		Status:     "pending_payment",
		ExpiresAt:  pgtype.Timestamptz{Time: now.Add(15 * time.Minute), Valid: true},
	}

	// First attempt with key A
	_, err := svc.InitiateWorkshopPayment(ctx, payments.InitiateWorkshopPaymentParams{
		TenantID:       tenantID,
		RegistrationID: regID,
		IdempotencyKey: "key_A",
		CallbackURL:    "https://clinic.ir/cb",
		Now:            now,
	})
	if err != nil {
		t.Fatalf("first attempt failed: %v", err)
	}

	// Second attempt with DIFFERENT key B while previous attempt is still active
	// Rule: "تا زمان تعیین تکلیف تلاش قبلی، همان خرید دوباره به درگاه فرستاده نمی‌شود."
	_, err = svc.InitiateWorkshopPayment(ctx, payments.InitiateWorkshopPaymentParams{
		TenantID:       tenantID,
		RegistrationID: regID,
		IdempotencyKey: "key_B",
		CallbackURL:    "https://clinic.ir/cb",
		Now:            now.Add(1 * time.Minute),
	})
	if !errors.Is(err, payments.ErrActiveAttemptInProgress) {
		t.Fatalf("expected ErrActiveAttemptInProgress, got: %v", err)
	}
}

func TestInitiatePayment_ExpiredRegistrationBlocked(t *testing.T) {
	ctx := context.Background()
	payMock := newMockPaymentsQuerier()
	wsMock := newMockWorkshopQuerier()
	prov := payments.NewMockPaymentProvider()
	svc := payments.NewService(payMock, prov, wsMock)

	tenantID := uuid.New()
	wsID := uuid.New()
	regID := uuid.New()
	now := time.Now()

	wsMock.workshops[wsID.String()] = workshopsdb.Workshop{
		ID:         toPgUUID(wsID),
		TenantID:   toPgUUID(tenantID),
		Title:      "کارگاه منقضی",
		PriceRials: 2000000,
		Status:     "published",
	}
	// Expired hold (expired 5 minutes ago)
	wsMock.registrations[regID.String()] = workshopsdb.WorkshopRegistration{
		ID:         toPgUUID(regID),
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(wsID),
		GuestName:  "کاربر دیرهنگام",
		GuestPhone: "09121239876",
		Status:     "pending_payment",
		ExpiresAt:  pgtype.Timestamptz{Time: now.Add(-5 * time.Minute), Valid: true},
	}

	_, err := svc.InitiateWorkshopPayment(ctx, payments.InitiateWorkshopPaymentParams{
		TenantID:       tenantID,
		RegistrationID: regID,
		IdempotencyKey: "key_expired",
		CallbackURL:    "https://clinic.ir/cb",
		Now:            now,
	})
	if !errors.Is(err, payments.ErrRegistrationExpired) {
		t.Fatalf("expected ErrRegistrationExpired, got: %v", err)
	}
}

func TestInitiatePayment_CrossTenantIsolation(t *testing.T) {
	ctx := context.Background()
	payMock := newMockPaymentsQuerier()
	wsMock := newMockWorkshopQuerier()
	prov := payments.NewMockPaymentProvider()
	svc := payments.NewService(payMock, prov, wsMock)

	tenantA := uuid.New()
	tenantB := uuid.New()
	wsID := uuid.New()
	regID := uuid.New()
	now := time.Now()

	wsMock.workshops[wsID.String()] = workshopsdb.Workshop{
		ID:         toPgUUID(wsID),
		TenantID:   toPgUUID(tenantA),
		Title:      "کارگاه کلینیک الف",
		PriceRials: 1000000,
		Status:     "published",
	}
	wsMock.registrations[regID.String()] = workshopsdb.WorkshopRegistration{
		ID:         toPgUUID(regID),
		TenantID:   toPgUUID(tenantA),
		WorkshopID: toPgUUID(wsID),
		GuestName:  "مهمان الف",
		GuestPhone: "09121111111",
		Status:     "pending_payment",
		ExpiresAt:  pgtype.Timestamptz{Time: now.Add(15 * time.Minute), Valid: true},
	}

	// Tenant B tries to initiate payment for Tenant A's registration
	_, err := svc.InitiateWorkshopPayment(ctx, payments.InitiateWorkshopPaymentParams{
		TenantID:       tenantB,
		RegistrationID: regID,
		IdempotencyKey: "key_cross_tenant",
		CallbackURL:    "https://clinic-b.ir/cb",
		Now:            now,
	})
	if !errors.Is(err, payments.ErrRegistrationNotFound) {
		t.Fatalf("expected ErrRegistrationNotFound for cross-tenant payment initiation, got: %v", err)
	}
}

// ----------------------------------------------------
// Mock Domain Querier for Payment Callback Tests
// ----------------------------------------------------

type mockDomainQuerier struct {
	domains map[string]domaindb.Domain
}

func (m *mockDomainQuerier) GetDomainByHost(ctx context.Context, host string) (domaindb.Domain, error) {
	d, ok := m.domains[host]
	if !ok {
		return domaindb.Domain{}, pgx.ErrNoRows
	}
	return d, nil
}

// ----------------------------------------------------
// PSY-1.6: Callback & Server-Side Verify Tests
// ----------------------------------------------------

func TestCallback_SuccessfulPaymentAndHoldConfirmation(t *testing.T) {
	ctx := context.Background()
	payMock := newMockPaymentsQuerier()
	wsMock := newMockWorkshopQuerier()
	prov := payments.NewMockPaymentProvider()
	svc := payments.NewService(payMock, prov, wsMock)

	tenantID := uuid.New()
	wsID := uuid.New()
	regID := uuid.New()
	now := time.Now()

	wsMock.workshops[wsID.String()] = workshopsdb.Workshop{
		ID:         toPgUUID(wsID),
		TenantID:   toPgUUID(tenantID),
		Title:      "کارگاه زوج‌درمانی پیشرفته",
		PriceRials: 6000000,
		Status:     "published",
	}
	wsMock.registrations[regID.String()] = workshopsdb.WorkshopRegistration{
		ID:         toPgUUID(regID),
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(wsID),
		GuestName:  "مهسا باقری",
		GuestPhone: "09121234567",
		Status:     "pending_payment",
		ExpiresAt:  pgtype.Timestamptz{Time: now.Add(15 * time.Minute), Valid: true},
	}

	// 1. Initiate payment to get authority
	initRes, err := svc.InitiateWorkshopPayment(ctx, payments.InitiateWorkshopPaymentParams{
		TenantID:       tenantID,
		RegistrationID: regID,
		IdempotencyKey: "cb_succ_key",
		CallbackURL:    "https://clinic.ir/cb",
		Now:            now,
	})
	if err != nil {
		t.Fatalf("initiate payment failed: %v", err)
	}

	// 2. Gateway returns customer via Callback with Status=OK and Authority
	res, err := svc.HandleCallbackAndVerify(ctx, tenantID, initRes.Authority, "OK", now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("handle callback failed: %v", err)
	}

	if res.Status != "confirmed" {
		t.Fatalf("expected status 'confirmed', got: %s", res.Status)
	}
	if res.RefID == "" {
		t.Fatal("expected non-empty RefID from verified gateway response")
	}

	// 3. Verify in DB:
	// Registration must be 'confirmed'
	regCheck, _ := wsMock.GetRegistrationByID(ctx, workshopsdb.GetRegistrationByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(regID),
	})
	if regCheck.Status != "confirmed" {
		t.Errorf("expected registration status 'confirmed', got %s", regCheck.Status)
	}

	// Purchase must be 'completed'
	purchCheck, _ := payMock.GetPurchaseByID(ctx, paymentsdb.GetPurchaseByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(initRes.PurchaseID),
	})
	if purchCheck.Status != "completed" {
		t.Errorf("expected purchase status 'completed', got %s", purchCheck.Status)
	}

	// Payment attempt must be 'verified'
	attCheck, _ := payMock.GetPaymentAttemptByAuthority(ctx, paymentsdb.GetPaymentAttemptByAuthorityParams{
		TenantID:  toPgUUID(tenantID),
		Authority: initRes.Authority,
	})
	if attCheck.Status != "verified" || attCheck.GatewayRefID.String != res.RefID {
		t.Errorf("expected attempt 'verified' with refID %s, got status %s ref %s", res.RefID, attCheck.Status, attCheck.GatewayRefID.String)
	}
}

func TestCallback_IdempotentDuplicateCallback(t *testing.T) {
	ctx := context.Background()
	payMock := newMockPaymentsQuerier()
	wsMock := newMockWorkshopQuerier()
	prov := payments.NewMockPaymentProvider()
	svc := payments.NewService(payMock, prov, wsMock)

	tenantID := uuid.New()
	wsID := uuid.New()
	regID := uuid.New()
	now := time.Now()

	wsMock.workshops[wsID.String()] = workshopsdb.Workshop{
		ID:         toPgUUID(wsID),
		TenantID:   toPgUUID(tenantID),
		Title:      "کارگاه تست آیدمپوتنت کال‌بک",
		PriceRials: 4000000,
		Status:     "published",
	}
	wsMock.registrations[regID.String()] = workshopsdb.WorkshopRegistration{
		ID:         toPgUUID(regID),
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(wsID),
		GuestName:  "سینا کاظمی",
		GuestPhone: "09124443322",
		Status:     "pending_payment",
		ExpiresAt:  pgtype.Timestamptz{Time: now.Add(15 * time.Minute), Valid: true},
	}

	initRes, _ := svc.InitiateWorkshopPayment(ctx, payments.InitiateWorkshopPaymentParams{
		TenantID:       tenantID,
		RegistrationID: regID,
		IdempotencyKey: "cb_idemp_key",
		CallbackURL:    "https://clinic.ir/cb",
		Now:            now,
	})

	// First callback
	res1, err := svc.HandleCallbackAndVerify(ctx, tenantID, initRes.Authority, "OK", now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("first callback failed: %v", err)
	}

	verifyCallsAfterFirst := len(prov.VerifyCalls)
	if verifyCallsAfterFirst != 1 {
		t.Fatalf("expected exactly 1 verify call to gateway, got %d", verifyCallsAfterFirst)
	}

	// Second duplicate callback (e.g. user refreshed browser or gateway retried callback)
	res2, err := svc.HandleCallbackAndVerify(ctx, tenantID, initRes.Authority, "OK", now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("second callback failed: %v", err)
	}

	// Must NOT call gateway verify again!
	if len(prov.VerifyCalls) != verifyCallsAfterFirst {
		t.Fatalf("gateway verify called %d times instead of %d (duplicate callback violated idempotency)", len(prov.VerifyCalls), verifyCallsAfterFirst)
	}

	if res2.Status != res1.Status || res2.RefID != res1.RefID {
		t.Fatalf("expected identical callback result on retry, got res1=%+v res2=%+v", res1, res2)
	}
}

func TestCallback_UserCanceledOnGateway(t *testing.T) {
	ctx := context.Background()
	payMock := newMockPaymentsQuerier()
	wsMock := newMockWorkshopQuerier()
	prov := payments.NewMockPaymentProvider()
	svc := payments.NewService(payMock, prov, wsMock)

	tenantID := uuid.New()
	wsID := uuid.New()
	regID := uuid.New()
	now := time.Now()

	wsMock.workshops[wsID.String()] = workshopsdb.Workshop{
		ID:         toPgUUID(wsID),
		TenantID:   toPgUUID(tenantID),
		Title:      "کارگاه تست انصراف درگاه",
		PriceRials: 2000000,
		Status:     "published",
	}
	wsMock.registrations[regID.String()] = workshopsdb.WorkshopRegistration{
		ID:         toPgUUID(regID),
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(wsID),
		GuestName:  "فرهاد فراهانی",
		GuestPhone: "09129990011",
		Status:     "pending_payment",
		ExpiresAt:  pgtype.Timestamptz{Time: now.Add(15 * time.Minute), Valid: true},
	}

	initRes, _ := svc.InitiateWorkshopPayment(ctx, payments.InitiateWorkshopPaymentParams{
		TenantID:       tenantID,
		RegistrationID: regID,
		IdempotencyKey: "cb_cancel_key",
		CallbackURL:    "https://clinic.ir/cb",
		Now:            now,
	})

	// User clicked "Cancel" on gateway -> status=NOK
	res, err := svc.HandleCallbackAndVerify(ctx, tenantID, initRes.Authority, "NOK", now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error handling canceled callback: %v", err)
	}

	if res.Status != "failed" {
		t.Errorf("expected status 'failed', got: %s", res.Status)
	}

	// Gateway verify must NOT be called for NOK
	if len(prov.VerifyCalls) != 0 {
		t.Errorf("expected 0 gateway verify calls for user cancel, got %d", len(prov.VerifyCalls))
	}

	// In DB: attempt and purchase marked failed
	att, _ := payMock.GetPaymentAttemptByAuthority(ctx, paymentsdb.GetPaymentAttemptByAuthorityParams{
		TenantID:  toPgUUID(tenantID),
		Authority: initRes.Authority,
	})
	if att.Status != "failed" {
		t.Errorf("expected attempt status 'failed', got %s", att.Status)
	}
}

func TestCallback_MismatchDetection(t *testing.T) {
	ctx := context.Background()
	payMock := newMockPaymentsQuerier()
	wsMock := newMockWorkshopQuerier()
	prov := payments.NewMockPaymentProvider()
	svc := payments.NewService(payMock, prov, wsMock)

	tenantA := uuid.New()
	tenantB := uuid.New()
	wsID := uuid.New()
	regID := uuid.New()
	now := time.Now()

	wsMock.workshops[wsID.String()] = workshopsdb.Workshop{
		ID:         toPgUUID(wsID),
		TenantID:   toPgUUID(tenantA),
		Title:      "کارگاه ناهماهنگی",
		PriceRials: 3000000,
		Status:     "published",
	}
	wsMock.registrations[regID.String()] = workshopsdb.WorkshopRegistration{
		ID:         toPgUUID(regID),
		TenantID:   toPgUUID(tenantA),
		WorkshopID: toPgUUID(wsID),
		GuestName:  "مهمان تست",
		GuestPhone: "09121111111",
		Status:     "pending_payment",
		ExpiresAt:  pgtype.Timestamptz{Time: now.Add(15 * time.Minute), Valid: true},
	}

	initRes, _ := svc.InitiateWorkshopPayment(ctx, payments.InitiateWorkshopPaymentParams{
		TenantID:       tenantA,
		RegistrationID: regID,
		IdempotencyKey: "mismatch_key",
		CallbackURL:    "https://clinic-a.ir/cb",
		Now:            now,
	})

	// 1. Cross-tenant Authority mismatch: Tenant B calls callback with Tenant A's authority
	_, err := svc.HandleCallbackAndVerify(ctx, tenantB, initRes.Authority, "OK", now.Add(2*time.Minute))
	if !errors.Is(err, payments.ErrPaymentAttemptNotFound) {
		t.Fatalf("expected ErrPaymentAttemptNotFound for cross-tenant authority, got: %v", err)
	}

	// 2. Amount Mismatch: attempt amount manipulated to differ from purchase
	for k, v := range payMock.paymentAttempts {
		if v.Authority == initRes.Authority {
			v.AmountRials = 999999999 // Mismatch!
			payMock.paymentAttempts[k] = v
			break
		}
	}

	_, err = svc.HandleCallbackAndVerify(ctx, tenantA, initRes.Authority, "OK", now.Add(2*time.Minute))
	if !errors.Is(err, payments.ErrAmountMismatch) {
		t.Fatalf("expected ErrAmountMismatch, got: %v", err)
	}
}

func TestCallback_PaymentAfterHoldExpired_RefundRequired(t *testing.T) {
	ctx := context.Background()
	payMock := newMockPaymentsQuerier()
	wsMock := newMockWorkshopQuerier()
	prov := payments.NewMockPaymentProvider()
	svc := payments.NewService(payMock, prov, wsMock)

	tenantID := uuid.New()
	wsID := uuid.New()
	regID := uuid.New()
	now := time.Now()

	wsMock.workshops[wsID.String()] = workshopsdb.Workshop{
		ID:         toPgUUID(wsID),
		TenantID:   toPgUUID(tenantID),
		Title:      "کارگاه انقضای رزرو",
		PriceRials: 5000000,
		Status:     "published",
	}
	// Hold was set with 15 minutes TTL at T0
	wsMock.registrations[regID.String()] = workshopsdb.WorkshopRegistration{
		ID:         toPgUUID(regID),
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(wsID),
		GuestName:  "مشتری با تاخیر",
		GuestPhone: "09128887766",
		Status:     "pending_payment",
		ExpiresAt:  pgtype.Timestamptz{Time: now.Add(15 * time.Minute), Valid: true},
	}

	initRes, _ := svc.InitiateWorkshopPayment(ctx, payments.InitiateWorkshopPaymentParams{
		TenantID:       tenantID,
		RegistrationID: regID,
		IdempotencyKey: "refund_expired_key",
		CallbackURL:    "https://clinic.ir/cb",
		Now:            now,
	})

	// Callback arrives at T0 + 16 minutes (HOLD HAS EXPIRED!)
	// Customer paid money on ZarinPal, but their seat reservation expired!
	// Rule: "اگر نگه‌داشت منقضی یا کارگاه لغو شده باشد، پرداخت موفق به refund_required می‌رود... صندلی جدید نمی‌گیرد."
	callbackTime := now.Add(16 * time.Minute)
	res, err := svc.HandleCallbackAndVerify(ctx, tenantID, initRes.Authority, "OK", callbackTime)
	if err != nil {
		t.Fatalf("callback failed: %v", err)
	}

	if res.Status != "refund_required" {
		t.Fatalf("expected status 'refund_required', got: %s", res.Status)
	}

	// Verify in DB:
	// 1. Purchase is in refund_required
	pCheck, _ := payMock.GetPurchaseByID(ctx, paymentsdb.GetPurchaseByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(initRes.PurchaseID),
	})
	if pCheck.Status != "refund_required" {
		t.Errorf("expected purchase status 'refund_required', got %s", pCheck.Status)
	}

	// 2. Payment attempt is verified (money was actually received)
	attCheck, _ := payMock.GetPaymentAttemptByAuthority(ctx, paymentsdb.GetPaymentAttemptByAuthorityParams{
		TenantID:  toPgUUID(tenantID),
		Authority: initRes.Authority,
	})
	if attCheck.Status != "verified" {
		t.Errorf("expected attempt status 'verified', got %s", attCheck.Status)
	}

	// 3. Registration must NOT be confirmed (seat released)!
	regCheck, _ := wsMock.GetRegistrationByID(ctx, workshopsdb.GetRegistrationByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(regID),
	})
	if regCheck.Status == "confirmed" {
		t.Fatal("oversell violation: expired registration must NOT be confirmed!")
	}
	if regCheck.Status != "expired" {
		t.Errorf("expected registration status 'expired', got %s", regCheck.Status)
	}
}

func TestCallback_PaymentAfterWorkshopCancelled_RefundRequired(t *testing.T) {
	ctx := context.Background()
	payMock := newMockPaymentsQuerier()
	wsMock := newMockWorkshopQuerier()
	prov := payments.NewMockPaymentProvider()
	svc := payments.NewService(payMock, prov, wsMock)

	tenantID := uuid.New()
	wsID := uuid.New()
	regID := uuid.New()
	now := time.Now()

	wsMock.workshops[wsID.String()] = workshopsdb.Workshop{
		ID:         toPgUUID(wsID),
		TenantID:   toPgUUID(tenantID),
		Title:      "کارگاه لغوشده توسط مدیر",
		PriceRials: 3500000,
		Status:     "published",
	}
	wsMock.registrations[regID.String()] = workshopsdb.WorkshopRegistration{
		ID:         toPgUUID(regID),
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(wsID),
		GuestName:  "مشتری کارگاه لغو شده",
		GuestPhone: "09125554433",
		Status:     "pending_payment",
		ExpiresAt:  pgtype.Timestamptz{Time: now.Add(15 * time.Minute), Valid: true},
	}

	initRes, _ := svc.InitiateWorkshopPayment(ctx, payments.InitiateWorkshopPaymentParams{
		TenantID:       tenantID,
		RegistrationID: regID,
		IdempotencyKey: "refund_cancel_key",
		CallbackURL:    "https://clinic.ir/cb",
		Now:            now,
	})

	// While customer is on the gateway, clinic owner cancels the workshop!
	wsMod := wsMock.workshops[wsID.String()]
	wsMod.Status = "cancelled"
	wsMock.workshops[wsID.String()] = wsMod

	// Customer completes payment and returns
	res, err := svc.HandleCallbackAndVerify(ctx, tenantID, initRes.Authority, "OK", now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("callback failed: %v", err)
	}

	if res.Status != "refund_required" {
		t.Fatalf("expected status 'refund_required' for cancelled workshop, got: %s", res.Status)
	}

	pCheck, _ := payMock.GetPurchaseByID(ctx, paymentsdb.GetPurchaseByIDParams{
		TenantID: toPgUUID(tenantID),
		ID:       toPgUUID(initRes.PurchaseID),
	})
	if pCheck.Status != "refund_required" {
		t.Errorf("expected purchase status 'refund_required', got %s", pCheck.Status)
	}
}

func TestCallback_HTTPHandlerIntegration(t *testing.T) {
	ctx := context.Background()
	payMock := newMockPaymentsQuerier()
	wsMock := newMockWorkshopQuerier()
	prov := payments.NewMockPaymentProvider()
	svc := payments.NewService(payMock, prov, wsMock)

	tenantID := uuid.New()
	wsID := uuid.New()
	regID := uuid.New()
	now := time.Now()

	domainMock := &mockDomainQuerier{
		domains: map[string]domaindb.Domain{
			"clinic.ir": {
				ID:         toPgUUID(uuid.New()),
				TenantID:   toPgUUID(tenantID),
				Host:       "clinic.ir",
				IsVerified: true,
			},
		},
	}
	resolver := domain.NewResolver(domainMock)
	handler := payments.NewHandler(svc, resolver, nil)

	r := chi.NewRouter()
	handler.MountRoutes(r)

	wsMock.workshops[wsID.String()] = workshopsdb.Workshop{
		ID:         toPgUUID(wsID),
		TenantID:   toPgUUID(tenantID),
		Title:      "کارگاه روان‌درمانی تحلیلی",
		PriceRials: 5000000,
		Status:     "published",
	}
	wsMock.registrations[regID.String()] = workshopsdb.WorkshopRegistration{
		ID:         toPgUUID(regID),
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(wsID),
		GuestName:  "نگار صادقی",
		GuestPhone: "09121113355",
		Status:     "pending_payment",
		ExpiresAt:  pgtype.Timestamptz{Time: now.Add(15 * time.Minute), Valid: true},
	}

	initRes, _ := svc.InitiateWorkshopPayment(ctx, payments.InitiateWorkshopPaymentParams{
		TenantID:       tenantID,
		RegistrationID: regID,
		IdempotencyKey: "http_cb_key",
		CallbackURL:    "https://clinic.ir/callbacks/payments/zarinpal",
		Now:            now,
	})

	// 1. GET /callbacks/payments/zarinpal?Authority=...&Status=OK
	req := httptest.NewRequest("GET", "/callbacks/payments/zarinpal?Authority="+initRes.Authority+"&Status=OK", nil)
	req.Host = "clinic.ir"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Must redirect 302 to registration status page
	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 Found, got: %d (%s)", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected Location header in 302 redirect")
	}
	expectedPrefix := "/workshops/registrations/" + regID.String() + "?payment_status=confirmed"
	if loc[:len(expectedPrefix)] != expectedPrefix {
		t.Fatalf("expected redirect to %s, got %s", expectedPrefix, loc)
	}

	// 2. Missing Authority -> 400 Bad Request
	reqNoAuth := httptest.NewRequest("GET", "/callbacks/payments/zarinpal?Status=OK", nil)
	reqNoAuth.Host = "clinic.ir"
	recNoAuth := httptest.NewRecorder()
	r.ServeHTTP(recNoAuth, reqNoAuth)

	if recNoAuth.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for missing Authority, got: %d", recNoAuth.Code)
	}

	// 3. Unknown Domain -> 404 Not Found
	reqBadHost := httptest.NewRequest("GET", "/callbacks/payments/zarinpal?Authority="+initRes.Authority+"&Status=OK", nil)
	reqBadHost.Host = "unknown-clinic.ir"
	recBadHost := httptest.NewRecorder()
	r.ServeHTTP(recBadHost, reqBadHost)

	if recBadHost.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown host, got: %d", recBadHost.Code)
	}
}

func TestReconcileStalePayments_AutoConfirmAndRefund(t *testing.T) {
	ctx := context.Background()
	payMock := newMockPaymentsQuerier()
	wsMock := newMockWorkshopQuerier()
	prov := payments.NewMockPaymentProvider()
	svc := payments.NewService(payMock, prov, wsMock)

	tenantID := uuid.New()
	now := time.Now()
	staleTime := now.Add(-30 * time.Minute)

	// Item 1: Paid on gateway, hold still valid -> Auto confirms
	ws1ID := uuid.New()
	wsMock.workshops[ws1ID.String()] = workshopsdb.Workshop{
		ID:         toPgUUID(ws1ID),
		TenantID:   toPgUUID(tenantID),
		Title:      "کارگاه یک",
		PriceRials: 1000000,
		Status:     "published",
	}
	reg1ID := uuid.New()
	p1ID := uuid.New()
	wsMock.registrations[reg1ID.String()] = workshopsdb.WorkshopRegistration{
		ID:         toPgUUID(reg1ID),
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(ws1ID),
		Status:     "pending_payment",
		ExpiresAt:  pgtype.Timestamptz{Time: now.Add(10 * time.Minute), Valid: true},
		PurchaseID: toPgUUID(p1ID),
	}
	payMock.purchases[p1ID.String()] = paymentsdb.Purchase{
		ID:            toPgUUID(p1ID),
		TenantID:      toPgUUID(tenantID),
		ItemType:      "workshop_registration",
		ItemID:        toPgUUID(reg1ID),
		TitleSnapshot: "کارگاه یک",
		AmountRials:   1000000,
		Status:        "pending",
		CreatedAt:     pgtype.Timestamptz{Time: staleTime, Valid: true},
	}
	att1ID := uuid.New()
	auth1 := "AUTH_RECON_CONFIRM_1"
	payMock.paymentAttempts[att1ID.String()] = paymentsdb.PaymentAttempt{
		ID:          toPgUUID(att1ID),
		TenantID:    toPgUUID(tenantID),
		PurchaseID:  toPgUUID(p1ID),
		Authority:   auth1,
		AmountRials: 1000000,
		Status:      "initiated",
		CreatedAt:   pgtype.Timestamptz{Time: staleTime, Valid: true},
	}

	// Item 2: Paid on gateway, but hold expired -> refund_required, reg expired (0 seats)
	ws2ID := uuid.New()
	wsMock.workshops[ws2ID.String()] = workshopsdb.Workshop{
		ID:         toPgUUID(ws2ID),
		TenantID:   toPgUUID(tenantID),
		Title:      "کارگاه دو",
		PriceRials: 2000000,
		Status:     "published",
	}
	reg2ID := uuid.New()
	p2ID := uuid.New()
	wsMock.registrations[reg2ID.String()] = workshopsdb.WorkshopRegistration{
		ID:         toPgUUID(reg2ID),
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(ws2ID),
		Status:     "pending_payment",
		ExpiresAt:  pgtype.Timestamptz{Time: now.Add(-5 * time.Minute), Valid: true}, // expired
		PurchaseID: toPgUUID(p2ID),
	}
	payMock.purchases[p2ID.String()] = paymentsdb.Purchase{
		ID:            toPgUUID(p2ID),
		TenantID:      toPgUUID(tenantID),
		ItemType:      "workshop_registration",
		ItemID:        toPgUUID(reg2ID),
		TitleSnapshot: "کارگاه دو",
		AmountRials:   2000000,
		Status:        "pending",
		CreatedAt:     pgtype.Timestamptz{Time: staleTime, Valid: true},
	}
	att2ID := uuid.New()
	auth2 := "AUTH_RECON_REFUND_2"
	payMock.paymentAttempts[att2ID.String()] = paymentsdb.PaymentAttempt{
		ID:          toPgUUID(att2ID),
		TenantID:    toPgUUID(tenantID),
		PurchaseID:  toPgUUID(p2ID),
		Authority:   auth2,
		AmountRials: 2000000,
		Status:      "initiated",
		CreatedAt:   pgtype.Timestamptz{Time: staleTime, Valid: true},
	}

	// Item 3: Customer never paid (gateway verification rejects)
	ws3ID := uuid.New()
	wsMock.workshops[ws3ID.String()] = workshopsdb.Workshop{
		ID:         toPgUUID(ws3ID),
		TenantID:   toPgUUID(tenantID),
		Title:      "کارگاه سه",
		PriceRials: 3000000,
		Status:     "published",
	}
	reg3ID := uuid.New()
	p3ID := uuid.New()
	wsMock.registrations[reg3ID.String()] = workshopsdb.WorkshopRegistration{
		ID:         toPgUUID(reg3ID),
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(ws3ID),
		Status:     "pending_payment",
		ExpiresAt:  pgtype.Timestamptz{Time: now.Add(10 * time.Minute), Valid: true},
		PurchaseID: toPgUUID(p3ID),
	}
	payMock.purchases[p3ID.String()] = paymentsdb.Purchase{
		ID:            toPgUUID(p3ID),
		TenantID:      toPgUUID(tenantID),
		ItemType:      "workshop_registration",
		ItemID:        toPgUUID(reg3ID),
		TitleSnapshot: "کارگاه سه",
		AmountRials:   3000000,
		Status:        "pending",
		CreatedAt:     pgtype.Timestamptz{Time: staleTime, Valid: true},
	}
	att3ID := uuid.New()
	auth3 := "AUTH_RECON_FAIL_3"
	payMock.paymentAttempts[att3ID.String()] = paymentsdb.PaymentAttempt{
		ID:          toPgUUID(att3ID),
		TenantID:    toPgUUID(tenantID),
		PurchaseID:  toPgUUID(p3ID),
		Authority:   auth3,
		AmountRials: 3000000,
		Status:      "initiated",
		CreatedAt:   pgtype.Timestamptz{Time: staleTime, Valid: true},
	}

	// Item 4: Gateway timeout / network transient issue
	ws4ID := uuid.New()
	wsMock.workshops[ws4ID.String()] = workshopsdb.Workshop{
		ID:         toPgUUID(ws4ID),
		TenantID:   toPgUUID(tenantID),
		Title:      "کارگاه چهار",
		PriceRials: 4000000,
		Status:     "published",
	}
	reg4ID := uuid.New()
	p4ID := uuid.New()
	wsMock.registrations[reg4ID.String()] = workshopsdb.WorkshopRegistration{
		ID:         toPgUUID(reg4ID),
		TenantID:   toPgUUID(tenantID),
		WorkshopID: toPgUUID(ws4ID),
		Status:     "pending_payment",
		ExpiresAt:  pgtype.Timestamptz{Time: now.Add(10 * time.Minute), Valid: true},
		PurchaseID: toPgUUID(p4ID),
	}
	payMock.purchases[p4ID.String()] = paymentsdb.Purchase{
		ID:            toPgUUID(p4ID),
		TenantID:      toPgUUID(tenantID),
		ItemType:      "workshop_registration",
		ItemID:        toPgUUID(reg4ID),
		TitleSnapshot: "کارگاه چهار",
		AmountRials:   4000000,
		Status:        "pending",
		CreatedAt:     pgtype.Timestamptz{Time: staleTime, Valid: true},
	}
	att4ID := uuid.New()
	auth4 := "AUTH_RECON_TIMEOUT_4"
	payMock.paymentAttempts[att4ID.String()] = paymentsdb.PaymentAttempt{
		ID:          toPgUUID(att4ID),
		TenantID:    toPgUUID(tenantID),
		PurchaseID:  toPgUUID(p4ID),
		Authority:   auth4,
		AmountRials: 4000000,
		Status:      "initiated",
		CreatedAt:   pgtype.Timestamptz{Time: staleTime, Valid: true},
	}

	prov.CustomVerifyHandler = func(ctx context.Context, req payments.VerifyRequest) (*payments.VerifyResponse, error) {
		switch req.Authority {
		case auth1:
			return &payments.VerifyResponse{RefID: "REF_RECON_1", Code: 100}, nil
		case auth2:
			return &payments.VerifyResponse{RefID: "REF_RECON_2", Code: 100}, nil
		case auth3:
			return nil, errors.New("gateway: payment not found or failed")
		case auth4:
			return nil, payments.ErrGatewayTimeout
		default:
			return &payments.VerifyResponse{RefID: "REF_DEFAULT", Code: 100}, nil
		}
	}

	// Execute reconciliation worker
	res, err := svc.ReconcileStalePayments(ctx, payments.ReconcileStalePaymentsParams{
		TenantID:  tenantID,
		OlderThan: 15 * time.Minute,
		Limit:     50,
		Now:       now,
	})
	if err != nil {
		t.Fatalf("unexpected error from ReconcileStalePayments: %v", err)
	}

	if res.ProcessedCount != 4 {
		t.Errorf("expected ProcessedCount=4, got %d", res.ProcessedCount)
	}
	if res.ConfirmedCount != 1 {
		t.Errorf("expected ConfirmedCount=1, got %d", res.ConfirmedCount)
	}
	if res.RefundRequired != 1 {
		t.Errorf("expected RefundRequired=1, got %d", res.RefundRequired)
	}
	if res.FailedCount != 1 {
		t.Errorf("expected FailedCount=1, got %d", res.FailedCount)
	}
	if res.TransientError != 1 {
		t.Errorf("expected TransientError=1, got %d", res.TransientError)
	}

	// Verify states in DB
	// Item 1: completed & confirmed
	p1 := payMock.purchases[p1ID.String()]
	if p1.Status != "completed" {
		t.Errorf("expected purchase 1 status 'completed', got '%s'", p1.Status)
	}
	r1 := wsMock.registrations[reg1ID.String()]
	if r1.Status != "confirmed" {
		t.Errorf("expected reg 1 status 'confirmed', got '%s'", r1.Status)
	}

	// Item 2: refund_required & expired
	p2 := payMock.purchases[p2ID.String()]
	if p2.Status != "refund_required" {
		t.Errorf("expected purchase 2 status 'refund_required', got '%s'", p2.Status)
	}
	r2 := wsMock.registrations[reg2ID.String()]
	if r2.Status != "expired" {
		t.Errorf("expected reg 2 status 'expired' (zero seats occupied), got '%s'", r2.Status)
	}

	// Item 3: failed
	p3 := payMock.purchases[p3ID.String()]
	if p3.Status != "failed" {
		t.Errorf("expected purchase 3 status 'failed', got '%s'", p3.Status)
	}

	// Item 4: left in initiated (safe for retry)
	att4 := payMock.paymentAttempts[att4ID.String()]
	if att4.Status != "initiated" {
		t.Errorf("expected attempt 4 to remain 'initiated' for retry, got '%s'", att4.Status)
	}
}

func TestReconcileStalePayments_CrashRecoveryAndRetry(t *testing.T) {
	ctx := context.Background()
	payMock := newMockPaymentsQuerier()
	wsMock := newMockWorkshopQuerier()
	prov := payments.NewMockPaymentProvider()
	svc := payments.NewService(payMock, prov, wsMock)

	tenantID := uuid.New()
	now := time.Now()
	staleTime := now.Add(-30 * time.Minute)

	wsID := uuid.New()
	wsMock.workshops[wsID.String()] = workshopsdb.Workshop{
		ID:         toPgUUID(wsID),
		TenantID:   toPgUUID(tenantID),
		Title:      "کارگاه بازیابی کرش",
		PriceRials: 1000000,
		Status:     "published",
	}

	for i := 1; i <= 2; i++ {
		regID := uuid.New()
		pID := uuid.New()
		attID := uuid.New()
		authCode := "AUTH_CRASH_" + string(rune('0'+i))

		wsMock.registrations[regID.String()] = workshopsdb.WorkshopRegistration{
			ID:         toPgUUID(regID),
			TenantID:   toPgUUID(tenantID),
			WorkshopID: toPgUUID(wsID),
			Status:     "pending_payment",
			ExpiresAt:  pgtype.Timestamptz{Time: now.Add(10 * time.Minute), Valid: true},
			PurchaseID: toPgUUID(pID),
		}
		payMock.purchases[pID.String()] = paymentsdb.Purchase{
			ID:            toPgUUID(pID),
			TenantID:      toPgUUID(tenantID),
			ItemType:      "workshop_registration",
			ItemID:        toPgUUID(regID),
			TitleSnapshot: "کارگاه بازیابی کرش",
			AmountRials:   1000000,
			Status:        "pending",
			CreatedAt:     pgtype.Timestamptz{Time: staleTime, Valid: true},
		}
		payMock.paymentAttempts[attID.String()] = paymentsdb.PaymentAttempt{
			ID:          toPgUUID(attID),
			TenantID:    toPgUUID(tenantID),
			PurchaseID:  toPgUUID(pID),
			Authority:   authCode,
			AmountRials: 1000000,
			Status:      "initiated",
			CreatedAt:   pgtype.Timestamptz{Time: staleTime, Valid: true},
		}
	}

	prov.CustomVerifyHandler = func(ctx context.Context, req payments.VerifyRequest) (*payments.VerifyResponse, error) {
		return &payments.VerifyResponse{RefID: "REF_" + req.Authority, Code: 100}, nil
	}

	// First batch: Limit = 1 (simulating process that crashes or terminates after 1 item)
	res1, err := svc.ReconcileStalePayments(ctx, payments.ReconcileStalePaymentsParams{
		TenantID:  tenantID,
		OlderThan: 15 * time.Minute,
		Limit:     1,
		Now:       now,
	})
	if err != nil {
		t.Fatalf("unexpected error on batch 1: %v", err)
	}
	if res1.ProcessedCount != 1 || res1.ConfirmedCount != 1 {
		t.Errorf("batch 1: expected 1 processed, 1 confirmed, got: %+v", res1)
	}

	// Process restarts / next tick: Limit = 1 again. It must seamlessly pick up the 2nd item!
	res2, err := svc.ReconcileStalePayments(ctx, payments.ReconcileStalePaymentsParams{
		TenantID:  tenantID,
		OlderThan: 15 * time.Minute,
		Limit:     1,
		Now:       now,
	})
	if err != nil {
		t.Fatalf("unexpected error on batch 2: %v", err)
	}
	if res2.ProcessedCount != 1 || res2.ConfirmedCount != 1 {
		t.Errorf("batch 2: expected 1 processed, 1 confirmed, got: %+v", res2)
	}

	// 3rd tick: nothing left to reconcile
	res3, err := svc.ReconcileStalePayments(ctx, payments.ReconcileStalePaymentsParams{
		TenantID:  tenantID,
		OlderThan: 15 * time.Minute,
		Limit:     1,
		Now:       now,
	})
	if err != nil {
		t.Fatalf("unexpected error on batch 3: %v", err)
	}
	if res3.ProcessedCount != 0 {
		t.Errorf("batch 3: expected 0 processed, got: %+v", res3)
	}
}

func TestAdminPayments_OwnerOnlyAccessAndListing(t *testing.T) {
	payMock := newMockPaymentsQuerier()
	wsMock := newMockWorkshopQuerier()
	prov := payments.NewMockPaymentProvider()
	svc := payments.NewService(payMock, prov, wsMock)

	tenantAID := uuid.New()
	tenantBID := uuid.New()

	ownerUID := uuid.New()
	receptionUID := uuid.New()

	mockAuth := &mockAuthQuerier{
		identities: map[string]authdb.PanelIdentity{
			ownerUID.String(): {
				ID:       toPgUUID(ownerUID),
				Email:    "owner@clinic-a.ir",
				FullName: "دکتر الف",
				Status:   "active",
			},
			receptionUID.String(): {
				ID:       toPgUUID(receptionUID),
				Email:    "reception@clinic-a.ir",
				FullName: "مسئول پذیرش",
				Status:   "active",
			},
		},
		sessions: map[string]authdb.PanelSession{
			auth.HashToken("token-owner"): {
				ID:        toPgUUID(uuid.New()),
				UserID:    toPgUUID(ownerUID),
				TokenHash: auth.HashToken("token-owner"),
			},
			auth.HashToken("token-reception"): {
				ID:        toPgUUID(uuid.New()),
				UserID:    toPgUUID(receptionUID),
				TokenHash: auth.HashToken("token-reception"),
			},
		},
		memberships: map[string]authdb.Membership{
			ownerUID.String() + ":" + tenantAID.String(): {
				ID:       toPgUUID(uuid.New()),
				TenantID: toPgUUID(tenantAID),
				UserID:   toPgUUID(ownerUID),
				Role:     "owner",
			},
			receptionUID.String() + ":" + tenantAID.String(): {
				ID:       toPgUUID(uuid.New()),
				TenantID: toPgUUID(tenantAID),
				UserID:   toPgUUID(receptionUID),
				Role:     "receptionist",
			},
		},
	}

	authSvc := auth.NewService(mockAuth)
	handler := payments.NewHandler(svc, nil, authSvc)

	r := chi.NewRouter()
	handler.MountRoutes(r)

	// Create test purchases
	// Tenant A purchases:
	pA1 := paymentsdb.Purchase{
		ID:            toPgUUID(uuid.New()),
		TenantID:      toPgUUID(tenantAID),
		ItemType:      "workshop_registration",
		ItemID:        toPgUUID(uuid.New()),
		TitleSnapshot: "کارگاه تکمیل‌شده الف",
		AmountRials:   5000000,
		Status:        "completed",
		CreatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	payMock.purchases[fromPgUUID(pA1.ID).String()] = pA1

	pA2 := paymentsdb.Purchase{
		ID:            toPgUUID(uuid.New()),
		TenantID:      toPgUUID(tenantAID),
		ItemType:      "workshop_registration",
		ItemID:        toPgUUID(uuid.New()),
		TitleSnapshot: "کارگاه نیازمند بازپرداخت ۱",
		AmountRials:   3000000,
		Status:        "refund_required",
		CreatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	payMock.purchases[fromPgUUID(pA2.ID).String()] = pA2
	// Add payment attempt with tracking code
	attA2 := paymentsdb.PaymentAttempt{
		ID:           toPgUUID(uuid.New()),
		TenantID:     toPgUUID(tenantAID),
		PurchaseID:   pA2.ID,
		Status:       "verified",
		TrackingCode: pgtype.Text{String: "TRK-222", Valid: true},
		CreatedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	payMock.paymentAttempts[fromPgUUID(attA2.ID).String()] = attA2

	pA3 := paymentsdb.Purchase{
		ID:            toPgUUID(uuid.New()),
		TenantID:      toPgUUID(tenantAID),
		ItemType:      "workshop_registration",
		ItemID:        toPgUUID(uuid.New()),
		TitleSnapshot: "کارگاه نیازمند بازپرداخت ۲",
		AmountRials:   4000000,
		Status:        "refund_required",
		CreatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	payMock.purchases[fromPgUUID(pA3.ID).String()] = pA3

	// Tenant B purchase (should never be seen by Tenant A owner)
	pB1 := paymentsdb.Purchase{
		ID:            toPgUUID(uuid.New()),
		TenantID:      toPgUUID(tenantBID),
		ItemType:      "workshop_registration",
		ItemID:        toPgUUID(uuid.New()),
		TitleSnapshot: "کارگاه کلینیک ب",
		AmountRials:   9000000,
		Status:        "refund_required",
		CreatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	payMock.purchases[fromPgUUID(pB1.ID).String()] = pB1

	// 1. Owner lists refund_required payments for Tenant A -> 200 OK
	req := httptest.NewRequest("GET", "/admin/payments?status=refund_required", nil)
	req.Header.Set("Authorization", "Bearer token-owner")
	req.Header.Set("X-Tenant-ID", tenantAID.String())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d (%s)", rec.Code, rec.Body.String())
	}

	var items []payments.AdminPurchaseItem
	if err := json.NewDecoder(rec.Body).Decode(&items); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items with refund_required, got: %d", len(items))
	}
	// Verify tracking code on item 1
	var foundTracking bool
	for _, it := range items {
		if it.TrackingCode != nil && *it.TrackingCode == "TRK-222" {
			foundTracking = true
		}
	}
	if !foundTracking {
		t.Errorf("expected to find tracking code TRK-222 in results")
	}

	// 2. Receptionist attempts to access /admin/payments -> 403 Forbidden (Owner only!)
	reqRecep := httptest.NewRequest("GET", "/admin/payments?status=refund_required", nil)
	reqRecep.Header.Set("Authorization", "Bearer token-reception")
	reqRecep.Header.Set("X-Tenant-ID", tenantAID.String())
	recRecep := httptest.NewRecorder()
	r.ServeHTTP(recRecep, reqRecep)

	if recRecep.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for receptionist, got: %d", recRecep.Code)
	}

	// 3. Cross-Tenant isolation: Owner of A attempts to view Tenant B -> 403 Forbidden
	reqCross := httptest.NewRequest("GET", "/admin/payments?status=refund_required", nil)
	reqCross.Header.Set("Authorization", "Bearer token-owner")
	reqCross.Header.Set("X-Tenant-ID", tenantBID.String())
	recCross := httptest.NewRecorder()
	r.ServeHTTP(recCross, reqCross)

	if recCross.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for cross-tenant access, got: %d", recCross.Code)
	}

	// 4. Missing auth token -> 401 Unauthorized
	reqNoAuth := httptest.NewRequest("GET", "/admin/payments", nil)
	reqNoAuth.Header.Set("X-Tenant-ID", tenantAID.String())
	recNoAuth := httptest.NewRecorder()
	r.ServeHTTP(recNoAuth, reqNoAuth)

	if recNoAuth.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got: %d", recNoAuth.Code)
	}
}

func TestRecordRefund_SuccessAndIdempotency(t *testing.T) {
	payMock := newMockPaymentsQuerier()
	wsMock := newMockWorkshopQuerier()
	prov := payments.NewMockPaymentProvider()
	svc := payments.NewService(payMock, prov, wsMock)

	tenantAID := uuid.New()
	ownerUID := uuid.New()
	receptionUID := uuid.New()

	mockAuth := &mockAuthQuerier{
		identities: map[string]authdb.PanelIdentity{
			ownerUID.String(): {
				ID:       toPgUUID(ownerUID),
				Email:    "owner@clinic-a.ir",
				FullName: "دکتر مالک",
				Status:   "active",
			},
			receptionUID.String(): {
				ID:       toPgUUID(receptionUID),
				Email:    "recep@clinic-a.ir",
				FullName: "پذیرش",
				Status:   "active",
			},
		},
		sessions: map[string]authdb.PanelSession{
			auth.HashToken("token-owner"): {
				ID:        toPgUUID(uuid.New()),
				UserID:    toPgUUID(ownerUID),
				TokenHash: auth.HashToken("token-owner"),
			},
			auth.HashToken("token-recep"): {
				ID:        toPgUUID(uuid.New()),
				UserID:    toPgUUID(receptionUID),
				TokenHash: auth.HashToken("token-recep"),
			},
		},
		memberships: map[string]authdb.Membership{
			ownerUID.String() + ":" + tenantAID.String(): {
				ID:       toPgUUID(uuid.New()),
				TenantID: toPgUUID(tenantAID),
				UserID:   toPgUUID(ownerUID),
				Role:     "owner",
			},
			receptionUID.String() + ":" + tenantAID.String(): {
				ID:       toPgUUID(uuid.New()),
				TenantID: toPgUUID(tenantAID),
				UserID:   toPgUUID(receptionUID),
				Role:     "receptionist",
			},
		},
	}

	authSvc := auth.NewService(mockAuth)
	handler := payments.NewHandler(svc, nil, authSvc)

	r := chi.NewRouter()
	handler.MountRoutes(r)

	// Purchases in Tenant A:
	pRefundable := paymentsdb.Purchase{
		ID:            toPgUUID(uuid.New()),
		TenantID:      toPgUUID(tenantAID),
		ItemType:      "workshop_registration",
		ItemID:        toPgUUID(uuid.New()),
		TitleSnapshot: "کارگاه نیازمند عودت",
		AmountRials:   5000000,
		Status:        "refund_required",
		CreatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	payMock.purchases[fromPgUUID(pRefundable.ID).String()] = pRefundable

	pCompleted := paymentsdb.Purchase{
		ID:            toPgUUID(uuid.New()),
		TenantID:      toPgUUID(tenantAID),
		ItemType:      "workshop_registration",
		ItemID:        toPgUUID(uuid.New()),
		TitleSnapshot: "کارگاه تکمیل‌شده عادی",
		AmountRials:   5000000,
		Status:        "completed",
		CreatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	payMock.purchases[fromPgUUID(pCompleted.ID).String()] = pCompleted

	pAnotherRefundable := paymentsdb.Purchase{
		ID:            toPgUUID(uuid.New()),
		TenantID:      toPgUUID(tenantAID),
		ItemType:      "workshop_registration",
		ItemID:        toPgUUID(uuid.New()),
		TitleSnapshot: "کارگاه نیازمند عودت دیگر",
		AmountRials:   2000000,
		Status:        "refund_required",
		CreatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	payMock.purchases[fromPgUUID(pAnotherRefundable.ID).String()] = pAnotherRefundable

	// 1. Owner records refund on pRefundable -> 200 OK
	body1, _ := json.Marshal(map[string]any{
		"reference_number": "REF-BANK-777",
		"reason":           "کارت به کارت به شماره کارت اعلامی",
		"amount_rials":     5000000,
	})
	req1 := httptest.NewRequest("POST", "/admin/payments/"+fromPgUUID(pRefundable.ID).String()+"/refund", bytes.NewReader(body1))
	req1.Header.Set("Authorization", "Bearer token-owner")
	req1.Header.Set("X-Tenant-ID", tenantAID.String())
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d (%s)", rec1.Code, rec1.Body.String())
	}

	var resp1 payments.RefundResponse
	if err := json.NewDecoder(rec1.Body).Decode(&resp1); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp1.RefundID == uuid.Nil || resp1.ReferenceNumber != "REF-BANK-777" {
		t.Errorf("unexpected refund response: %+v", resp1)
	}

	// Verify purchase status is updated to 'refunded'
	updatedP := payMock.purchases[fromPgUUID(pRefundable.ID).String()]
	if updatedP.Status != "refunded" {
		t.Errorf("expected purchase status 'refunded', got '%s'", updatedP.Status)
	}

	// 2. Idempotent re-submission: Same reference number on same purchase -> 200 OK, same refund ID
	req2 := httptest.NewRequest("POST", "/admin/payments/"+fromPgUUID(pRefundable.ID).String()+"/refund", bytes.NewReader(body1))
	req2.Header.Set("Authorization", "Bearer token-owner")
	req2.Header.Set("X-Tenant-ID", tenantAID.String())
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected idempotent 200 OK, got: %d (%s)", rec2.Code, rec2.Body.String())
	}
	var resp2 payments.RefundResponse
	_ = json.NewDecoder(rec2.Body).Decode(&resp2)
	if resp2.RefundID != resp1.RefundID {
		t.Errorf("expected same refund ID on idempotent call, got %s vs %s", resp2.RefundID, resp1.RefundID)
	}
	if len(payMock.refunds) != 1 {
		t.Errorf("expected exactly 1 refund in DB after idempotent retry, got %d", len(payMock.refunds))
	}

	// 3. Attempt refund on completed purchase (not refund_required) -> 400 Bad Request
	bodyCompleted, _ := json.Marshal(map[string]any{
		"reference_number": "REF-COMP-111",
		"reason":           "اشتباه",
		"amount_rials":     5000000,
	})
	reqCompleted := httptest.NewRequest("POST", "/admin/payments/"+fromPgUUID(pCompleted.ID).String()+"/refund", bytes.NewReader(bodyCompleted))
	reqCompleted.Header.Set("Authorization", "Bearer token-owner")
	reqCompleted.Header.Set("X-Tenant-ID", tenantAID.String())
	reqCompleted.Header.Set("Content-Type", "application/json")
	recCompleted := httptest.NewRecorder()
	r.ServeHTTP(recCompleted, reqCompleted)

	if recCompleted.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for refund on completed purchase, got: %d", recCompleted.Code)
	}

	// 4. Amount mismatch -> 400 Bad Request
	bodyMismatch, _ := json.Marshal(map[string]any{
		"reference_number": "REF-DIFF-222",
		"reason":           "مبلغ کمتر",
		"amount_rials":     1000000, // actual is 2000000
	})
	reqMismatch := httptest.NewRequest("POST", "/admin/payments/"+fromPgUUID(pAnotherRefundable.ID).String()+"/refund", bytes.NewReader(bodyMismatch))
	reqMismatch.Header.Set("Authorization", "Bearer token-owner")
	reqMismatch.Header.Set("X-Tenant-ID", tenantAID.String())
	reqMismatch.Header.Set("Content-Type", "application/json")
	recMismatch := httptest.NewRecorder()
	r.ServeHTTP(recMismatch, reqMismatch)

	if recMismatch.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for amount mismatch, got: %d", recMismatch.Code)
	}

	// 5. Duplicate reference number on a DIFFERENT purchase -> 409 Conflict
	bodyDuplicateRef, _ := json.Marshal(map[string]any{
		"reference_number": "REF-BANK-777", // already used for pRefundable!
		"reason":           "استفاده مجدد از کد مرجع",
		"amount_rials":     2000000,
	})
	reqDuplicate := httptest.NewRequest("POST", "/admin/payments/"+fromPgUUID(pAnotherRefundable.ID).String()+"/refund", bytes.NewReader(bodyDuplicateRef))
	reqDuplicate.Header.Set("Authorization", "Bearer token-owner")
	reqDuplicate.Header.Set("X-Tenant-ID", tenantAID.String())
	reqDuplicate.Header.Set("Content-Type", "application/json")
	recDuplicate := httptest.NewRecorder()
	r.ServeHTTP(recDuplicate, reqDuplicate)

	if recDuplicate.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for duplicate reference number on another purchase, got: %d", recDuplicate.Code)
	}

	// 6. Non-owner (receptionist) role -> 403 Forbidden
	reqRecep := httptest.NewRequest("POST", "/admin/payments/"+fromPgUUID(pAnotherRefundable.ID).String()+"/refund", bytes.NewReader(bodyMismatch))
	reqRecep.Header.Set("Authorization", "Bearer token-recep")
	reqRecep.Header.Set("X-Tenant-ID", tenantAID.String())
	reqRecep.Header.Set("Content-Type", "application/json")
	recRecep := httptest.NewRecorder()
	r.ServeHTTP(recRecep, reqRecep)

	if recRecep.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for receptionist refund attempt, got: %d", recRecep.Code)
	}

	// 7. CSRF protection: cross-site -> 403 Forbidden
	reqCSRF := httptest.NewRequest("POST", "/admin/payments/"+fromPgUUID(pAnotherRefundable.ID).String()+"/refund", bytes.NewReader(body1))
	reqCSRF.Header.Set("Authorization", "Bearer token-owner")
	reqCSRF.Header.Set("X-Tenant-ID", tenantAID.String())
	reqCSRF.Header.Set("Content-Type", "application/json")
	reqCSRF.Header.Set("Sec-Fetch-Site", "cross-site")
	recCSRF := httptest.NewRecorder()
	r.ServeHTTP(recCSRF, reqCSRF)

	if recCSRF.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for cross-site CSRF attempt, got: %d", recCSRF.Code)
	}
}
