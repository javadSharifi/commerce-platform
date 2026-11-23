package main

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
	"psychology/api/internal/domain"
	"psychology/api/internal/domain/domaindb"
	"psychology/api/internal/platform/db"
	"psychology/api/internal/platform/tenantctx"
	"psychology/api/internal/settings"
	"psychology/api/internal/settings/settingsdb"
	"psychology/api/internal/testsupport/dbfixture"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
func (s *serverMockStore) GetSiteSettings(ctx context.Context, tenantID pgtype.UUID) (settingsdb.GetSiteSettingsRow, error) {
	st, ok := s.settings[fromPgUUID(tenantID).String()]
	if !ok {
		return settingsdb.GetSiteSettingsRow{}, pgx.ErrNoRows
	}
	pal := st.PaletteID
	if pal == "" {
		pal = "calm-sage"
	}
	return settingsdb.GetSiteSettingsRow{
		TenantID:        st.TenantID,
		Title:           st.Title,
		Description:     st.Description,
		Phone:           st.Phone,
		Address:         st.Address,
		PrimaryColor:    st.PrimaryColor,
		LogoMediaID:     st.LogoMediaID,
		TemplateID:      st.TemplateID,
		TemplateVersion: st.TemplateVersion,
		ThemeRadius:     st.ThemeRadius,
		ThemeScale:      st.ThemeScale,
		PaletteID:       pal,
		CreatedAt:       st.CreatedAt,
		UpdatedAt:       st.UpdatedAt,
	}, nil
}

func (s *serverMockStore) UpsertSiteSettings(ctx context.Context, arg settingsdb.UpsertSiteSettingsParams) (settingsdb.UpsertSiteSettingsRow, error) {
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
		PaletteID:       arg.PaletteID,
	}
	s.settings[fromPgUUID(arg.TenantID).String()] = st
	return settingsdb.UpsertSiteSettingsRow{
		TenantID:        st.TenantID,
		Title:           st.Title,
		Description:     st.Description,
		Phone:           st.Phone,
		Address:         st.Address,
		PrimaryColor:    st.PrimaryColor,
		LogoMediaID:     st.LogoMediaID,
		TemplateID:      st.TemplateID,
		TemplateVersion: st.TemplateVersion,
		ThemeRadius:     st.ThemeRadius,
		ThemeScale:      st.ThemeScale,
		PaletteID:       st.PaletteID,
		CreatedAt:       st.CreatedAt,
		UpdatedAt:       st.UpdatedAt,
	}, nil
}

func (s *serverMockStore) GetSiteAppearance(ctx context.Context, tenantID pgtype.UUID) (settingsdb.GetSiteAppearanceRow, error) {
	st, ok := s.settings[fromPgUUID(tenantID).String()]
	if !ok {
		return settingsdb.GetSiteAppearanceRow{}, pgx.ErrNoRows
	}
	pal := st.PaletteID
	if pal == "" {
		pal = "calm-sage"
	}
	return settingsdb.GetSiteAppearanceRow{
		TenantID:        st.TenantID,
		TemplateID:      st.TemplateID,
		TemplateVersion: st.TemplateVersion,
		PaletteID:       pal,
		ThemeRadius:     st.ThemeRadius,
		ThemeScale:      st.ThemeScale,
		CreatedAt:       st.CreatedAt,
		UpdatedAt:       st.UpdatedAt,
	}, nil
}

func (s *serverMockStore) UpsertSiteAppearance(ctx context.Context, arg settingsdb.UpsertSiteAppearanceParams) (settingsdb.UpsertSiteAppearanceRow, error) {
	key := fromPgUUID(arg.TenantID).String()
	st := s.settings[key]
	st.TenantID = arg.TenantID
	st.TemplateID = arg.TemplateID
	st.TemplateVersion = arg.TemplateVersion
	st.PaletteID = arg.PaletteID
	st.ThemeRadius = arg.ThemeRadius
	st.ThemeScale = arg.ThemeScale
	s.settings[key] = st
	return settingsdb.UpsertSiteAppearanceRow{
		TenantID:        st.TenantID,
		TemplateID:      st.TemplateID,
		TemplateVersion: st.TemplateVersion,
		PaletteID:       st.PaletteID,
		ThemeRadius:     st.ThemeRadius,
		ThemeScale:      st.ThemeScale,
		CreatedAt:       st.CreatedAt,
		UpdatedAt:       st.UpdatedAt,
	}, nil
}

