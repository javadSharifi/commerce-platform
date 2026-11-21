package reports_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"psychology/api/internal/auth"
	"psychology/api/internal/auth/authdb"
	"psychology/api/internal/reports"
	"psychology/api/internal/reports/reportsdb"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type mockReportsStore struct {
	mu sync.RWMutex

	workshops       map[string]reportsdb.GetWorkshopMetricsRow
	registrations   map[string]reportsdb.GetRegistrationMetricsRow
	therapists      map[string]int64
	slots           map[string]reportsdb.GetSlotMetricsRow
	appointments    map[string]reportsdb.GetAppointmentMetricsRow
	courses         map[string]reportsdb.GetCourseMetricsRow
	lessons         map[string]int64
	entitlements    map[string]reportsdb.GetEntitlementMetricsRow
	financials      map[string]reportsdb.GetFinancialMetricsRow
	refunds         map[string]reportsdb.GetRefundMetricsRow
	paymentAttempts map[string]reportsdb.GetPaymentAttemptMetricsRow
	media           map[string]reportsdb.GetMediaMetricsRow
}

func newMockReportsStore() *mockReportsStore {
	return &mockReportsStore{
		workshops:       make(map[string]reportsdb.GetWorkshopMetricsRow),
		registrations:   make(map[string]reportsdb.GetRegistrationMetricsRow),
		therapists:      make(map[string]int64),
		slots:           make(map[string]reportsdb.GetSlotMetricsRow),
		appointments:    make(map[string]reportsdb.GetAppointmentMetricsRow),
		courses:         make(map[string]reportsdb.GetCourseMetricsRow),
		lessons:         make(map[string]int64),
		entitlements:    make(map[string]reportsdb.GetEntitlementMetricsRow),
		financials:      make(map[string]reportsdb.GetFinancialMetricsRow),
		refunds:         make(map[string]reportsdb.GetRefundMetricsRow),
		paymentAttempts: make(map[string]reportsdb.GetPaymentAttemptMetricsRow),
		media:           make(map[string]reportsdb.GetMediaMetricsRow),
	}
}

func (m *mockReportsStore) GetWorkshopMetrics(ctx context.Context, tenantID pgtype.UUID) (reportsdb.GetWorkshopMetricsRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tid := uuid.UUID(tenantID.Bytes).String()
	return m.workshops[tid], nil
}

func (m *mockReportsStore) GetRegistrationMetrics(ctx context.Context, tenantID pgtype.UUID) (reportsdb.GetRegistrationMetricsRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tid := uuid.UUID(tenantID.Bytes).String()
	return m.registrations[tid], nil
}

func (m *mockReportsStore) GetTherapistCount(ctx context.Context, tenantID pgtype.UUID) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tid := uuid.UUID(tenantID.Bytes).String()
	return m.therapists[tid], nil
}

func (m *mockReportsStore) GetSlotMetrics(ctx context.Context, tenantID pgtype.UUID) (reportsdb.GetSlotMetricsRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tid := uuid.UUID(tenantID.Bytes).String()
	return m.slots[tid], nil
}

func (m *mockReportsStore) GetAppointmentMetrics(ctx context.Context, tenantID pgtype.UUID) (reportsdb.GetAppointmentMetricsRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tid := uuid.UUID(tenantID.Bytes).String()
	return m.appointments[tid], nil
}

func (m *mockReportsStore) GetCourseMetrics(ctx context.Context, tenantID pgtype.UUID) (reportsdb.GetCourseMetricsRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tid := uuid.UUID(tenantID.Bytes).String()
	return m.courses[tid], nil
}

func (m *mockReportsStore) GetLessonCount(ctx context.Context, tenantID pgtype.UUID) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tid := uuid.UUID(tenantID.Bytes).String()
	return m.lessons[tid], nil
}

func (m *mockReportsStore) GetEntitlementMetrics(ctx context.Context, tenantID pgtype.UUID) (reportsdb.GetEntitlementMetricsRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tid := uuid.UUID(tenantID.Bytes).String()
	return m.entitlements[tid], nil
}

func (m *mockReportsStore) GetFinancialMetrics(ctx context.Context, tenantID pgtype.UUID) (reportsdb.GetFinancialMetricsRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tid := uuid.UUID(tenantID.Bytes).String()
	return m.financials[tid], nil
}

func (m *mockReportsStore) GetRefundMetrics(ctx context.Context, tenantID pgtype.UUID) (reportsdb.GetRefundMetricsRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tid := uuid.UUID(tenantID.Bytes).String()
	return m.refunds[tid], nil
}

