package settings_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"psychology/api/internal/auth"
	"psychology/api/internal/auth/authdb"
	"psychology/api/internal/domain"
	"psychology/api/internal/domain/domaindb"
	"psychology/api/internal/platform/tenantctx"
	"psychology/api/internal/settings"
	"psychology/api/internal/settings/settingsdb"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type mockSettingsQuerier struct {
	settings map[string]settingsdb.SiteSetting
	clinics  map[string]string // id -> clinicName
}

func newMockSettingsQuerier() *mockSettingsQuerier {
	return &mockSettingsQuerier{
		settings: make(map[string]settingsdb.SiteSetting),
		clinics:  make(map[string]string),
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

func (m *mockSettingsQuerier) GetSiteSettings(ctx context.Context, tenantID pgtype.UUID) (settingsdb.GetSiteSettingsRow, error) {
	s, ok := m.settings[fromPgUUID(tenantID).String()]
	if !ok {
		return settingsdb.GetSiteSettingsRow{}, pgx.ErrNoRows
	}
	paletteID := s.PaletteID
	if paletteID == "" {
		paletteID = "calm-sage"
	}
	return settingsdb.GetSiteSettingsRow{
		TenantID:        s.TenantID,
		Title:           s.Title,
		Description:     s.Description,
		Phone:           s.Phone,
		Address:         s.Address,
		PrimaryColor:    s.PrimaryColor,
		LogoMediaID:     s.LogoMediaID,
		TemplateID:      s.TemplateID,
		TemplateVersion: s.TemplateVersion,
		ThemeRadius:     s.ThemeRadius,
		ThemeScale:      s.ThemeScale,
		PaletteID:       paletteID,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
	}, nil
}

func (m *mockSettingsQuerier) UpsertSiteSettings(ctx context.Context, arg settingsdb.UpsertSiteSettingsParams) (settingsdb.UpsertSiteSettingsRow, error) {
	s := settingsdb.SiteSetting{
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
	m.settings[fromPgUUID(arg.TenantID).String()] = s
	return settingsdb.UpsertSiteSettingsRow{
		TenantID:        s.TenantID,
		Title:           s.Title,
		Description:     s.Description,
		Phone:           s.Phone,
		Address:         s.Address,
		PrimaryColor:    s.PrimaryColor,
		LogoMediaID:     s.LogoMediaID,
		TemplateID:      s.TemplateID,
		TemplateVersion: s.TemplateVersion,
		ThemeRadius:     s.ThemeRadius,
		ThemeScale:      s.ThemeScale,
		PaletteID:       s.PaletteID,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
	}, nil
}

func (m *mockSettingsQuerier) GetSiteAppearance(ctx context.Context, tenantID pgtype.UUID) (settingsdb.GetSiteAppearanceRow, error) {
	s, ok := m.settings[fromPgUUID(tenantID).String()]
	if !ok {
		return settingsdb.GetSiteAppearanceRow{}, pgx.ErrNoRows
	}
	paletteID := s.PaletteID
	if paletteID == "" {
		paletteID = "calm-sage"
	}
	return settingsdb.GetSiteAppearanceRow{
		TenantID:        s.TenantID,
		TemplateID:      s.TemplateID,
		TemplateVersion: s.TemplateVersion,
		PaletteID:       paletteID,
		ThemeRadius:     s.ThemeRadius,
		ThemeScale:      s.ThemeScale,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
	}, nil
}

func (m *mockSettingsQuerier) UpsertSiteAppearance(ctx context.Context, arg settingsdb.UpsertSiteAppearanceParams) (settingsdb.UpsertSiteAppearanceRow, error) {
	key := fromPgUUID(arg.TenantID).String()
	s := m.settings[key]
	s.TenantID = arg.TenantID
	s.TemplateID = arg.TemplateID
	s.TemplateVersion = arg.TemplateVersion
	s.PaletteID = arg.PaletteID
	s.ThemeRadius = arg.ThemeRadius
	s.ThemeScale = arg.ThemeScale
	m.settings[key] = s
	return settingsdb.UpsertSiteAppearanceRow{
		TenantID:        s.TenantID,
		TemplateID:      s.TemplateID,
		TemplateVersion: s.TemplateVersion,
		PaletteID:       s.PaletteID,
		ThemeRadius:     s.ThemeRadius,
		ThemeScale:      s.ThemeScale,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
	}, nil
}

func (m *mockSettingsQuerier) UpsertSiteProfile(ctx context.Context, arg settingsdb.UpsertSiteProfileParams) (settingsdb.UpsertSiteProfileRow, error) {
	key := fromPgUUID(arg.TenantID).String()
	s := m.settings[key]
	s.TenantID = arg.TenantID
	s.Title = arg.Title
	s.Description = arg.Description
	s.Phone = arg.Phone
	s.Address = arg.Address
	s.PrimaryColor = arg.PrimaryColor
	s.LogoMediaID = arg.LogoMediaID
	if s.TemplateID == "" {
		s.TemplateID = "minimal"
	}
	if s.TemplateVersion == "" {
		s.TemplateVersion = "1.0.0"
	}
	if s.ThemeRadius == "" {
		s.ThemeRadius = "md"
	}
	if s.ThemeScale == "" {
		s.ThemeScale = "md"
	}
	if s.PaletteID == "" {
		s.PaletteID = "calm-sage"
	}
	m.settings[key] = s
	return settingsdb.UpsertSiteProfileRow{
		TenantID:        s.TenantID,
		Title:           s.Title,
		Description:     s.Description,
		Phone:           s.Phone,
		Address:         s.Address,
		PrimaryColor:    s.PrimaryColor,
		LogoMediaID:     s.LogoMediaID,
		TemplateID:      s.TemplateID,
		TemplateVersion: s.TemplateVersion,
		ThemeRadius:     s.ThemeRadius,
		ThemeScale:      s.ThemeScale,
		PaletteID:       s.PaletteID,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
	}, nil
}

func (m *mockSettingsQuerier) GetPublicSiteProfile(ctx context.Context, id pgtype.UUID) (settingsdb.GetPublicSiteProfileRow, error) {
	clinicName, ok := m.clinics[fromPgUUID(id).String()]
	if !ok {
		return settingsdb.GetPublicSiteProfileRow{}, pgx.ErrNoRows
	}
	s, ok := m.settings[fromPgUUID(id).String()]
	if !ok {
		return settingsdb.GetPublicSiteProfileRow{
			ClinicName:      clinicName,
			Title:           pgtype.Text{String: clinicName, Valid: true},
			Description:     pgtype.Text{Valid: false},
			PrimaryColor:    pgtype.Text{String: "#0284c7", Valid: true},
			TemplateID:      "minimal",
			TemplateVersion: "1.0.0",
			ThemeRadius:     "md",
			ThemeScale:      "md",
			PaletteID:       "calm-sage",
		}, nil
	}
	paletteID := s.PaletteID
	if paletteID == "" {
		paletteID = "calm-sage"
	}
	return settingsdb.GetPublicSiteProfileRow{
		ClinicName:      clinicName,
		Title:           pgtype.Text{String: s.Title, Valid: true},
		Description:     pgtype.Text{String: s.Description, Valid: true},
		Phone:           pgtype.Text{String: s.Phone, Valid: true},
		Address:         pgtype.Text{String: s.Address, Valid: true},
		PrimaryColor:    pgtype.Text{String: s.PrimaryColor, Valid: true},
		LogoMediaID:     s.LogoMediaID,
		TemplateID:      s.TemplateID,
		TemplateVersion: s.TemplateVersion,
		ThemeRadius:     s.ThemeRadius,
		ThemeScale:      s.ThemeScale,
		PaletteID:       paletteID,
	}, nil
}

func (m *mockSettingsQuerier) GetPrimaryVerifiedDomain(ctx context.Context, tenantID pgtype.UUID) (string, error) {
	return "test-clinic.ir", nil
}

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

func TestPublicSiteProfile(t *testing.T) {
	tenantAID := uuid.New()
	mockSettings := newMockSettingsQuerier()
	mockSettings.clinics[tenantAID.String()] = "کلینیک آرامش"
	mockSettings.settings[tenantAID.String()] = settingsdb.SiteSetting{
		TenantID:     toPgUUID(tenantAID),
		Title:        "دکتر کیانی - روان‌درمانگر",
		Description:  "مشاوره فردی و زوج‌درمانی",
		Phone:        "02122222222",
		Address:      "تهران، ونک",
		PrimaryColor: "#10b981",
	}

	mockDomain := &mockDomainQuerier{
		domains: map[string]domaindb.Domain{
			"dr-kiani.ir": {
				Host:       "dr-kiani.ir",
				TenantID:   toPgUUID(tenantAID),
				IsVerified: true,
			},
		},
	}

	settingsService := settings.NewService(mockSettings)
	domainResolver := domain.NewResolver(mockDomain)
	handler := settings.NewHandler(settingsService, domainResolver, nil)

	r := chi.NewRouter()
	handler.MountRoutes(r)

	// Call public endpoint with Host header
	req := httptest.NewRequest(http.MethodGet, "/public/site", nil)
	req.Host = "dr-kiani.ir"
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp settings.PublicSiteProfile
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}

	if resp.Name != "کلینیک آرامش" {
		t.Errorf("expected clinic name 'کلینیک آرامش', got %s", resp.Name)
	}
	if resp.PrimaryColor != "#10b981" {
		t.Errorf("expected primary color '#10b981', got %s", resp.PrimaryColor)
	}
	if resp.Title != "دکتر کیانی - روان‌درمانگر" {
		t.Errorf("expected title 'دکتر کیانی - روان‌درمانگر', got %s", resp.Title)
	}
}

func TestAdminUpdateSiteSettingsValidationAndIsolation(t *testing.T) {
	tenantAID := uuid.New()
	tenantBID := uuid.New()
	userAID := uuid.New()

	mockSettings := newMockSettingsQuerier()
	mockAuth := &mockAuthQuerier{
		identities: map[string]authdb.PanelIdentity{
			userAID.String(): {
				ID:       toPgUUID(userAID),
				Email:    "owner@clinic-a.ir",
				FullName: "Doctor A",
				Status:   "active",
			},
		},
		sessions: map[string]authdb.PanelSession{
			auth.HashToken("valid-token-a"): {
				ID:        toPgUUID(uuid.New()),
				UserID:    toPgUUID(userAID),
				TokenHash: auth.HashToken("valid-token-a"),
			},
		},
		memberships: map[string]authdb.Membership{
			userAID.String() + ":" + tenantAID.String(): {
				ID:       toPgUUID(uuid.New()),
				TenantID: toPgUUID(tenantAID),
				UserID:   toPgUUID(userAID),
				Role:     "owner",
			},
		},
	}

	settingsService := settings.NewService(mockSettings)
	authService := auth.NewService(mockAuth)
	handler := settings.NewHandler(settingsService, nil, authService)

	r := chi.NewRouter()
	handler.MountRoutes(r)

	// 1. Rejection of appearance fields on PUT /admin/site -> 400 APPEARANCE_FIELDS_NOT_ALLOWED
	appearanceFieldTests := []struct {
		name string
		body map[string]string
	}{
		{
			name: "primary_color is rejected",
			body: map[string]string{
				"title":         "کلینیک جدید",
				"primary_color": "#0ea5e9",
			},
		},
		{
			name: "template_id is rejected",
			body: map[string]string{
				"title":       "کلینیک جدید",
				"template_id": "clinical",
			},
		},
		{
			name: "palette_id is rejected",
			body: map[string]string{
				"title":      "کلینیک جدید",
				"palette_id": "calm-sage",
			},
		},
		{
			name: "theme_radius is rejected",
			body: map[string]string{
				"title":        "کلینیک جدید",
				"theme_radius": "lg",
			},
		},
		{
			name: "theme_scale is rejected",
			body: map[string]string{
				"title":       "کلینیک جدید",
				"theme_scale": "sm",
			},
		},
	}

	for _, tc := range appearanceFieldTests {
		b, _ := json.Marshal(tc.body)
		req := httptest.NewRequest(http.MethodPut, "/admin/site", bytes.NewBuffer(b))
		req.Header.Set("Authorization", "Bearer valid-token-a")
		req.Header.Set("X-Tenant-ID", tenantAID.String())
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("[%s] expected 400 Bad Request, got %d: %s", tc.name, rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "APPEARANCE_FIELDS_NOT_ALLOWED") {
			t.Fatalf("[%s] expected code APPEARANCE_FIELDS_NOT_ALLOWED, got: %s", tc.name, rr.Body.String())
		}
	}

	// 1.b Content validation: empty title -> 400
	emptyTitleBody, _ := json.Marshal(map[string]string{
		"title": "",
	})
	reqEmptyTitle := httptest.NewRequest(http.MethodPut, "/admin/site", bytes.NewBuffer(emptyTitleBody))
	reqEmptyTitle.Header.Set("Authorization", "Bearer valid-token-a")
	reqEmptyTitle.Header.Set("X-Tenant-ID", tenantAID.String())
	reqEmptyTitle.Header.Set("Content-Type", "application/json")
	rrEmptyTitle := httptest.NewRecorder()
	r.ServeHTTP(rrEmptyTitle, reqEmptyTitle)
	if rrEmptyTitle.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty title, got %d", rrEmptyTitle.Code)
	}

	// 2. Valid Content Update for Tenant A by Owner -> 200
	validBody, _ := json.Marshal(map[string]string{
		"title":       "کلینیک تخصصی مهر",
		"description": "روان‌شناسی سلامت",
		"phone":       "02188887777",
		"address":     "تهران، میدان ونک",
	})
	reqValid := httptest.NewRequest(http.MethodPut, "/admin/site", bytes.NewBuffer(validBody))
	reqValid.Header.Set("Authorization", "Bearer valid-token-a")
	reqValid.Header.Set("X-Tenant-ID", tenantAID.String())
	reqValid.Header.Set("Content-Type", "application/json")
	rrValid := httptest.NewRecorder()
	r.ServeHTTP(rrValid, reqValid)
	if rrValid.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid settings update, got %d: %s", rrValid.Code, rrValid.Body.String())
	}
	if rrValid.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("expected Cache-Control: private, no-store, got %s", rrValid.Header().Get("Cache-Control"))
	}

	// Verify Tenant A content settings are stored
	sA, err := settingsService.GetSettings(context.Background(), tenantAID)
	if err != nil || sA.Title != "کلینیک تخصصی مهر" {
		t.Fatalf("expected title 'کلینیک تخصصی مهر', got %v", sA)
	}
	if sA.Description != "روان‌شناسی سلامت" || sA.Phone != "02188887777" || sA.Address != "تهران، میدان ونک" {
		t.Fatalf("expected content fields updated, got %+v", sA)
	}

	// 3. User A attempts to update Tenant B -> 403 Forbidden
	reqCross := httptest.NewRequest(http.MethodPut, "/admin/site", bytes.NewBuffer(validBody))
	reqCross.Header.Set("Authorization", "Bearer valid-token-a")
	reqCross.Header.Set("X-Tenant-ID", tenantBID.String())
	reqCross.Header.Set("Content-Type", "application/json")
	rrCross := httptest.NewRecorder()
	r.ServeHTTP(rrCross, reqCross)
	if rrCross.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unauthorized tenant update, got %d", rrCross.Code)
	}
}

