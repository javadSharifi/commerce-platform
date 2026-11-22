package settings_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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

func (m *mockSettingsQuerier) GetSiteSettings(ctx context.Context, tenantID pgtype.UUID) (settingsdb.SiteSetting, error) {
	s, ok := m.settings[fromPgUUID(tenantID).String()]
	if !ok {
		return settingsdb.SiteSetting{}, pgx.ErrNoRows
	}
	return s, nil
}

func (m *mockSettingsQuerier) UpsertSiteSettings(ctx context.Context, arg settingsdb.UpsertSiteSettingsParams) (settingsdb.SiteSetting, error) {
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
	}
	m.settings[fromPgUUID(arg.TenantID).String()] = s
	return s, nil
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
		}, nil
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

	// 1. Invalid color format -> 400
	invalidColorBody, _ := json.Marshal(map[string]string{
		"title":         "کلینیک جدید",
		"primary_color": "not-a-hex",
	})
	reqInvalidColor := httptest.NewRequest(http.MethodPut, "/admin/site", bytes.NewBuffer(invalidColorBody))
	reqInvalidColor.Header.Set("Authorization", "Bearer valid-token-a")
	reqInvalidColor.Header.Set("X-Tenant-ID", tenantAID.String())
	reqInvalidColor.Header.Set("Content-Type", "application/json")
	rrInvalidColor := httptest.NewRecorder()
	r.ServeHTTP(rrInvalidColor, reqInvalidColor)
	if rrInvalidColor.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid color, got %d", rrInvalidColor.Code)
	}

	// 1.b Invalid template_id -> 400
	invalidTplBody, _ := json.Marshal(map[string]string{
		"title":         "کلینیک جدید",
		"primary_color": "#0ea5e9",
		"template_id":   "INVALID_CAPS!",
	})
	reqInvalidTpl := httptest.NewRequest(http.MethodPut, "/admin/site", bytes.NewBuffer(invalidTplBody))
	reqInvalidTpl.Header.Set("Authorization", "Bearer valid-token-a")
	reqInvalidTpl.Header.Set("X-Tenant-ID", tenantAID.String())
	reqInvalidTpl.Header.Set("Content-Type", "application/json")
	rrInvalidTpl := httptest.NewRecorder()
	r.ServeHTTP(rrInvalidTpl, reqInvalidTpl)
	if rrInvalidTpl.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid template_id, got %d", rrInvalidTpl.Code)
	}

	// 1.c Invalid theme_radius -> 400
	invalidRadiusBody, _ := json.Marshal(map[string]string{
		"title":         "کلینیک جدید",
		"primary_color": "#0ea5e9",
		"theme_radius":  "xl",
	})
	reqInvalidRadius := httptest.NewRequest(http.MethodPut, "/admin/site", bytes.NewBuffer(invalidRadiusBody))
	reqInvalidRadius.Header.Set("Authorization", "Bearer valid-token-a")
	reqInvalidRadius.Header.Set("X-Tenant-ID", tenantAID.String())
	reqInvalidRadius.Header.Set("Content-Type", "application/json")
	rrInvalidRadius := httptest.NewRecorder()
	r.ServeHTTP(rrInvalidRadius, reqInvalidRadius)
	if rrInvalidRadius.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid theme_radius, got %d", rrInvalidRadius.Code)
	}

	// 1.d Invalid theme_scale -> 400
	invalidScaleBody, _ := json.Marshal(map[string]string{
		"title":         "کلینیک جدید",
		"primary_color": "#0ea5e9",
		"theme_scale":   "2xl",
	})
	reqInvalidScale := httptest.NewRequest(http.MethodPut, "/admin/site", bytes.NewBuffer(invalidScaleBody))
	reqInvalidScale.Header.Set("Authorization", "Bearer valid-token-a")
	reqInvalidScale.Header.Set("X-Tenant-ID", tenantAID.String())
	reqInvalidScale.Header.Set("Content-Type", "application/json")
	rrInvalidScale := httptest.NewRecorder()
	r.ServeHTTP(rrInvalidScale, reqInvalidScale)
	if rrInvalidScale.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid theme_scale, got %d", rrInvalidScale.Code)
	}

	// 2. Valid Update for Tenant A by Owner -> 200
	validBody, _ := json.Marshal(map[string]string{
		"title":         "کلینیک تخصصی مهر",
		"description":   "روان‌شناسی سلامت",
		"primary_color": "#0ea5e9",
		"template_id":   "clinical",
		"theme_radius":  "lg",
		"theme_scale":   "sm",
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

	// Verify Tenant A settings are stored with template and active version
	sA, err := settingsService.GetSettings(context.Background(), tenantAID)
	if err != nil || sA.Title != "کلینیک تخصصی مهر" {
		t.Fatalf("expected title 'کلینیک تخصصی مهر', got %v", sA)
	}
	if sA.TemplateID != "clinical" || sA.TemplateVersion != "1.0.0" || sA.ThemeRadius != "lg" || sA.ThemeScale != "sm" {
		t.Fatalf("expected template fields clinical/1.0.0/lg/sm, got %+v", sA)
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