func (m *mockReportsStore) GetPaymentAttemptMetrics(ctx context.Context, tenantID pgtype.UUID) (reportsdb.GetPaymentAttemptMetricsRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tid := uuid.UUID(tenantID.Bytes).String()
	return m.paymentAttempts[tid], nil
}

func (m *mockReportsStore) GetMediaMetrics(ctx context.Context, tenantID pgtype.UUID) (reportsdb.GetMediaMetricsRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tid := uuid.UUID(tenantID.Bytes).String()
	return m.media[tid], nil
}

// Mock auth store for testing HTTP handlers
type mockAuthStore struct {
	mu         sync.Mutex
	identities map[string]authdb.PanelIdentity
	sessions   map[string]authdb.PanelSession
	members    map[string]authdb.Membership
}

func newMockAuthStore() *mockAuthStore {
	return &mockAuthStore{
		identities: make(map[string]authdb.PanelIdentity),
		sessions:   make(map[string]authdb.PanelSession),
		members:    make(map[string]authdb.Membership),
	}
}

func (a *mockAuthStore) CreateUser(email, pass, name string) (authdb.PanelIdentity, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	id := uuid.New()
	hash, _ := auth.HashPassword(pass)
	var pgID pgtype.UUID
	_ = pgID.Scan(id.String())
	pi := authdb.PanelIdentity{
		ID:           pgID,
		Email:        email,
		PasswordHash: hash,
		FullName:     name,
		Status:       "active",
	}
	a.identities[email] = pi
	return pi, nil
}

func (a *mockAuthStore) AddMembership(userID pgtype.UUID, tenantID uuid.UUID, role string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	mID := uuid.New()
	var pgMID pgtype.UUID
	_ = pgMID.Scan(mID.String())
	var pgTID pgtype.UUID
	_ = pgTID.Scan(tenantID.String())
	m := authdb.Membership{
		ID:        pgMID,
		UserID:    userID,
		TenantID:  pgTID,
		Role:      role,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	uStr := uuid.UUID(userID.Bytes).String()
	a.members[uStr+":"+tenantID.String()] = m
}

func (a *mockAuthStore) CreateSession(userID pgtype.UUID) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	sID := uuid.New()
	var pgSID pgtype.UUID
	_ = pgSID.Scan(sID.String())
	rawToken := "token_" + uuid.NewString()
	tokenHash := auth.HashToken(rawToken)
	sess := authdb.PanelSession{
		ID:        pgSID,
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	}
	a.sessions[tokenHash] = sess
	return rawToken, nil
}

func (a *mockAuthStore) GetPanelIdentityByEmail(ctx context.Context, email string) (authdb.PanelIdentity, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	pi, ok := a.identities[email]
	if !ok {
		return authdb.PanelIdentity{}, auth.ErrInvalidCredentials
	}
	return pi, nil
}

func (a *mockAuthStore) GetPanelIdentityByID(ctx context.Context, id pgtype.UUID) (authdb.PanelIdentity, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, pi := range a.identities {
		if pi.ID == id {
			return pi, nil
		}
	}
	return authdb.PanelIdentity{}, auth.ErrInvalidCredentials
}

func (a *mockAuthStore) CreatePanelIdentity(ctx context.Context, arg authdb.CreatePanelIdentityParams) (authdb.PanelIdentity, error) {
	return authdb.PanelIdentity{}, nil
}

func (a *mockAuthStore) CreatePanelSession(ctx context.Context, arg authdb.CreatePanelSessionParams) (authdb.PanelSession, error) {
	return authdb.PanelSession{}, nil
}

func (a *mockAuthStore) GetPanelSessionByTokenHash(ctx context.Context, hash string) (authdb.PanelSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s, ok := a.sessions[hash]
	if !ok {
		return authdb.PanelSession{}, auth.ErrInvalidCredentials
	}
	return s, nil
}

func (a *mockAuthStore) DeletePanelSession(ctx context.Context, hash string) error {
	return nil
}

func (a *mockAuthStore) DeleteExpiredPanelSessions(ctx context.Context) error {
	return nil
}

func (a *mockAuthStore) GetMembershipByUserAndTenant(ctx context.Context, arg authdb.GetMembershipByUserAndTenantParams) (authdb.Membership, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	key := uuid.UUID(arg.UserID.Bytes).String() + ":" + uuid.UUID(arg.TenantID.Bytes).String()
	m, ok := a.members[key]
	if !ok {
		return authdb.Membership{}, pgx.ErrNoRows
	}
	return m, nil
}

