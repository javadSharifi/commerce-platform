package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"psychology/api/internal/auth"
	"psychology/api/internal/auth/authdb"
	"psychology/api/internal/domain"
	"psychology/api/internal/domain/domaindb"
	"psychology/api/internal/platform/db"
	"psychology/api/internal/settings"
	"psychology/api/internal/settings/settingsdb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func toPgUUID(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

func fromPgUUID(p pgtype.UUID) uuid.UUID {
	var u uuid.UUID
	copy(u[:], p.Bytes[:])
	return u
}

type serverMockStore struct {
	clinics     map[string]string
	domains     map[string]domaindb.Domain
	settings    map[string]settingsdb.SiteSetting
	identities  map[string]authdb.PanelIdentity
	sessions    map[string]authdb.PanelSession
	memberships map[string]authdb.Membership
}

func newServerMockStore() *serverMockStore {
	return &serverMockStore{
		clinics:     make(map[string]string),
		domains:     make(map[string]domaindb.Domain),
		settings:    make(map[string]settingsdb.SiteSetting),
		identities:  make(map[string]authdb.PanelIdentity),
		sessions:    make(map[string]authdb.PanelSession),
		memberships: make(map[string]authdb.Membership),
	}
}

// Domain Querier
func (s *serverMockStore) GetDomainByHost(ctx context.Context, host string) (domaindb.Domain, error) {
	d, ok := s.domains[host]
	if !ok {
		return domaindb.Domain{}, pgx.ErrNoRows
	}
	return d, nil
}

// Settings Querier
func (s *serverMockStore) GetSiteSettings(ctx context.Context, tenantID pgtype.UUID) (settingsdb.SiteSetting, error) {
	st, ok := s.settings[fromPgUUID(tenantID).String()]
	if !ok {
		return settingsdb.SiteSetting{}, pgx.ErrNoRows
	}
	return st, nil
}

func (s *serverMockStore) UpsertSiteSettings(ctx context.Context, arg settingsdb.UpsertSiteSettingsParams) (settingsdb.SiteSetting, error) {
	st := settingsdb.SiteSetting{
		TenantID:        arg.TenantID,
		Title:           arg.Title,
		Description:     arg.Description,
		Phone:           arg.Phone,
		Address:         arg.Address,
		PrimaryColor:    arg.PrimaryColor,
		LogoMediaID:     arg.LogoMediaID,
		TemplateID:      arg.TemplateID,
		TemplateVersion: arg.TemplateVersion,
		ThemeRadius:     arg.ThemeRadius,
		ThemeScale:      arg.ThemeScale,
	}
	s.settings[fromPgUUID(arg.TenantID).String()] = st
	return st, nil
}

func (s *serverMockStore) GetPublicSiteProfile(ctx context.Context, id pgtype.UUID) (settingsdb.GetPublicSiteProfileRow, error) {
	name, ok := s.clinics[fromPgUUID(id).String()]
	if !ok {
		return settingsdb.GetPublicSiteProfileRow{}, pgx.ErrNoRows
	}
	st, ok := s.settings[fromPgUUID(id).String()]
	if !ok {
		return settingsdb.GetPublicSiteProfileRow{
			ClinicName:      name,
			Title:           pgtype.Text{String: name, Valid: true},
			PrimaryColor:    pgtype.Text{String: "#0284c7", Valid: true},
			TemplateID:      "minimal",
			TemplateVersion: "1.0.0",
			ThemeRadius:     "md",
			ThemeScale:      "md",
		}, nil
	}
	return settingsdb.GetPublicSiteProfileRow{
		ClinicName:      name,
		Title:           pgtype.Text{String: st.Title, Valid: true},
		Description:     pgtype.Text{String: st.Description, Valid: true},
		Phone:           pgtype.Text{String: st.Phone, Valid: true},
		Address:         pgtype.Text{String: st.Address, Valid: true},
		PrimaryColor:    pgtype.Text{String: st.PrimaryColor, Valid: true},
		LogoMediaID:     st.LogoMediaID,
		TemplateID:      st.TemplateID,
		TemplateVersion: st.TemplateVersion,
		ThemeRadius:     st.ThemeRadius,
		ThemeScale:      st.ThemeScale,
	}, nil
}

func (s *serverMockStore) GetPrimaryVerifiedDomain(ctx context.Context, tenantID pgtype.UUID) (string, error) {
	tid := fromPgUUID(tenantID).String()
	for host, d := range s.domains {
		if fromPgUUID(d.TenantID).String() == tid && d.IsVerified {
			return host, nil
		}
	}
	return "", pgx.ErrNoRows
}