type mockSpyTenantStore struct {
	calledTenant uuid.UUID
	errToReturn  error
}

func (m *mockSpyTenantStore) WithTenantTx(ctx context.Context, fn func(tx pgx.Tx, tenantID uuid.UUID) error) error {
	tid, ok := tenantctx.ID(ctx)
	if !ok {
		return errors.New("missing tenant in context")
	}
	m.calledTenant = tid
	if m.errToReturn != nil {
		return m.errToReturn
	}
	return nil
}

func TestServiceWithStore_ErrorPropagation(t *testing.T) {
	expectedErr := errors.New("db error")
	spy := &mockSpyTenantStore{errToReturn: expectedErr}
	svc := settings.NewServiceWithStore(spy)

	tenantID := uuid.New()
	ctx := tenantctx.With(context.Background(), tenantID)

	// 1. GetSettings executes within WithTenantTx and propagates error
	_, err := svc.GetSettings(ctx, tenantID)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
	if spy.calledTenant != tenantID {
		t.Fatalf("expected tenant %v, got %v", tenantID, spy.calledTenant)
	}

	// 2. UpdateSettings executes within WithTenantTx and propagates error
	spy.calledTenant = uuid.Nil
	_, err = svc.UpdateSettings(ctx, tenantID, settings.UpdateSettingsParams{
		Title:        "کلینیک تست",
		PrimaryColor: "#0284c7",
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
	if spy.calledTenant != tenantID {
		t.Fatalf("expected tenant %v, got %v", tenantID, spy.calledTenant)
	}

	// 3. GetPublicProfile executes within WithTenantTx and propagates error
	spy.calledTenant = uuid.Nil
	_, err = svc.GetPublicProfile(ctx, tenantID)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
	if spy.calledTenant != tenantID {
		t.Fatalf("expected tenant %v, got %v", tenantID, spy.calledTenant)
	}
}

func TestAdminAppearance_GuardValidationAndIsolation(t *testing.T) {
	tenantAID := uuid.New()
	tenantBID := uuid.New()
	ownerUserID := uuid.New()
	regularUserID := uuid.New()

	mockSettings := newMockSettingsQuerier()
	mockAuth := &mockAuthQuerier{
		identities: map[string]authdb.PanelIdentity{
			ownerUserID.String(): {
				ID:       toPgUUID(ownerUserID),
				Email:    "appearance-owner@platform.ir",
				FullName: "مالک پلتفرم",
				Status:   "active",
			},
			regularUserID.String(): {
				ID:       toPgUUID(regularUserID),
				Email:    "regular-owner@clinic-a.ir",
				FullName: "پزشک مطب الف",
				Status:   "active",
			},
		},
		sessions: map[string]authdb.PanelSession{
			auth.HashToken("token-appearance-owner"): {
				ID:        toPgUUID(uuid.New()),
				UserID:    toPgUUID(ownerUserID),
				TokenHash: auth.HashToken("token-appearance-owner"),
			},
			auth.HashToken("token-regular-user"): {
				ID:        toPgUUID(uuid.New()),
				UserID:    toPgUUID(regularUserID),
				TokenHash: auth.HashToken("token-regular-user"),
			},
		},
		memberships: map[string]authdb.Membership{
			ownerUserID.String() + ":" + tenantAID.String(): {
				ID:       toPgUUID(uuid.New()),
				TenantID: toPgUUID(tenantAID),
				UserID:   toPgUUID(ownerUserID),
				Role:     "owner",
			},
			regularUserID.String() + ":" + tenantAID.String(): {
				ID:       toPgUUID(uuid.New()),
				TenantID: toPgUUID(tenantAID),
				UserID:   toPgUUID(regularUserID),
				Role:     "owner",
			},
		},
	}

	settingsService := settings.NewService(mockSettings)
	authService := auth.NewService(mockAuth)
	handler := settings.NewHandler(settingsService, nil, authService)

	// Set Appearance Owner User ID
	handler.SetAppearanceOwnerUserID(ownerUserID)

	r := chi.NewRouter()
	handler.MountRoutes(r)

	// --- GET /admin/site/appearance tests ---

	// 1. Without session -> 401 UNAUTHORIZED
	reqNoAuth := httptest.NewRequest(http.MethodGet, "/admin/site/appearance", nil)
	reqNoAuth.Header.Set("X-Tenant-ID", tenantAID.String())
	rrNoAuth := httptest.NewRecorder()
	r.ServeHTTP(rrNoAuth, reqNoAuth)
	if rrNoAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", rrNoAuth.Code)
	}

	// 2. Without X-Tenant-ID -> 400 MISSING_TENANT
	reqNoTenant := httptest.NewRequest(http.MethodGet, "/admin/site/appearance", nil)
	reqNoTenant.Header.Set("Authorization", "Bearer token-appearance-owner")
	rrNoTenant := httptest.NewRecorder()
	r.ServeHTTP(rrNoTenant, reqNoTenant)
	if rrNoTenant.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without X-Tenant-ID, got %d", rrNoTenant.Code)
	}

	// 3. User is not a member of Tenant B -> 403 FORBIDDEN
	reqNotMember := httptest.NewRequest(http.MethodGet, "/admin/site/appearance", nil)
	reqNotMember.Header.Set("Authorization", "Bearer token-appearance-owner")
	reqNotMember.Header.Set("X-Tenant-ID", tenantBID.String())
	rrNotMember := httptest.NewRecorder()
	r.ServeHTTP(rrNotMember, reqNotMember)
	if rrNotMember.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when not a member of tenant, got %d", rrNotMember.Code)
	}

	// 4. User is a member of Tenant A, but is NOT the configured appearance owner -> 403 FORBIDDEN
	reqNotOwner := httptest.NewRequest(http.MethodGet, "/admin/site/appearance", nil)
	reqNotOwner.Header.Set("Authorization", "Bearer token-regular-user")
	reqNotOwner.Header.Set("X-Tenant-ID", tenantAID.String())
	rrNotOwner := httptest.NewRecorder()
	r.ServeHTTP(rrNotOwner, reqNotOwner)
	if rrNotOwner.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-appearance owner, got %d", rrNotOwner.Code)
	}

	// 5. Valid appearance owner -> 200 OK with default appearance
	reqValidGet := httptest.NewRequest(http.MethodGet, "/admin/site/appearance", nil)
	reqValidGet.Header.Set("Authorization", "Bearer token-appearance-owner")
	reqValidGet.Header.Set("X-Tenant-ID", tenantAID.String())
	rrValidGet := httptest.NewRecorder()
	r.ServeHTTP(rrValidGet, reqValidGet)
	if rrValidGet.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid appearance owner, got %d: %s", rrValidGet.Code, rrValidGet.Body.String())
	}
	if rrValidGet.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("expected Cache-Control: private, no-store, got %q", rrValidGet.Header().Get("Cache-Control"))
	}

	var initResp map[string]string
	_ = json.Unmarshal(rrValidGet.Body.Bytes(), &initResp)
	if initResp["template_id"] != "minimal" || initResp["palette_id"] != "calm-sage" || initResp["theme_radius"] != "md" || initResp["theme_scale"] != "md" {
		t.Fatalf("expected default appearance fields, got %+v", initResp)
	}

	// --- PUT /admin/site/appearance tests ---

	// 6. CSRF Detection: Sec-Fetch-Site cross-site -> 403
	bodyValid, _ := json.Marshal(map[string]string{
		"template_id": "mobile-app",
		"palette_id":  "ocean-trust",
	})
	reqCSRF := httptest.NewRequest(http.MethodPut, "/admin/site/appearance", bytes.NewBuffer(bodyValid))
	reqCSRF.Header.Set("Authorization", "Bearer token-appearance-owner")
	reqCSRF.Header.Set("X-Tenant-ID", tenantAID.String())
	reqCSRF.Header.Set("Content-Type", "application/json")
	reqCSRF.Header.Set("Sec-Fetch-Site", "cross-site")
	rrCSRF := httptest.NewRecorder()
	r.ServeHTTP(rrCSRF, reqCSRF)
	if rrCSRF.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on CSRF cross-site, got %d", rrCSRF.Code)
	}

	// 7. Invalid Content-Type -> 415
	reqMediaType := httptest.NewRequest(http.MethodPut, "/admin/site/appearance", bytes.NewBuffer(bodyValid))
	reqMediaType.Header.Set("Authorization", "Bearer token-appearance-owner")
	reqMediaType.Header.Set("X-Tenant-ID", tenantAID.String())
	reqMediaType.Header.Set("Content-Type", "text/plain")
	rrMediaType := httptest.NewRecorder()
	r.ServeHTTP(rrMediaType, reqMediaType)
	if rrMediaType.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415 on invalid content-type, got %d", rrMediaType.Code)
	}

	// 8. Reject template_version (out of contract field) -> 400
	bodyWithVersion, _ := json.Marshal(map[string]string{
		"template_id":      "mobile-app",
		"palette_id":       "ocean-trust",
		"template_version": "1.0.0",
	})
	reqVersion := httptest.NewRequest(http.MethodPut, "/admin/site/appearance", bytes.NewBuffer(bodyWithVersion))
	reqVersion.Header.Set("Authorization", "Bearer token-appearance-owner")
	reqVersion.Header.Set("X-Tenant-ID", tenantAID.String())
	reqVersion.Header.Set("Content-Type", "application/json")
	rrVersion := httptest.NewRecorder()
	r.ServeHTTP(rrVersion, reqVersion)
	if rrVersion.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when sending template_version, got %d: %s", rrVersion.Code, rrVersion.Body.String())
	}

	// 9. Reject extra out-of-contract field -> 400
	bodyWithExtra, _ := json.Marshal(map[string]any{
		"template_id": "mobile-app",
		"palette_id":  "ocean-trust",
		"title":       "should be rejected",
	})
	reqExtra := httptest.NewRequest(http.MethodPut, "/admin/site/appearance", bytes.NewBuffer(bodyWithExtra))
	reqExtra.Header.Set("Authorization", "Bearer token-appearance-owner")
	reqExtra.Header.Set("X-Tenant-ID", tenantAID.String())
	reqExtra.Header.Set("Content-Type", "application/json")
	rrExtra := httptest.NewRecorder()
	r.ServeHTTP(rrExtra, reqExtra)
	if rrExtra.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when sending unknown field, got %d", rrExtra.Code)
	}

	// 10. Missing template_id -> 400
	bodyNoTpl, _ := json.Marshal(map[string]string{
		"palette_id": "ocean-trust",
	})
	reqNoTpl := httptest.NewRequest(http.MethodPut, "/admin/site/appearance", bytes.NewBuffer(bodyNoTpl))
	reqNoTpl.Header.Set("Authorization", "Bearer token-appearance-owner")
	reqNoTpl.Header.Set("X-Tenant-ID", tenantAID.String())
	reqNoTpl.Header.Set("Content-Type", "application/json")
	rrNoTpl := httptest.NewRecorder()
	r.ServeHTTP(rrNoTpl, reqNoTpl)
	if rrNoTpl.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing template_id, got %d", rrNoTpl.Code)
	}

	// 11. Missing palette_id -> 400
	bodyNoPal, _ := json.Marshal(map[string]string{
		"template_id": "mobile-app",
	})
	reqNoPal := httptest.NewRequest(http.MethodPut, "/admin/site/appearance", bytes.NewBuffer(bodyNoPal))
	reqNoPal.Header.Set("Authorization", "Bearer token-appearance-owner")
	reqNoPal.Header.Set("X-Tenant-ID", tenantAID.String())
	reqNoPal.Header.Set("Content-Type", "application/json")
	rrNoPal := httptest.NewRecorder()
	r.ServeHTTP(rrNoPal, reqNoPal)
	if rrNoPal.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing palette_id, got %d", rrNoPal.Code)
	}

	// 12. Invalid template_id -> 400
	bodyInvTpl, _ := json.Marshal(map[string]string{
		"template_id": "nonexistent-template",
		"palette_id":  "ocean-trust",
	})
	reqInvTpl := httptest.NewRequest(http.MethodPut, "/admin/site/appearance", bytes.NewBuffer(bodyInvTpl))
	reqInvTpl.Header.Set("Authorization", "Bearer token-appearance-owner")
	reqInvTpl.Header.Set("X-Tenant-ID", tenantAID.String())
	reqInvTpl.Header.Set("Content-Type", "application/json")
	rrInvTpl := httptest.NewRecorder()
	r.ServeHTTP(rrInvTpl, reqInvTpl)
	if rrInvTpl.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid template_id, got %d", rrInvTpl.Code)
	}

	// 13. Invalid palette_id -> 400
	bodyInvPal, _ := json.Marshal(map[string]string{
		"template_id": "mobile-app",
		"palette_id":  "neon-pink",
	})
	reqInvPal := httptest.NewRequest(http.MethodPut, "/admin/site/appearance", bytes.NewBuffer(bodyInvPal))
	reqInvPal.Header.Set("Authorization", "Bearer token-appearance-owner")
	reqInvPal.Header.Set("X-Tenant-ID", tenantAID.String())
	reqInvPal.Header.Set("Content-Type", "application/json")
	rrInvPal := httptest.NewRecorder()
	r.ServeHTTP(rrInvPal, reqInvPal)
	if rrInvPal.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid palette_id, got %d", rrInvPal.Code)
	}

	// 14. Invalid theme_radius -> 400
	bodyInvRad, _ := json.Marshal(map[string]string{
		"template_id":  "mobile-app",
		"palette_id":   "ocean-trust",
		"theme_radius": "xl",
	})
	reqInvRad := httptest.NewRequest(http.MethodPut, "/admin/site/appearance", bytes.NewBuffer(bodyInvRad))
	reqInvRad.Header.Set("Authorization", "Bearer token-appearance-owner")
	reqInvRad.Header.Set("X-Tenant-ID", tenantAID.String())
	reqInvRad.Header.Set("Content-Type", "application/json")
	rrInvRad := httptest.NewRecorder()
	r.ServeHTTP(rrInvRad, reqInvRad)
	if rrInvRad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid theme_radius, got %d", rrInvRad.Code)
	}

	// 15. Valid PUT appearance: template_id and palette_id, radius and scale omitted -> 200 OK
	reqValidPut := httptest.NewRequest(http.MethodPut, "/admin/site/appearance", bytes.NewBuffer(bodyValid))
	reqValidPut.Header.Set("Authorization", "Bearer token-appearance-owner")
	reqValidPut.Header.Set("X-Tenant-ID", tenantAID.String())
	reqValidPut.Header.Set("Content-Type", "application/json")
	rrValidPut := httptest.NewRecorder()
	r.ServeHTTP(rrValidPut, reqValidPut)
	if rrValidPut.Code != http.StatusOK {
		t.Fatalf("expected 200 on valid PUT appearance, got %d: %s", rrValidPut.Code, rrValidPut.Body.String())
	}
	if rrValidPut.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("expected Cache-Control: private, no-store, got %q", rrValidPut.Header().Get("Cache-Control"))
	}

	var putResp map[string]string
	_ = json.Unmarshal(rrValidPut.Body.Bytes(), &putResp)
	if putResp["template_id"] != "mobile-app" || putResp["palette_id"] != "ocean-trust" || putResp["theme_radius"] != "md" || putResp["theme_scale"] != "md" {
		t.Fatalf("unexpected PUT response: %+v", putResp)
	}

	// 16. Update with specific radius and scale -> 200 OK
	bodyWithRadScale, _ := json.Marshal(map[string]string{
		"template_id":  "clinical",
		"palette_id":   "warm-sand",
		"theme_radius": "lg",
		"theme_scale":  "sm",
	})
	reqRadScale := httptest.NewRequest(http.MethodPut, "/admin/site/appearance", bytes.NewBuffer(bodyWithRadScale))
	reqRadScale.Header.Set("Authorization", "Bearer token-appearance-owner")
	reqRadScale.Header.Set("X-Tenant-ID", tenantAID.String())
	reqRadScale.Header.Set("Content-Type", "application/json")
	rrRadScale := httptest.NewRecorder()
	r.ServeHTTP(rrRadScale, reqRadScale)
	if rrRadScale.Code != http.StatusOK {
		t.Fatalf("expected 200 on radius/scale update, got %d: %s", rrRadScale.Code, rrRadScale.Body.String())
	}

	var radScaleResp map[string]string
	_ = json.Unmarshal(rrRadScale.Body.Bytes(), &radScaleResp)
	if radScaleResp["template_id"] != "clinical" || radScaleResp["palette_id"] != "warm-sand" || radScaleResp["theme_radius"] != "lg" || radScaleResp["theme_scale"] != "sm" {
		t.Fatalf("unexpected radius/scale update response: %+v", radScaleResp)
	}

	// 17. Verify Subsequent GET reflects updated appearance
	reqGetUpdated := httptest.NewRequest(http.MethodGet, "/admin/site/appearance", nil)
	reqGetUpdated.Header.Set("Authorization", "Bearer token-appearance-owner")
	reqGetUpdated.Header.Set("X-Tenant-ID", tenantAID.String())
	rrGetUpdated := httptest.NewRecorder()
	r.ServeHTTP(rrGetUpdated, reqGetUpdated)
	if rrGetUpdated.Code != http.StatusOK {
		t.Fatalf("expected 200 on subsequent GET, got %d", rrGetUpdated.Code)
	}
	var getUpResp map[string]string
	_ = json.Unmarshal(rrGetUpdated.Body.Bytes(), &getUpResp)
	if getUpResp["template_id"] != "clinical" || getUpResp["palette_id"] != "warm-sand" || getUpResp["theme_radius"] != "lg" || getUpResp["theme_scale"] != "sm" {
		t.Fatalf("GET appearance mismatch: %+v", getUpResp)
	}

	// 18. Cross-tenant isolation on PUT: attempt to update Tenant B without membership -> 403
	reqCrossPut := httptest.NewRequest(http.MethodPut, "/admin/site/appearance", bytes.NewBuffer(bodyValid))
	reqCrossPut.Header.Set("Authorization", "Bearer token-appearance-owner")
	reqCrossPut.Header.Set("X-Tenant-ID", tenantBID.String())
	reqCrossPut.Header.Set("Content-Type", "application/json")
	rrCrossPut := httptest.NewRecorder()
	r.ServeHTTP(rrCrossPut, reqCrossPut)
	if rrCrossPut.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on cross-tenant PUT appearance, got %d", rrCrossPut.Code)
	}
}

