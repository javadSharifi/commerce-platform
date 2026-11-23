package testsupport_test

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
	"psychology/api/internal/settings"
	"psychology/api/internal/settings/settingsdb"

	"github.com/go-chi/chi/v5"
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

type fullMockStore struct {
	clinics     map[string]string // id -> name
	domains     map[string]domaindb.Domain
	settings    map[string]settingsdb.SiteSetting
	identities  map[string]authdb.PanelIdentity
	sessions    map[string]authdb.PanelSession
	memberships map[string]authdb.Membership
}

func newFullMockStore() *fullMockStore {
	return &fullMockStore{
		clinics:     make(map[string]string),
		domains:     make(map[string]domaindb.Domain),
		settings:    make(map[string]settingsdb.SiteSetting),
		identities:  make(map[string]authdb.PanelIdentity),
		sessions:    make(map[string]authdb.PanelSession),
		memberships: make(map[string]authdb.Membership),
	}
}

// Domain Querier
func (s *fullMockStore) GetDomainByHost(ctx context.Context, host string) (domaindb.Domain, error) {
	d, ok := s.domains[host]
	if !ok {
		return domaindb.Domain{}, pgx.ErrNoRows
	}
	return d, nil
}

// Settings Querier
func (s *fullMockStore) GetSiteSettings(ctx context.Context, tenantID pgtype.UUID) (settingsdb.GetSiteSettingsRow, error) {
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

func (s *fullMockStore) UpsertSiteSettings(ctx context.Context, arg settingsdb.UpsertSiteSettingsParams) (settingsdb.UpsertSiteSettingsRow, error) {
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

func (s *fullMockStore) GetSiteAppearance(ctx context.Context, tenantID pgtype.UUID) (settingsdb.GetSiteAppearanceRow, error) {
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

func (s *fullMockStore) UpsertSiteAppearance(ctx context.Context, arg settingsdb.UpsertSiteAppearanceParams) (settingsdb.UpsertSiteAppearanceRow, error) {
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

func (s *fullMockStore) UpsertSiteProfile(ctx context.Context, arg settingsdb.UpsertSiteProfileParams) (settingsdb.UpsertSiteProfileRow, error) {
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

func (s *fullMockStore) GetPublicSiteProfile(ctx context.Context, id pgtype.UUID) (settingsdb.GetPublicSiteProfileRow, error) {
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

func (s *fullMockStore) GetPrimaryVerifiedDomain(ctx context.Context, tenantID pgtype.UUID) (string, error) {
	tid := fromPgUUID(tenantID).String()
	for host, d := range s.domains {
		if fromPgUUID(d.TenantID).String() == tid && d.IsVerified {
			return host, nil
		}
	}
	return "", pgx.ErrNoRows
}

// Auth Querier
func (s *fullMockStore) GetPanelIdentityByEmail(ctx context.Context, email string) (authdb.PanelIdentity, error) {
	for _, id := range s.identities {
		if id.Email == email {
			return id, nil
		}
	}
	return authdb.PanelIdentity{}, pgx.ErrNoRows
}

func (s *fullMockStore) GetPanelIdentityByID(ctx context.Context, id pgtype.UUID) (authdb.PanelIdentity, error) {
	i, ok := s.identities[fromPgUUID(id).String()]
	if !ok {
		return authdb.PanelIdentity{}, pgx.ErrNoRows
	}
	return i, nil
}

func (s *fullMockStore) CreatePanelIdentity(ctx context.Context, arg authdb.CreatePanelIdentityParams) (authdb.PanelIdentity, error) {
	return authdb.PanelIdentity{}, nil
}

func (s *fullMockStore) CreatePanelSession(ctx context.Context, arg authdb.CreatePanelSessionParams) (authdb.PanelSession, error) {
	sess := authdb.PanelSession{
		ID:        arg.ID,
		UserID:    arg.UserID,
		TokenHash: arg.TokenHash,
		ExpiresAt: arg.ExpiresAt,
	}
	s.sessions[arg.TokenHash] = sess
	return sess, nil
}

func (s *fullMockStore) GetPanelSessionByTokenHash(ctx context.Context, tokenHash string) (authdb.PanelSession, error) {
	sess, ok := s.sessions[tokenHash]
	if !ok || sess.ExpiresAt.Time.Before(time.Now()) {
		return authdb.PanelSession{}, pgx.ErrNoRows
	}
	return sess, nil
}

func (s *fullMockStore) DeletePanelSession(ctx context.Context, tokenHash string) error {
	delete(s.sessions, tokenHash)
	return nil
}

func (s *fullMockStore) ListMembershipsByUserID(ctx context.Context, userID pgtype.UUID) ([]authdb.ListMembershipsByUserIDRow, error) {
	var rows []authdb.ListMembershipsByUserIDRow
	for _, m := range s.memberships {
		if fromPgUUID(m.UserID) == fromPgUUID(userID) {
			rows = append(rows, authdb.ListMembershipsByUserIDRow{
				ID:         m.ID,
				TenantID:   m.TenantID,
				UserID:     m.UserID,
				Role:       m.Role,
				TenantName: s.clinics[fromPgUUID(m.TenantID).String()],
				TenantSlug: "slug",
			})
		}
	}
	return rows, nil
}

func (s *fullMockStore) GetMembershipByUserAndTenant(ctx context.Context, arg authdb.GetMembershipByUserAndTenantParams) (authdb.Membership, error) {
	key := fromPgUUID(arg.UserID).String() + ":" + fromPgUUID(arg.TenantID).String()
	m, ok := s.memberships[key]
	if !ok {
		return authdb.Membership{}, pgx.ErrNoRows
	}
	return m, nil
}

func (s *fullMockStore) CreateMembership(ctx context.Context, arg authdb.CreateMembershipParams) (authdb.Membership, error) {
	return authdb.Membership{}, nil
}

// TestScenario_PSY_A01 verifies Acceptance Criteria PSY-A01:
// - Tenant A and B on two distinct hosts return strictly isolated data
// - Invalid or unverified hosts are safely rejected
// - Forged tenant IDs/headers cannot read or modify data of another tenant
func TestScenario_PSY_A01(t *testing.T) {
	store := newFullMockStore()

	// Fixture A: Clinic A (Dr. Rezaei)
	tenantAID := uuid.New()
	ownerAID := uuid.New()
	hashA, _ := auth.HashPassword("rezaeiPass123")
	store.clinics[tenantAID.String()] = "مطب دکتر رضایی"
	store.domains["rezaei.ir"] = domaindb.Domain{
		Host:       "rezaei.ir",
		TenantID:   toPgUUID(tenantAID),
		IsVerified: true,
	}
	store.settings[tenantAID.String()] = settingsdb.SiteSetting{
		TenantID:     toPgUUID(tenantAID),
		Title:        "دکتر رضایی - روان‌درمانگر",
		Description:  "درمان شناختی رفتاری",
		Phone:        "02111111111",
		PrimaryColor: "#0284c7",
	}
	store.identities[ownerAID.String()] = authdb.PanelIdentity{
		ID:           toPgUUID(ownerAID),
		Email:        "owner@rezaei.ir",
		PasswordHash: hashA,
		FullName:     "دکتر علی رضایی",
		Status:       "active",
	}
	store.memberships[ownerAID.String()+":"+tenantAID.String()] = authdb.Membership{
		ID:       toPgUUID(uuid.New()),
		TenantID: toPgUUID(tenantAID),
		UserID:   toPgUUID(ownerAID),
		Role:     "owner",
	}

	// Fixture B: Clinic B (Niloofar Clinic)
	tenantBID := uuid.New()
	ownerBID := uuid.New()
	hashB, _ := auth.HashPassword("niloofarPass123")
	store.clinics[tenantBID.String()] = "مرکز مشاوره نیلوفر"
	store.domains["niloofar.ir"] = domaindb.Domain{
		Host:       "niloofar.ir",
		TenantID:   toPgUUID(tenantBID),
		IsVerified: true,
	}
	store.settings[tenantBID.String()] = settingsdb.SiteSetting{
		TenantID:     toPgUUID(tenantBID),
		Title:        "مرکز مشاوره و سلامت روان نیلوفر",
		Description:  "روان‌شناسی خانواده و کودک",
		Phone:        "02122222222",
		PrimaryColor: "#10b981",
	}
	store.identities[ownerBID.String()] = authdb.PanelIdentity{
		ID:           toPgUUID(ownerBID),
		Email:        "owner@niloofar.ir",
		PasswordHash: hashB,
		FullName:     "دکتر نیلوفر سپهری",
		Status:       "active",
	}
	store.memberships[ownerBID.String()+":"+tenantBID.String()] = authdb.Membership{
		ID:       toPgUUID(uuid.New()),
		TenantID: toPgUUID(tenantBID),
		UserID:   toPgUUID(ownerBID),
		Role:     "owner",
	}

	// Fixture C: Unverified Domain
	store.domains["pending-clinic.ir"] = domaindb.Domain{
		Host:       "pending-clinic.ir",
		TenantID:   toPgUUID(uuid.New()),
		IsVerified: false,
	}

	// Assemble router
	domainResolver := domain.NewResolver(store)
	authService := auth.NewService(store)
	settingsService := settings.NewService(store)

	authHandler := auth.NewHandler(authService)
	settingsHandler := settings.NewHandler(settingsService, domainResolver, authService)

	r := chi.NewRouter()
	r.Route("/v1", func(v1 chi.Router) {
		authHandler.MountRoutes(v1)
		settingsHandler.MountRoutes(v1)
	})

	// 1. Host A Public Site
	reqA := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqA.Host = "rezaei.ir"
	rrA := httptest.NewRecorder()
	r.ServeHTTP(rrA, reqA)
	if rrA.Code != http.StatusOK {
		t.Fatalf("expected 200 for Host A, got %d", rrA.Code)
	}
	var resA settings.PublicSiteProfile
	_ = json.Unmarshal(rrA.Body.Bytes(), &resA)
	if resA.Name != "مطب دکتر رضایی" || resA.PrimaryColor != "#0284c7" {
		t.Fatalf("unexpected profile for Host A: %v", resA)
	}

	// 2. Host B Public Site
	reqB := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqB.Host = "niloofar.ir"
	rrB := httptest.NewRecorder()
	r.ServeHTTP(rrB, reqB)
	if rrB.Code != http.StatusOK {
		t.Fatalf("expected 200 for Host B, got %d", rrB.Code)
	}
	var resB settings.PublicSiteProfile
	_ = json.Unmarshal(rrB.Body.Bytes(), &resB)
	if resB.Name != "مرکز مشاوره نیلوفر" || resB.PrimaryColor != "#10b981" {
		t.Fatalf("unexpected profile for Host B: %v", resB)
	}

	// 3. Unverified Host -> 404
	reqC := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqC.Host = "pending-clinic.ir"
	rrC := httptest.NewRecorder()
	r.ServeHTTP(rrC, reqC)
	if rrC.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unverified host, got %d", rrC.Code)
	}

	// 4. Non-existent Host -> 404
	reqUnknown := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqUnknown.Host = "doesnotexist.ir"
	rrUnknown := httptest.NewRecorder()
	r.ServeHTTP(rrUnknown, reqUnknown)
	if rrUnknown.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown host, got %d", rrUnknown.Code)
	}

	// 5. Authenticate Owner A
	sessA, err := authService.Login(context.Background(), "owner@rezaei.ir", "rezaeiPass123")
	if err != nil {
		t.Fatalf("failed to login owner A: %v", err)
	}

	// 6. Cross-Tenant Attempt: Owner A tries to mutate Tenant B settings -> Blocked with 403 Forbidden
	tamperBody, _ := json.Marshal(map[string]string{
		"title": "کلینیک دستکاری‌شده",
	})
	reqTamper := httptest.NewRequest(http.MethodPut, "/v1/admin/site", bytes.NewBuffer(tamperBody))
	reqTamper.Header.Set("Authorization", "Bearer "+sessA.Token)
	reqTamper.Header.Set("X-Tenant-ID", tenantBID.String()) // Forged tenant ID
	reqTamper.Header.Set("Content-Type", "application/json")
	rrTamper := httptest.NewRecorder()
	r.ServeHTTP(rrTamper, reqTamper)

	if rrTamper.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for cross-tenant tampering attempt, got %d", rrTamper.Code)
	}

	// Verify Tenant B settings were NOT modified
	stB, _ := settingsService.GetSettings(context.Background(), tenantBID)
	if stB.Title != "مرکز مشاوره و سلامت روان نیلوفر" {
		t.Fatalf("Tenant B settings were corrupted! Got: %s", stB.Title)
	}

	// 7. Legitimate update on Tenant A by Owner A -> 200 OK
	legitBody, _ := json.Marshal(map[string]string{
		"title": "مطب تخصصی دکتر علی رضایی (به‌روزرسانی)",
	})
	reqLegit := httptest.NewRequest(http.MethodPut, "/v1/admin/site", bytes.NewBuffer(legitBody))
	reqLegit.Header.Set("Authorization", "Bearer "+sessA.Token)
	reqLegit.Header.Set("X-Tenant-ID", tenantAID.String())
	reqLegit.Header.Set("Content-Type", "application/json")
	rrLegit := httptest.NewRecorder()
	r.ServeHTTP(rrLegit, reqLegit)

	if rrLegit.Code != http.StatusOK {
		t.Fatalf("expected 200 for legitimate update on Tenant A, got %d", rrLegit.Code)
	}

	stA, _ := settingsService.GetSettings(context.Background(), tenantAID)
	if stA.Title != "مطب تخصصی دکتر علی رضایی (به‌روزرسانی)" {
		t.Fatalf("Tenant A update failed to persist! Got: %s", stA.Title)
	}
}