// Auth Querier
func (s *serverMockStore) GetPanelIdentityByEmail(ctx context.Context, email string) (authdb.PanelIdentity, error) {
	for _, u := range s.identities {
		if u.Email == email {
			return u, nil
		}
	}
	return authdb.PanelIdentity{}, pgx.ErrNoRows
}

func (s *serverMockStore) GetPanelIdentityByID(ctx context.Context, id pgtype.UUID) (authdb.PanelIdentity, error) {
	u, ok := s.identities[fromPgUUID(id).String()]
	if !ok {
		return authdb.PanelIdentity{}, pgx.ErrNoRows
	}
	return u, nil
}

func (s *serverMockStore) CreatePanelIdentity(ctx context.Context, arg authdb.CreatePanelIdentityParams) (authdb.PanelIdentity, error) {
	return authdb.PanelIdentity{}, nil
}

func (s *serverMockStore) CreatePanelSession(ctx context.Context, arg authdb.CreatePanelSessionParams) (authdb.PanelSession, error) {
	return authdb.PanelSession{}, nil
}

func (s *serverMockStore) GetPanelSessionByTokenHash(ctx context.Context, tokenHash string) (authdb.PanelSession, error) {
	sess, ok := s.sessions[tokenHash]
	if !ok {
		return authdb.PanelSession{}, pgx.ErrNoRows
	}
	return sess, nil
}

func (s *serverMockStore) DeletePanelSession(ctx context.Context, tokenHash string) error {
	return nil
}

func (s *serverMockStore) ListMembershipsByUserID(ctx context.Context, userID pgtype.UUID) ([]authdb.ListMembershipsByUserIDRow, error) {
	return nil, nil
}

func (s *serverMockStore) GetMembershipByUserAndTenant(ctx context.Context, arg authdb.GetMembershipByUserAndTenantParams) (authdb.Membership, error) {
	key := fromPgUUID(arg.UserID).String() + ":" + fromPgUUID(arg.TenantID).String()
	mem, ok := s.memberships[key]
	if !ok {
		return authdb.Membership{}, pgx.ErrNoRows
	}
	return mem, nil
}

func (s *serverMockStore) CreateMembership(ctx context.Context, arg authdb.CreateMembershipParams) (authdb.Membership, error) {
	return authdb.Membership{}, nil
}

func TestHealthCheck(t *testing.T) {
	router := setupRouter(nil, nil, nil, nil, nil)

	req, err := http.NewRequest(http.MethodGet, "/healthz", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%s'", response["status"])
	}
}