// TestGuard_CatalogAndVersionConsistency verifies PSY-6.5.3:
// 1. Matches Go active template IDs and versions against packages/ui/src/template-catalog.ts.
// 2. Matches Go active palette IDs against packages/ui/src/palette-catalog.ts.
// 3. Verifies OpenAPI contract schemas define palette_id and appearance routes.
// 4. Verifies rejection of uncatalogued IDs in Go service.
func TestGuard_CatalogAndVersionConsistency(t *testing.T) {
	// 1. Verify template-catalog.ts matches Go active templates
	candidatesTpl := []string{
		"../../../../packages/ui/src/template-catalog.ts",
		"../../../packages/ui/src/template-catalog.ts",
		"packages/ui/src/template-catalog.ts",
	}
	var tplContent string
	for _, c := range candidatesTpl {
		if data, err := os.ReadFile(c); err == nil {
			tplContent = string(data)
			break
		}
	}
	if tplContent == "" {
		t.Fatalf("could not read template-catalog.ts")
	}

	expectedTemplates := map[string]string{
		"minimal":    "1.0.0",
		"clinical":   "1.0.0",
		"mobile-app": "1.0.0",
		"editorial":  "1.0.0",
	}
	for id, ver := range expectedTemplates {
		if !strings.Contains(tplContent, "id: '"+id+"'") {
			t.Fatalf("template-catalog.ts missing template id %q", id)
		}
		if !strings.Contains(tplContent, "version: '"+ver+"'") {
			t.Fatalf("template-catalog.ts missing template version %q for %q", ver, id)
		}
	}

	// 2. Verify palette-catalog.ts matches Go active palettes
	candidatesPal := []string{
		"../../../../packages/ui/src/palette-catalog.ts",
		"../../../packages/ui/src/palette-catalog.ts",
		"packages/ui/src/palette-catalog.ts",
	}
	var palContent string
	for _, c := range candidatesPal {
		if data, err := os.ReadFile(c); err == nil {
			palContent = string(data)
			break
		}
	}
	if palContent == "" {
		t.Fatalf("could not read palette-catalog.ts")
	}

	expectedPalettes := []string{
		"calm-sage",
		"ocean-trust",
		"warm-sand",
		"royal-plum",
		"stitch-editorial",
	}
	for _, pid := range expectedPalettes {
		if !strings.Contains(palContent, "id: '"+pid+"'") {
			t.Fatalf("palette-catalog.ts missing palette id %q", pid)
		}
	}

	// 3. Verify OpenAPI contract specifies appearance routes and palette_id
	candidatesOpenAPI := []string{
		"../../../../packages/contracts/psychology/openapi.yaml",
		"../../../packages/contracts/psychology/openapi.yaml",
		"packages/contracts/psychology/openapi.yaml",
	}
	var openapiContent string
	for _, c := range candidatesOpenAPI {
		if data, err := os.ReadFile(c); err == nil {
			openapiContent = string(data)
			break
		}
	}
	if openapiContent == "" {
		t.Fatalf("could not read openapi.yaml")
	}

	requiredOpenAPISnippets := []string{
		"/admin/site/appearance:",
		"operationId: getAdminAppearance",
		"operationId: updateAdminAppearance",
		"SiteAppearanceResponse:",
		"UpdateSiteAppearanceRequest:",
		"palette_id:",
	}
	for _, s := range requiredOpenAPISnippets {
		if !strings.Contains(openapiContent, s) {
			t.Fatalf("openapi.yaml missing required appearance schema/route snippet: %q", s)
		}
	}

	// 4. Verify Go service rejects any uncatalogued template or palette
	svc := settings.NewService(newMockSettingsQuerier())
	tenantID := uuid.New()

	// Invalid template
	_, err := svc.UpdateAppearance(context.Background(), tenantID, settings.UpdateAppearanceParams{
		TemplateID: "unknown-tpl-404",
		PaletteID:  "calm-sage",
	})
	if !errors.Is(err, settings.ErrInvalidTemplateID) {
		t.Fatalf("expected ErrInvalidTemplateID for unknown template, got %v", err)
	}

	// Invalid palette
	_, err = svc.UpdateAppearance(context.Background(), tenantID, settings.UpdateAppearanceParams{
		TemplateID: "minimal",
		PaletteID:  "unknown-palette-404",
	})
	if !errors.Is(err, settings.ErrInvalidPaletteID) {
		t.Fatalf("expected ErrInvalidPaletteID for unknown palette, got %v", err)
	}
}

func TestUpdateAppearanceAcceptsStitchPalette(t *testing.T) {
	svc := settings.NewService(newMockSettingsQuerier())
	appearance, err := svc.UpdateAppearance(context.Background(), uuid.New(), settings.UpdateAppearanceParams{
		TemplateID: "editorial",
		PaletteID:  "stitch-editorial",
	})
	if err != nil {
		t.Fatal(err)
	}
	if appearance.PaletteID != "stitch-editorial" || appearance.TemplateID != "editorial" {
		t.Fatalf("unexpected saved appearance: %+v", appearance)
	}
}