func (a *mockAuthStore) ListMembershipsByUserID(ctx context.Context, userID pgtype.UUID) ([]authdb.ListMembershipsByUserIDRow, error) {
	return nil, nil
}

func (a *mockAuthStore) CreateMembership(ctx context.Context, arg authdb.CreateMembershipParams) (authdb.Membership, error) {
	return authdb.Membership{}, nil
}

func TestReportsService_CalculationsAndMetricsAccuracy(t *testing.T) {
	store := newMockReportsStore()
	svc := reports.NewService(store)
	tenantID := uuid.New()
	tIDStr := tenantID.String()

	// Seed workshop metrics
	store.workshops[tIDStr] = reportsdb.GetWorkshopMetricsRow{
		TotalWorkshops:     5,
		PublishedWorkshops: 3,
		DraftWorkshops:     1,
		ClosedWorkshops:    1,
		CancelledWorkshops: 0,
	}
	store.registrations[tIDStr] = reportsdb.GetRegistrationMetricsRow{
		TotalRegistrations:     45,
		ConfirmedRegistrations: 40,
		PendingRegistrations:   3,
		CancelledRegistrations: 2,
	}

	// Seed booking metrics
	store.therapists[tIDStr] = 3
	store.slots[tIDStr] = reportsdb.GetSlotMetricsRow{
		TotalSlots:     50,
		AvailableSlots: 20,
		BookedSlots:    25,
		HeldSlots:      5,
	}
	store.appointments[tIDStr] = reportsdb.GetAppointmentMetricsRow{
		TotalAppointments:     30,
		ConfirmedAppointments: 22,
		PendingAppointments:   4,
		CompletedAppointments: 18,
		CancelledAppointments: 4,
	}

	// Seed courses metrics
	store.courses[tIDStr] = reportsdb.GetCourseMetricsRow{
		TotalCourses:     4,
		PublishedCourses: 2,
		DraftCourses:     1,
		ArchivedCourses:  1,
	}
	store.lessons[tIDStr] = 24
	store.entitlements[tIDStr] = reportsdb.GetEntitlementMetricsRow{
		TotalEntitlements:   80,
		ActiveEntitlements:  75,
		RevokedEntitlements: 5,
		PendingEntitlements: 0,
	}

	// Seed financials
	store.financials[tIDStr] = reportsdb.GetFinancialMetricsRow{
		TotalPurchases:          125,
		CompletedPurchases:      115,
		PendingPurchases:        3,
		FailedPurchases:         2,
		RefundRequiredPurchases: 1,
		RefundedPurchases:       4,
		GrossRevenueRials:       500000000, // 50,000,000 Tomans
		WorkshopRevenueRials:    200000000,
		CourseRevenueRials:      300000000,
	}
	store.refunds[tIDStr] = reportsdb.GetRefundMetricsRow{
		TotalRefunds:       4,
		TotalRefundedRials: 15000000, // 1,500,000 Tomans
	}
	store.paymentAttempts[tIDStr] = reportsdb.GetPaymentAttemptMetricsRow{
		TotalAttempts:     130,
		InitiatedAttempts: 130,
		VerifiedAttempts:  115,
		FailedAttempts:    15,
	}

	// Seed media storage: 1.5 GiB used out of 10 GiB
	var usedBytes int64 = 1610612736 // ~1.5 GiB
	store.media[tIDStr] = reportsdb.GetMediaMetricsRow{
		TotalAssets:        12,
		TotalBytes:         usedBytes,
		PublicBytes:        104857600,  // 100 MiB
		CoursePrivateBytes: 1505755136, // ~1.4 GiB
		VideoCount:         8,
		AudioCount:         3,
		ImageCount:         1,
	}

	rep, err := svc.GetTenantMetrics(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify calculations
	if rep.Workshops.TotalWorkshops != 5 || rep.Workshops.PublishedWorkshops != 3 {
		t.Errorf("workshops count mismatch")
	}
	if rep.Workshops.TotalRegistrations != 45 || rep.Workshops.ConfirmedRegistrations != 40 {
		t.Errorf("registration count mismatch")
	}

	if rep.Booking.ActiveTherapists != 3 || rep.Booking.AvailableSlots != 20 {
		t.Errorf("booking metrics mismatch")
	}

	if rep.Courses.TotalLessons != 24 || rep.Courses.ActiveEntitlements != 75 {
		t.Errorf("course metrics mismatch")
	}

	// Financials & Net Revenue calculation (Gross - Refunds)
	expectedNet := int64(500000000 - 15000000)
	if rep.Financial.NetRevenueRials != expectedNet {
		t.Errorf("expected net revenue %d, got %d", expectedNet, rep.Financial.NetRevenueRials)
	}

	// Media quota and percentage
	if rep.Media.StorageQuotaBytes != 10*1024*1024*1024 {
		t.Errorf("expected 10 GiB quota, got %d", rep.Media.StorageQuotaBytes)
	}
	expectedPermille := (usedBytes * 1000) / (10 * 1024 * 1024 * 1024)
	if rep.Media.UsagePercentagePermille != expectedPermille {
		t.Errorf("expected permille %d, got %d", expectedPermille, rep.Media.UsagePercentagePermille)
	}
	expectedPercent := (usedBytes * 100) / (10 * 1024 * 1024 * 1024)
	if rep.Media.UsagePercentage != expectedPercent {
		t.Errorf("expected percent %d, got %d", expectedPercent, rep.Media.UsagePercentage)
	}
}

func TestReportsService_ZeroPIIGuardian(t *testing.T) {
	store := newMockReportsStore()
	svc := reports.NewService(store)
	tenantID := uuid.New()
	tIDStr := tenantID.String()

	store.workshops[tIDStr] = reportsdb.GetWorkshopMetricsRow{TotalWorkshops: 1}
	store.financials[tIDStr] = reportsdb.GetFinancialMetricsRow{GrossRevenueRials: 10000000}

	rep, err := svc.GetTenantMetrics(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Serialize report to JSON as sent across HTTP API
	data, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}
	jsonStr := strings.ToLower(string(data))

	// Guardian check: Ensure zero PII / medical / personal keys exist in the payload
	disallowedKeys := []string{
		`"guest_name"`,
		`"full_name"`,
		`"phone"`,
		`"mobile"`,
		`"email"`,
		`"notes"`,
		`"lookup_hash"`,
		`"patient"`,
		`"client_name"`,
		`"medical"`,
		`"diagnosis"`,
		`"treatment"`,
		`"session_notes"`,
	}

	for _, key := range disallowedKeys {
		if strings.Contains(jsonStr, key) {
			t.Fatalf("SECURITY VIOLATION: Zero PII rule violated! Found key %s in reports metrics output", key)
		}
	}
}

func TestReportsService_CrossTenantIsolation(t *testing.T) {
	store := newMockReportsStore()
	svc := reports.NewService(store)

	tenantA := uuid.New()
	tenantB := uuid.New()

	// Seed Tenant A (Clinic Omid)
	store.workshops[tenantA.String()] = reportsdb.GetWorkshopMetricsRow{
		TotalWorkshops:     2,
		PublishedWorkshops: 2,
	}
	store.financials[tenantA.String()] = reportsdb.GetFinancialMetricsRow{
		TotalPurchases:    10,
		GrossRevenueRials: 100000000,
	}
	store.media[tenantA.String()] = reportsdb.GetMediaMetricsRow{
		TotalAssets: 5,
		TotalBytes:  500 * 1024 * 1024,
	}

	// Seed Tenant B (Clinic Mahan)
	store.workshops[tenantB.String()] = reportsdb.GetWorkshopMetricsRow{
		TotalWorkshops:     7,
		PublishedWorkshops: 5,
	}
	store.financials[tenantB.String()] = reportsdb.GetFinancialMetricsRow{
		TotalPurchases:    35,
		GrossRevenueRials: 750000000,
	}
	store.media[tenantB.String()] = reportsdb.GetMediaMetricsRow{
		TotalAssets: 18,
		TotalBytes:  2 * 1024 * 1024 * 1024,
	}

	// Query metrics for Tenant A
	repA, err := svc.GetTenantMetrics(context.Background(), tenantA)
	if err != nil {
		t.Fatalf("Tenant A query failed: %v", err)
	}
	if repA.Workshops.TotalWorkshops != 2 || repA.Financial.TotalPurchases != 10 || repA.Financial.GrossRevenueRials != 100000000 {
		t.Errorf("Tenant A metrics corrupted or leaking: %+v", repA)
	}

	// Query metrics for Tenant B
	repB, err := svc.GetTenantMetrics(context.Background(), tenantB)
	if err != nil {
		t.Fatalf("Tenant B query failed: %v", err)
	}
	if repB.Workshops.TotalWorkshops != 7 || repB.Financial.TotalPurchases != 35 || repB.Financial.GrossRevenueRials != 750000000 {
		t.Errorf("Tenant B metrics corrupted or leaking: %+v", repB)
	}

	// Strict cross-check: Tenant A must not have any numbers from Tenant B
	if repA.Financial.GrossRevenueRials == repB.Financial.GrossRevenueRials {
		t.Errorf("Tenant isolation violated: identical financial data detected")
	}
}

func TestReportsHTTP_RBACAndTenantMembership(t *testing.T) {
	store := newMockReportsStore()
	svc := reports.NewService(store)

	tenantA := uuid.New()
	tenantB := uuid.New()

	authStore := newMockAuthStore()
	authSvc := auth.NewService(authStore)

	// Owner of Tenant A
	ownerA, _ := authStore.CreateUser("owner@omid.ir", "pass123", "مالک امید")
	authStore.AddMembership(ownerA.ID, tenantA, "owner")
	tokenOwnerA, _ := authStore.CreateSession(ownerA.ID)

	// Therapist of Tenant A (cannot view financial reports)
	thA, _ := authStore.CreateUser("therapist@omid.ir", "pass123", "درمانگر امید")
	authStore.AddMembership(thA.ID, tenantA, "therapist")
	tokenThA, _ := authStore.CreateSession(thA.ID)

	// Owner of Tenant B
	ownerB, _ := authStore.CreateUser("owner@mahan.ir", "pass123", "مالک ماهان")
	authStore.AddMembership(ownerB.ID, tenantB, "owner")
	tokenOwnerB, _ := authStore.CreateSession(ownerB.ID)

	handler := reports.NewHandler(svc, nil, authSvc)
	r := chi.NewRouter()
	handler.MountRoutes(r)

	// Seed Tenant A metrics
	store.workshops[tenantA.String()] = reportsdb.GetWorkshopMetricsRow{TotalWorkshops: 3}

	// 1. Unauthenticated request -> 401 Unauthorized
	unauthReq := httptest.NewRequest(http.MethodGet, "/admin/reports/metrics", nil)
	unauthReq.Header.Set("X-Tenant-ID", tenantA.String())
	unauthRec := httptest.NewRecorder()
	r.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated request, got %d", unauthRec.Code)
	}

	// 2. Owner A requesting Tenant A -> 200 OK
	ownerAReq := httptest.NewRequest(http.MethodGet, "/admin/reports/metrics", nil)
	ownerAReq.Header.Set("Authorization", "Bearer "+tokenOwnerA)
	ownerAReq.Header.Set("X-Tenant-ID", tenantA.String())
	ownerARec := httptest.NewRecorder()
	r.ServeHTTP(ownerARec, ownerAReq)
	if ownerARec.Code != http.StatusOK {
		t.Fatalf("expected 200 for owner request, got %d: %s", ownerARec.Code, ownerARec.Body.String())
	}
	var repOut reports.TenantMetricsReport
	_ = json.Unmarshal(ownerARec.Body.Bytes(), &repOut)
	if repOut.Workshops.TotalWorkshops != 3 {
		t.Errorf("expected 3 workshops, got %d", repOut.Workshops.TotalWorkshops)
	}

	// Check cache control header
	if !strings.Contains(ownerARec.Header().Get("Cache-Control"), "no-store") {
		t.Errorf("expected no-store cache control on reports, got %s", ownerARec.Header().Get("Cache-Control"))
	}

	// 3. Therapist A requesting Tenant A metrics -> 403 Forbidden (Only owner permitted)
	thAReq := httptest.NewRequest(http.MethodGet, "/admin/reports/metrics", nil)
	thAReq.Header.Set("Authorization", "Bearer "+tokenThA)
	thAReq.Header.Set("X-Tenant-ID", tenantA.String())
	thARec := httptest.NewRecorder()
	r.ServeHTTP(thARec, thAReq)
	if thARec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for non-owner role, got %d", thARec.Code)
	}

	// 4. Owner B requesting Tenant A data -> 403 Forbidden (Tenant boundary enforcement)
	crossReq := httptest.NewRequest(http.MethodGet, "/admin/reports/metrics", nil)
	crossReq.Header.Set("Authorization", "Bearer "+tokenOwnerB)
	crossReq.Header.Set("X-Tenant-ID", tenantA.String()) // Owner B trying to access Tenant A
	crossRec := httptest.NewRecorder()
	r.ServeHTTP(crossRec, crossReq)
	if crossRec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for cross-tenant access attempt, got %d", crossRec.Code)
	}
}