func TestServer_SettingsEndToEnd_TwoHostsAndAuth(t *testing.T) {
	store := newServerMockStore()

	// Fixture: Clinic A
	tenantAID := uuid.New()
	store.clinics[tenantAID.String()] = "کلینیک روان‌شناسی امید"
	store.domains["omid-psych.ir"] = domaindb.Domain{
		Host:       "omid-psych.ir",
		TenantID:   toPgUUID(tenantAID),
		IsVerified: true,
	}
	store.settings[tenantAID.String()] = settingsdb.SiteSetting{
		TenantID:     toPgUUID(tenantAID),
		Title:        "دکتر علوی - مشاوره و درمان",
		Description:  "روان‌درمانی تخصصی",
		PrimaryColor: "#0284c7",
	}

	// Fixture: Clinic B
	tenantBID := uuid.New()
	store.clinics[tenantBID.String()] = "مرکز مشاوره ماهان"
	store.domains["mahan-clinic.ir"] = domaindb.Domain{
		Host:       "mahan-clinic.ir",
		TenantID:   toPgUUID(tenantBID),
		IsVerified: true,
	}
	store.settings[tenantBID.String()] = settingsdb.SiteSetting{
		TenantID:     toPgUUID(tenantBID),
		Title:        "مرکز مشاوره ماهان",
		Description:  "کودک و نوجوان",
		PrimaryColor: "#10b981",
	}

	// Fixture: Unverified Domain
	tenantCID := uuid.New()
	store.domains["unverified.ir"] = domaindb.Domain{
		Host:       "unverified.ir",
		TenantID:   toPgUUID(tenantCID),
		IsVerified: false,
	}

	// Fixture: User A (Owner of Tenant A)
	userAID := uuid.New()
	store.identities[userAID.String()] = authdb.PanelIdentity{
		ID:       toPgUUID(userAID),
		Email:    "owner@omid-psych.ir",
		FullName: "دکتر علوی",
		Status:   "active",
	}
	validTokenA := "token-owner-a"
	store.sessions[auth.HashToken(validTokenA)] = authdb.PanelSession{
		ID:        toPgUUID(uuid.New()),
		UserID:    toPgUUID(userAID),
		TokenHash: auth.HashToken(validTokenA),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	}
	store.memberships[userAID.String()+":"+tenantAID.String()] = authdb.Membership{
		ID:       toPgUUID(uuid.New()),
		TenantID: toPgUUID(tenantAID),
		UserID:   toPgUUID(userAID),
		Role:     "owner",
	}

	// Assemble real router via setupRouter
	domainResolver := domain.NewResolver(store)
	authService := auth.NewService(store)
	authHandler := auth.NewHandler(authService)
	settingsService := settings.NewService(store)
	settingsHandler := settings.NewHandler(settingsService, domainResolver, authService)

	router := setupRouter(authHandler, nil, nil, nil, settingsHandler)

	// 1. Host A public profile -> 200 OK
	reqA := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqA.Host = "omid-psych.ir"
	rrA := httptest.NewRecorder()
	router.ServeHTTP(rrA, reqA)
	if rrA.Code != http.StatusOK {
		t.Fatalf("expected 200 for Host A, got %d: %s", rrA.Code, rrA.Body.String())
	}
	var resA settings.PublicSiteProfile
	if err := json.Unmarshal(rrA.Body.Bytes(), &resA); err != nil {
		t.Fatalf("failed to decode response for Host A: %v", err)
	}
	if resA.Name != "کلینیک روان‌شناسی امید" || resA.PrimaryColor != "#0284c7" {
		t.Fatalf("unexpected profile for Host A: %+v", resA)
	}

	// 2. Host B public profile -> 200 OK
	reqB := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqB.Host = "mahan-clinic.ir"
	rrB := httptest.NewRecorder()
	router.ServeHTTP(rrB, reqB)
	if rrB.Code != http.StatusOK {
		t.Fatalf("expected 200 for Host B, got %d: %s", rrB.Code, rrB.Body.String())
	}
	var resB settings.PublicSiteProfile
	if err := json.Unmarshal(rrB.Body.Bytes(), &resB); err != nil {
		t.Fatalf("failed to decode response for Host B: %v", err)
	}
	if resB.Name != "مرکز مشاوره ماهان" || resB.PrimaryColor != "#10b981" {
		t.Fatalf("unexpected profile for Host B: %+v", resB)
	}

	// 3. Unknown Host -> 404 DOMAIN_NOT_FOUND
	reqUnknown := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqUnknown.Host = "unknown.ir"
	rrUnknown := httptest.NewRecorder()
	router.ServeHTTP(rrUnknown, reqUnknown)
	if rrUnknown.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown host, got %d", rrUnknown.Code)
	}

	// 4. Unverified Host -> 404 DOMAIN_NOT_VERIFIED
	reqUnverified := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqUnverified.Host = "unverified.ir"
	rrUnverified := httptest.NewRecorder()
	router.ServeHTTP(rrUnverified, reqUnverified)
	if rrUnverified.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unverified host, got %d", rrUnverified.Code)
	}

	// 5. Unauthenticated Admin Request -> 401 UNAUTHORIZED
	reqUnauth := httptest.NewRequest(http.MethodGet, "/v1/admin/site", nil)
	rrUnauth := httptest.NewRecorder()
	router.ServeHTTP(rrUnauth, reqUnauth)
	if rrUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request, got %d", rrUnauth.Code)
	}

	// 6. Cross-Tenant Mutation Attempt: Owner A tries to mutate Tenant B -> 403 FORBIDDEN
	tamperBody, _ := json.Marshal(map[string]string{
		"title":         "کلینیک دستکاری‌شده ماهان",
		"primary_color": "#ff0000",
	})
	reqTamper := httptest.NewRequest(http.MethodPut, "/v1/admin/site", bytes.NewBuffer(tamperBody))
	reqTamper.Header.Set("Authorization", "Bearer "+validTokenA)
	reqTamper.Header.Set("X-Tenant-ID", tenantBID.String())
	reqTamper.Header.Set("Content-Type", "application/json")
	rrTamper := httptest.NewRecorder()
	router.ServeHTTP(rrTamper, reqTamper)
	if rrTamper.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for cross-tenant mutation attempt, got %d", rrTamper.Code)
	}

	// Verify Tenant B settings were NOT modified
	stB := store.settings[tenantBID.String()]
	if stB.Title != "مرکز مشاوره ماهان" {
		t.Fatalf("Tenant B settings were corrupted! Got: %s", stB.Title)
	}

	// 7. Authorized Admin Read for Tenant A by Owner A -> 200 OK
	reqAdminGet := httptest.NewRequest(http.MethodGet, "/v1/admin/site", nil)
	reqAdminGet.Header.Set("Authorization", "Bearer "+validTokenA)
	reqAdminGet.Header.Set("X-Tenant-ID", tenantAID.String())
	rrAdminGet := httptest.NewRecorder()
	router.ServeHTTP(rrAdminGet, reqAdminGet)
	if rrAdminGet.Code != http.StatusOK {
		t.Fatalf("expected 200 for authorized admin get, got %d: %s", rrAdminGet.Code, rrAdminGet.Body.String())
	}

	// 8. Authorized Admin Update for Tenant A by Owner A -> 200 OK
	validBody, _ := json.Marshal(map[string]string{
		"title":         "کلینیک تخصصی دکتر علوی (به‌روزشده)",
		"description":   "مشاوره و روان‌درمانی فردی و خانواده",
		"phone":         "02188889999",
		"address":       "تهران، خیابان ولیعصر",
		"primary_color": "#0ea5e9",
	})
	reqAdminPut := httptest.NewRequest(http.MethodPut, "/v1/admin/site", bytes.NewBuffer(validBody))
	reqAdminPut.Header.Set("Authorization", "Bearer "+validTokenA)
	reqAdminPut.Header.Set("X-Tenant-ID", tenantAID.String())
	reqAdminPut.Header.Set("Content-Type", "application/json")
	rrAdminPut := httptest.NewRecorder()
	router.ServeHTTP(rrAdminPut, reqAdminPut)
	if rrAdminPut.Code != http.StatusOK {
		t.Fatalf("expected 200 for authorized admin put, got %d: %s", rrAdminPut.Code, rrAdminPut.Body.String())
	}

	// 9. Host A public profile now reflects the updated settings -> 200 OK
	reqAUpdated := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqAUpdated.Host = "omid-psych.ir"
	rrAUpdated := httptest.NewRecorder()
	router.ServeHTTP(rrAUpdated, reqAUpdated)
	if rrAUpdated.Code != http.StatusOK {
		t.Fatalf("expected 200 for Host A updated, got %d", rrAUpdated.Code)
	}
	var resAUpdated settings.PublicSiteProfile
	_ = json.Unmarshal(rrAUpdated.Body.Bytes(), &resAUpdated)
	if resAUpdated.Title != "کلینیک تخصصی دکتر علوی (به‌روزشده)" || resAUpdated.PrimaryColor != "#0ea5e9" {
		t.Fatalf("Host A profile was not updated: %+v", resAUpdated)
	}

	// 10. Host B public profile remains completely unchanged -> 200 OK
	reqBCheck := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqBCheck.Host = "mahan-clinic.ir"
	rrBCheck := httptest.NewRecorder()
	router.ServeHTTP(rrBCheck, reqBCheck)
	var resBCheck settings.PublicSiteProfile
	_ = json.Unmarshal(rrBCheck.Body.Bytes(), &resBCheck)
	if resBCheck.Title != "مرکز مشاوره ماهان" || resBCheck.PrimaryColor != "#10b981" {
		t.Fatalf("Host B profile was polluted by Host A update! Got: %+v", resBCheck)
	}
}

func TestServer_LiveSettingsWithRealPool(t *testing.T) {
	dbURL := os.Getenv("TEST_APP_DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		t.Skip("skipping live server database test: TEST_APP_DATABASE_URL or DATABASE_URL not set")
		return
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live server test (failed to connect: %v)", err)
		return
	}
	defer pool.Close()

	store := db.NewStore(pool)
	domainResolver := domain.NewResolver(domaindb.New(pool))
	authService := auth.NewService(authdb.New(pool))
	authHandler := auth.NewHandler(authService)

	settingsService := settings.NewServiceWithStore(store)
	settingsHandler := settings.NewHandler(settingsService, domainResolver, authService)

	router := setupRouter(authHandler, nil, nil, nil, settingsHandler)

	// Verify health check works with full setup
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for /v1/health with live pool, got %d", rr.Code)
	}

	// Verify unknown host handling with live pool
	reqUnknown := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqUnknown.Host = "unknown-" + uuid.NewString() + ".ir"
	rrUnknown := httptest.NewRecorder()
	router.ServeHTTP(rrUnknown, reqUnknown)
	if rrUnknown.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown host with live pool, got %d", rrUnknown.Code)
	}
}