func (s *serverMockStore) UpsertSiteProfile(ctx context.Context, arg settingsdb.UpsertSiteProfileParams) (settingsdb.UpsertSiteProfileRow, error) {
	key := fromPgUUID(arg.TenantID).String()
	st := s.settings[key]
	st.TenantID = arg.TenantID
	st.Title = arg.Title
	st.Description = arg.Description
	st.Phone = arg.Phone
	st.Address = arg.Address
	st.PrimaryColor = arg.PrimaryColor
	st.LogoMediaID = arg.LogoMediaID
	if st.TemplateID == "" {
		st.TemplateID = "minimal"
	}
	if st.TemplateVersion == "" {
		st.TemplateVersion = "1.0.0"
	}
	if st.ThemeRadius == "" {
		st.ThemeRadius = "md"
	}
	if st.ThemeScale == "" {
		st.ThemeScale = "md"
	}
	if st.PaletteID == "" {
		st.PaletteID = "calm-sage"
	}
	s.settings[key] = st
	return settingsdb.UpsertSiteProfileRow{
		TenantID:        st.TenantID,
		Title:           st.Title,
		Description:     st.Description,
		Phone:           st.Phone,
		Address:         st.Address,
		PrimaryColor:    st.PrimaryColor,
		LogoMediaID:     st.LogoMediaID,
		TemplateID:      st.TemplateID,
		TemplateVersion: st.TemplateVersion,
		ThemeRadius:     st.ThemeRadius,
		ThemeScale:      st.ThemeScale,
		PaletteID:       st.PaletteID,
		CreatedAt:       st.CreatedAt,
		UpdatedAt:       st.UpdatedAt,
	}, nil
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
			PaletteID:       "calm-sage",
		}, nil
	}
	pal := st.PaletteID
	if pal == "" {
		pal = "calm-sage"
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
		PaletteID:       pal,
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
	router := setupRouter(nil, nil, nil, nil, nil, nil, nil, nil, nil)

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

	router := setupRouter(authHandler, nil, nil, nil, settingsHandler, nil, nil, nil, nil)

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
		"title": "کلینیک دستکاری‌شده ماهان",
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
		"title":       "کلینیک تخصصی دکتر علوی (به‌روزشده)",
		"description": "مشاوره و روان‌درمانی فردی و خانواده",
		"phone":       "02188889999",
		"address":     "تهران، خیابان ولیعصر",
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
	if resAUpdated.Title != "کلینیک تخصصی دکتر علوی (به‌روزشده)" || resAUpdated.PrimaryColor != "#0284c7" {
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
	ctx := context.Background()
	pool := dbfixture.Connect(t, dbfixture.AppDSN, "app_user")

	store := db.NewStore(pool)
	settingsService := settings.NewServiceWithStore(store)
	dbfixture.Connect(t, dbfixture.WorkerDSN, "worker_user")
	userA := uuid.New()
	address, serve := startFixtureServer(t, userA.String())

	// 1. Verify health check works with full setup
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	rr := httptest.NewRecorder()
	serve(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for /v1/health with live pool, got %d", rr.Code)
	}

	// 2. Verify unknown host handling with live pool returns 404
	reqUnknown := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqUnknown.Host = "unknown-" + uuid.NewString() + ".ir"
	rrUnknown := httptest.NewRecorder()
	serve(rrUnknown, reqUnknown)
	if rrUnknown.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown host with live pool, got %d", rrUnknown.Code)
	}

	// 3. Seed two distinct tenants and panel identities
	tenantA := uuid.New()
	tenantB := uuid.New()
	slugA := "live-a-" + tenantA.String()[:8]
	slugB := "live-b-" + tenantB.String()[:8]
	hostA := slugA + ".test"
	hostB := slugB + ".test"

	_, err := pool.Exec(ctx, `
		INSERT INTO tenants (id, name, slug, status)
		VALUES ($1, $2, $3, 'active'), ($4, $5, $6, 'active')
	`, tenantA, "کلینیک دکتر شمس", slugA, tenantB, "کلینیک دکتر فردوس", slugB)
	if err != nil {
		t.Fatalf("failed to insert live tenants: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM tenants WHERE id IN ($1, $2)", tenantA, tenantB)
	}()

	userB := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO panel_identities (id, email, password_hash, full_name, status)
		VALUES ($1, $2, 'hash', 'دکتر شمس', 'active'), ($3, $4, 'hash', 'دکتر فردوس', 'active')
	`, userA, slugA+"@clinic.ir", userB, slugB+"@clinic.ir")
	if err != nil {
		t.Fatalf("failed to insert live panel identities: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM panel_identities WHERE id IN ($1, $2)", userA, userB)
	}()

	ctxA := tenantctx.With(ctx, tenantA)
	ctxB := tenantctx.With(ctx, tenantB)

	// Seed domain, membership, and initial settings for Tenant A
	err = store.WithTenantTx(ctxA, func(tx pgx.Tx, tid uuid.UUID) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO domains (id, tenant_id, host, is_primary, is_verified, verified_at)
			VALUES ($1, $2, $3, true, true, now())
		`, uuid.New(), tid, hostA); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO memberships (id, tenant_id, user_id, role)
			VALUES ($1, $2, $3, 'owner')
		`, uuid.New(), tid, userA); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to seed domain/membership for Tenant A: %v", err)
	}
	if _, err := settingsService.UpdateSettings(ctxA, tenantA, settings.UpdateSettingsParams{
		Title:        "کلینیک دکتر شمس",
		PrimaryColor: "#0ea5e9",
		TemplateID:   "minimal",
	}); err != nil {
		t.Fatalf("failed to set initial settings for Tenant A: %v", err)
	}

	// Seed domain, membership, and initial settings for Tenant B
	err = store.WithTenantTx(ctxB, func(tx pgx.Tx, tid uuid.UUID) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO domains (id, tenant_id, host, is_primary, is_verified, verified_at)
			VALUES ($1, $2, $3, true, true, now())
		`, uuid.New(), tid, hostB); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO memberships (id, tenant_id, user_id, role)
			VALUES ($1, $2, $3, 'owner')
		`, uuid.New(), tid, userB); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to seed domain/membership for Tenant B: %v", err)
	}
	if _, err := settingsService.UpdateSettings(ctxB, tenantB, settings.UpdateSettingsParams{
		Title:        "کلینیک دکتر فردوس",
		PrimaryColor: "#10b981",
		TemplateID:   "minimal",
	}); err != nil {
		t.Fatalf("failed to set initial settings for Tenant B: %v", err)
	}

	// Seed active session for userA
	tokenA := "live-test-token-" + uuid.NewString()
	tokenHashA := auth.HashToken(tokenA)
	_, err = pool.Exec(ctx, `
		INSERT INTO panel_sessions (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, now() + interval '1 day')
	`, uuid.New(), userA, tokenHashA)
	if err != nil {
		t.Fatalf("failed to seed panel session for user A: %v", err)
	}

	for _, fixture := range []struct {
		ctx      context.Context
		id       uuid.UUID
		template string
		palette  string
	}{
		{ctxA, tenantA, "clinical", "ocean-trust"},
		{ctxB, tenantB, "minimal", "warm-sand"},
	} {
		if _, err := settingsService.UpdateAppearance(fixture.ctx, fixture.id, settings.UpdateAppearanceParams{
			TemplateID: fixture.template, PaletteID: fixture.palette,
		}); err != nil {
			t.Fatalf("seed fixture appearance: %v", err)
		}
	}
	unverifiedHost := "unverified-" + slugA + ".ir"
	if err := store.WithTenantTx(ctxA, func(tx pgx.Tx, tid uuid.UUID) error {
		_, err := tx.Exec(ctx, "INSERT INTO domains (tenant_id, host) VALUES ($1, $2)", tid, unverifiedHost)
		return err
	}); err != nil {
		t.Fatalf("seed unverified Host: %v", err)
	}
	assertUnknownPublicHost(t, serve, unverifiedHost)

	// 4. Verify Host A gets its public profile (200 OK)
	reqA := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqA.Host = hostA
	rrA := httptest.NewRecorder()
	serve(rrA, reqA)
	if rrA.Code != http.StatusOK {
		t.Fatalf("expected 200 for Host A public site, got %d: %s", rrA.Code, rrA.Body.String())
	}
	var resA settings.PublicSiteProfile
	if err := json.Unmarshal(rrA.Body.Bytes(), &resA); err != nil {
		t.Fatalf("failed to unmarshal Host A response: %v", err)
	}
	if resA.Title != "کلینیک دکتر شمس" || resA.PrimaryColor != "#0ea5e9" {
		t.Fatalf("unexpected Host A profile: %+v", resA)
	}

	// 5. Verify Host B gets its public profile (200 OK)
	reqB := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqB.Host = hostB
	rrB := httptest.NewRecorder()
	serve(rrB, reqB)
	if rrB.Code != http.StatusOK {
		t.Fatalf("expected 200 for Host B public site, got %d: %s", rrB.Code, rrB.Body.String())
	}
	var resB settings.PublicSiteProfile
	if err := json.Unmarshal(rrB.Body.Bytes(), &resB); err != nil {
		t.Fatalf("failed to unmarshal Host B response: %v", err)
	}
	if resB.Title != "کلینیک دکتر فردوس" || resB.PrimaryColor != "#10b981" {
		t.Fatalf("unexpected Host B profile: %+v", resB)
	}

	// 6. Authorized Admin GET for Tenant A by Owner A -> 200 OK
	reqAdminGet := httptest.NewRequest(http.MethodGet, "/v1/admin/site", nil)
	reqAdminGet.Header.Set("Authorization", "Bearer "+tokenA)
	reqAdminGet.Header.Set("X-Tenant-ID", tenantA.String())
	rrAdminGet := httptest.NewRecorder()
	serve(rrAdminGet, reqAdminGet)
	if rrAdminGet.Code != http.StatusOK {
		t.Fatalf("expected 200 for authorized admin get, got %d: %s", rrAdminGet.Code, rrAdminGet.Body.String())
	}
	var adminRes settings.SiteSettings
	if err := json.Unmarshal(rrAdminGet.Body.Bytes(), &adminRes); err != nil {
		t.Fatalf("failed to unmarshal admin get response: %v", err)
	}
	if adminRes.Title != "کلینیک دکتر شمس" {
		t.Fatalf("unexpected admin settings title: %s", adminRes.Title)
	}

	// 7. Authorized Admin PUT for Tenant A by Owner A -> 200 OK
	updateBody, _ := json.Marshal(map[string]string{
		"title": "کلینیک تخصصی دکتر شمس (به‌روزشده)",
		"phone": "02177778888",
	})
	reqAdminPut := httptest.NewRequest(http.MethodPut, "/v1/admin/site", bytes.NewBuffer(updateBody))
	reqAdminPut.Header.Set("Authorization", "Bearer "+tokenA)
	reqAdminPut.Header.Set("X-Tenant-ID", tenantA.String())
	reqAdminPut.Header.Set("Content-Type", "application/json")
	rrAdminPut := httptest.NewRecorder()
	serve(rrAdminPut, reqAdminPut)
	if rrAdminPut.Code != http.StatusOK {
		t.Fatalf("expected 200 for authorized admin put, got %d: %s", rrAdminPut.Code, rrAdminPut.Body.String())
	}

	// 8. Host A public profile now reflects the updated settings -> 200 OK
	reqAUpdated := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqAUpdated.Host = hostA
	rrAUpdated := httptest.NewRecorder()
	serve(rrAUpdated, reqAUpdated)
	if rrAUpdated.Code != http.StatusOK {
		t.Fatalf("expected 200 for Host A updated, got %d", rrAUpdated.Code)
	}
	var resAUpdated settings.PublicSiteProfile
	if err := json.Unmarshal(rrAUpdated.Body.Bytes(), &resAUpdated); err != nil {
		t.Fatal(err)
	}
	if resAUpdated.Title != "کلینیک تخصصی دکتر شمس (به‌روزشده)" || resAUpdated.Phone != "02177778888" ||
		resAUpdated.TemplateID != "clinical" || resAUpdated.PaletteID != "ocean-trust" {
		t.Fatalf("Host A profile was not updated: %+v", resAUpdated)
	}

	// 9. Host B public profile remains completely unchanged -> 200 OK
	reqBCheck := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqBCheck.Host = hostB
	rrBCheck := httptest.NewRecorder()
	serve(rrBCheck, reqBCheck)
	var resBCheck settings.PublicSiteProfile
	if err := json.Unmarshal(rrBCheck.Body.Bytes(), &resBCheck); err != nil {
		t.Fatal(err)
	}
	if rrBCheck.Code != http.StatusOK || resBCheck.Title != "کلینیک دکتر فردوس" ||
		resBCheck.PrimaryColor != "#10b981" || resBCheck.TemplateID != "minimal" || resBCheck.PaletteID != "warm-sand" {
		t.Fatalf("Host B profile was polluted by Host A update! Got: %+v", resBCheck)
	}

	// 10. Tampering attempt: Owner A attempts to read Tenant B settings -> 403 Forbidden
	reqTamperGet := httptest.NewRequest(http.MethodGet, "/v1/admin/site", nil)
	reqTamperGet.Header.Set("Authorization", "Bearer "+tokenA)
	reqTamperGet.Header.Set("X-Tenant-ID", tenantB.String())
	rrTamperGet := httptest.NewRecorder()
	serve(rrTamperGet, reqTamperGet)
	if rrTamperGet.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for cross-tenant read attempt, got %d: %s", rrTamperGet.Code, rrTamperGet.Body.String())
	}

	// 11. Tampering attempt: Owner A attempts to update Tenant B settings -> 403 Forbidden
	t.Run("denied_settings_write_has_no_side_effects", func(t *testing.T) {
		type clinicState struct {
			settings settingsdb.GetSiteSettingsRow
			outbox   string
			pending  bool
		}
		snapshotClinicB := func(ctx context.Context) clinicState {
			t.Helper()
			var state clinicState
			err := store.WithTenantTx(tenantctx.With(ctx, tenantB), func(tx pgx.Tx, tid uuid.UUID) error {
				var err error
				state.settings, err = settingsdb.New(tx).GetSiteSettings(ctx, toPgUUID(tid))
				if err != nil {
					return err
				}
				return tx.QueryRow(ctx, `SELECT coalesce(jsonb_agg(to_jsonb(event) ORDER BY event.id), '[]')::text,
				coalesce(bool_or(event.status IN ('pending', 'processing')), false)
				FROM outbox_events AS event WHERE event.tenant_id = $1`, tid).Scan(&state.outbox, &state.pending)
			})
			if err != nil {
				t.Fatalf("read persisted Clinic B settings and outbox: %v", err)
			}
			return state
		}
		// Let the real worker finish events emitted during setup before comparing every outbox field.
		snapshotCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		beforeDeniedWrite := snapshotClinicB(snapshotCtx)
		for beforeDeniedWrite.pending {
			time.Sleep(50 * time.Millisecond)
			beforeDeniedWrite = snapshotClinicB(snapshotCtx)
		}
		if beforeDeniedWrite.outbox == "[]" {
			t.Fatal("expected Clinic B fixture to include at least one settled outbox event")
		}
		reqTamperPut := httptest.NewRequest(http.MethodPut, "/v1/admin/site", bytes.NewBuffer(updateBody))
		reqTamperPut.Header.Set("Authorization", "Bearer "+tokenA)
		reqTamperPut.Header.Set("X-Tenant-ID", tenantB.String())
		reqTamperPut.Header.Set("Content-Type", "application/json")
		rrTamperPut := httptest.NewRecorder()
		serve(rrTamperPut, reqTamperPut)
		if rrTamperPut.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for cross-tenant update attempt, got %d: %s", rrTamperPut.Code, rrTamperPut.Body.String())
		}
		afterDeniedWrite := snapshotClinicB(snapshotCtx)
		if afterDeniedWrite.settings != beforeDeniedWrite.settings {
			t.Fatal("denied write changed Clinic B's persisted settings")
		}
		if afterDeniedWrite.outbox != beforeDeniedWrite.outbox {
			t.Fatal("denied write changed Clinic B's persisted outbox records")
		}
	})
	checkPublicHome(t, store, serve, tenantA, tenantB, userA, userB, hostA, hostB, tokenA)
	checkPublicBooking(t, store, tenantA, tenantB, hostA, hostB, serve)
	runFixtureBrowser(t, address, hostA, resAUpdated.Title, hostB, resBCheck.Title, tokenA, tenantA.String())
}

func TestServer_GracefulShutdownAndTimeouts(t *testing.T) {
	router := setupRouter(nil, nil, nil, nil, nil, nil, nil, nil, nil)

	srv := &http.Server{
		Addr:              ":0",
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	if srv.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("expected ReadHeaderTimeout = 5s, got %v", srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != 15*time.Second {
		t.Errorf("expected ReadTimeout = 15s, got %v", srv.ReadTimeout)
	}
	if srv.WriteTimeout != 30*time.Second {
		t.Errorf("expected WriteTimeout = 30s, got %v", srv.WriteTimeout)
	}
	if srv.IdleTimeout != 60*time.Second {
		t.Errorf("expected IdleTimeout = 60s, got %v", srv.IdleTimeout)
	}

	// Verify server starts and gracefully shuts down
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("graceful shutdown failed: %v", err)
	}
}
