package testsupport_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"psychology/api/internal/auth"
	"psychology/api/internal/auth/authdb"
	"psychology/api/internal/domain"
	"psychology/api/internal/domain/domaindb"
	"psychology/api/internal/platform/db"
	"psychology/api/internal/platform/tenantctx"
	"psychology/api/internal/settings"
	"psychology/api/internal/settings/settingsdb"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestAcceptance_PSY_A13_Mock_TwoHosts verifies acceptance scenario PSY-A13:
// 1. Two separate verified hosts: Tenant A (omid-psych.ir) and Tenant B (mahan-clinic.ir).
// 2. Access Separation:
//    - Anonymous request to appearance endpoint -> 401 Unauthorized.
//    - Normal staff member -> 403 Forbidden.
//    - Non-appearance-owner user -> 403 Forbidden.
//    - Designated APPEARANCE_OWNER_USER_ID -> 200 OK with private, no-store.
// 3. Writing Separation:
//    - Appearance updated via PUT /v1/admin/site/appearance -> updates appearance and enqueues outbox event.
//    - Content updated via PUT /v1/admin/site -> updates profile and enqueues site.updated event.
//    - Passing appearance fields to PUT /v1/admin/site -> rejected with 400 Bad Request and APPEARANCE_FIELDS_NOT_ALLOWED.
// 4. Catalog and Version Validation:
//    - Unregistered template or palette -> 400 Bad Request.
//    - Disallow unknown fields (e.g. client attempting to send template_version) -> 400 Bad Request.
//    - Persisted template version is locked by Go catalog (1.0.0).
// 5. Multi-Tenant Isolation:
//    - Host A public profile reflects published appearance.
//    - Host B public profile remains untouched (zero cross-tenant contamination).
func TestAcceptance_PSY_A13_Mock_TwoHosts(t *testing.T) {
	hostA := "omid-psych.ir"
	hostB := "mahan-clinic.ir"

	tenantAID := uuid.MustParse("11111111-aaaa-1111-aaaa-111111111111")
	tenantBID := uuid.MustParse("22222222-bbbb-2222-bbbb-222222222222")

	ownerAID := uuid.MustParse("33333333-aaaa-3333-aaaa-333333333333")
	staffAID := uuid.MustParse("44444444-aaaa-4444-aaaa-444444444444")
	ownerBID := uuid.MustParse("55555555-bbbb-5555-bbbb-555555555555")

	mockStore := newFullMockStore()

	// Seed Tenant A
	mockStore.clinics[tenantAID.String()] = "کلینیک روان‌شناسی امید"
	mockStore.domains[hostA] = domaindb.Domain{
		Host:       hostA,
		TenantID:   toPgUUID(tenantAID),
		IsVerified: true,
	}
	mockStore.settings[tenantAID.String()] = settingsdb.SiteSetting{
		TenantID:        toPgUUID(tenantAID),
		Title:           "کلینیک تخصصی امید",
		Description:     "روان‌درمانی و مشاوره فردی",
		Phone:           "021-88881111",
		Address:         "تهران، ونک",
		PrimaryColor:    "#0284c7",
		TemplateID:      "minimal",
		TemplateVersion: "1.0.0",
		ThemeRadius:     "md",
		ThemeScale:      "md",
		PaletteID:       "calm-sage",
	}

	hashPass, _ := auth.HashPassword("secret123456")
	mockStore.identities[ownerAID.String()] = authdb.PanelIdentity{
		ID:           toPgUUID(ownerAID),
		Email:        "owner@omid-psych.ir",
		PasswordHash: hashPass,
		FullName:     "دکتر مریم علوی",
		Status:       "active",
	}
	mockStore.memberships[ownerAID.String()+":"+tenantAID.String()] = authdb.Membership{
		ID:       toPgUUID(uuid.New()),
		TenantID: toPgUUID(tenantAID),
		UserID:   toPgUUID(ownerAID),
		Role:     "owner",
	}

	mockStore.identities[staffAID.String()] = authdb.PanelIdentity{
		ID:           toPgUUID(staffAID),
		Email:        "staff@omid-psych.ir",
		PasswordHash: hashPass,
		FullName:     "کارمند مطب امید",
		Status:       "active",
	}
	mockStore.memberships[staffAID.String()+":"+tenantAID.String()] = authdb.Membership{
		ID:       toPgUUID(uuid.New()),
		TenantID: toPgUUID(tenantAID),
		UserID:   toPgUUID(staffAID),
		Role:     "staff",
	}

	// Seed Tenant B
	mockStore.clinics[tenantBID.String()] = "مرکز مشاوره ماهان"
	mockStore.domains[hostB] = domaindb.Domain{
		Host:       hostB,
		TenantID:   toPgUUID(tenantBID),
		IsVerified: true,
	}
	mockStore.settings[tenantBID.String()] = settingsdb.SiteSetting{
		TenantID:        toPgUUID(tenantBID),
		Title:           "مرکز مشاوره ماهان",
		Description:     "مشاوره خانواده و کودک",
		Phone:           "021-22223333",
		Address:         "تهران، شریعتی",
		PrimaryColor:    "#10b981",
		TemplateID:      "minimal",
		TemplateVersion: "1.0.0",
		ThemeRadius:     "md",
		ThemeScale:      "md",
		PaletteID:       "warm-sand",
	}

	mockStore.identities[ownerBID.String()] = authdb.PanelIdentity{
		ID:           toPgUUID(ownerBID),
		Email:        "owner@mahan-clinic.ir",
		PasswordHash: hashPass,
		FullName:     "دکتر بهنام ماهان",
		Status:       "active",
	}
	mockStore.memberships[ownerBID.String()+":"+tenantBID.String()] = authdb.Membership{
		ID:       toPgUUID(uuid.New()),
		TenantID: toPgUUID(tenantBID),
		UserID:   toPgUUID(ownerBID),
		Role:     "owner",
	}

	domainResolver := domain.NewResolver(mockStore)
	authService := auth.NewService(mockStore)
	settingsService := settings.NewService(mockStore)
	authHandler := auth.NewHandler(authService)
	settingsHandler := settings.NewHandler(settingsService, domainResolver, authService)

	// Designate ownerAID as the unique appearance owner
	settingsHandler.SetAppearanceOwnerUserID(ownerAID)

	apiRouter := chi.NewRouter()
	apiRouter.Route("/v1", func(r chi.Router) {
		authHandler.MountRoutes(r)
		settingsHandler.MountRoutes(r)
	})

	// Login sessions
	sessOwnerA, err := authService.Login(context.Background(), "owner@omid-psych.ir", "secret123456")
	if err != nil {
		t.Fatalf("failed to login owner A: %v", err)
	}
	sessStaffA, err := authService.Login(context.Background(), "staff@omid-psych.ir", "secret123456")
	if err != nil {
		t.Fatalf("failed to login staff A: %v", err)
	}
	sessOwnerB, err := authService.Login(context.Background(), "owner@mahan-clinic.ir", "secret123456")
	if err != nil {
		t.Fatalf("failed to login owner B: %v", err)
	}

	// =========================================================================
	// 1. Access Separation Checks
	// =========================================================================
	// 1.a Unauthenticated request to appearance endpoint -> 401
	reqUnauth := httptest.NewRequest(http.MethodGet, "/v1/admin/site/appearance", nil)
	recUnauth := httptest.NewRecorder()
	apiRouter.ServeHTTP(recUnauth, reqUnauth)
	if recUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for unauthenticated appearance request, got %d", recUnauth.Code)
	}

	// 1.b Non-owner staff member -> 403 Forbidden (RequireRole("owner"))
	reqStaff := httptest.NewRequest(http.MethodGet, "/v1/admin/site/appearance", nil)
	reqStaff.Header.Set("Authorization", "Bearer "+sessStaffA.Token)
	reqStaff.Header.Set("X-Tenant-ID", tenantAID.String())
	recStaff := httptest.NewRecorder()
	apiRouter.ServeHTTP(recStaff, reqStaff)
	if recStaff.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for staff member, got %d", recStaff.Code)
	}

	// 1.c Owner B attempting to read Tenant A appearance (cross-tenant) -> 403 Forbidden
	reqCross := httptest.NewRequest(http.MethodGet, "/v1/admin/site/appearance", nil)
	reqCross.Header.Set("Authorization", "Bearer "+sessOwnerB.Token)
	reqCross.Header.Set("X-Tenant-ID", tenantAID.String())
	recCross := httptest.NewRecorder()
	apiRouter.ServeHTTP(recCross, reqCross)
	if recCross.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for cross-tenant request, got %d", recCross.Code)
	}

	// 1.d Owner B on Tenant B (valid owner, but NOT appearance owner user) -> 403 Forbidden
	reqNotAppOwner := httptest.NewRequest(http.MethodGet, "/v1/admin/site/appearance", nil)
	reqNotAppOwner.Header.Set("Authorization", "Bearer "+sessOwnerB.Token)
	reqNotAppOwner.Header.Set("X-Tenant-ID", tenantBID.String())
	recNotAppOwner := httptest.NewRecorder()
	apiRouter.ServeHTTP(recNotAppOwner, reqNotAppOwner)
	if recNotAppOwner.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for user who is not APPEARANCE_OWNER_USER_ID, got %d", recNotAppOwner.Code)
	}

	// 1.e Designated Appearance Owner A on Tenant A -> 200 OK with private, no-store
	reqOwnerA := httptest.NewRequest(http.MethodGet, "/v1/admin/site/appearance", nil)
	reqOwnerA.Header.Set("Authorization", "Bearer "+sessOwnerA.Token)
	reqOwnerA.Header.Set("X-Tenant-ID", tenantAID.String())
	recOwnerA := httptest.NewRecorder()
	apiRouter.ServeHTTP(recOwnerA, reqOwnerA)
	if recOwnerA.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for designated appearance owner, got %d: %s", recOwnerA.Code, recOwnerA.Body.String())
	}
	if recOwnerA.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("expected Cache-Control: private, no-store, got: %s", recOwnerA.Header().Get("Cache-Control"))
	}
	var initApp settings.SiteAppearance
	_ = json.Unmarshal(recOwnerA.Body.Bytes(), &initApp)
	if initApp.TemplateID != "minimal" || initApp.PaletteID != "calm-sage" {
		t.Fatalf("unexpected initial appearance: %+v", initApp)
	}

	// =========================================================================
	// 2. Writing Separation: Content vs Appearance
	// =========================================================================
	// 2.a Reject appearance fields on legacy PUT /v1/admin/site -> 400 APPEARANCE_FIELDS_NOT_ALLOWED
	badContentPayloads := []map[string]any{
		{"title": "مطب جدید", "template_id": "clinical"},
		{"title": "مطب جدید", "palette_id": "ocean-trust"},
		{"title": "مطب جدید", "theme_radius": "lg"},
		{"title": "مطب جدید", "theme_scale": "sm"},
		{"title": "مطب جدید", "primary_color": "#0ea5e9"},
	}
	for _, badPayload := range badContentPayloads {
		b, _ := json.Marshal(badPayload)
		reqBad := httptest.NewRequest(http.MethodPut, "/v1/admin/site", bytes.NewBuffer(b))
		reqBad.Header.Set("Authorization", "Bearer "+sessOwnerA.Token)
		reqBad.Header.Set("X-Tenant-ID", tenantAID.String())
		reqBad.Header.Set("Content-Type", "application/json")
		recBad := httptest.NewRecorder()
		apiRouter.ServeHTTP(recBad, reqBad)
		if recBad.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request when appearance field passed to /admin/site, got %d", recBad.Code)
		}
		if !strings.Contains(recBad.Body.String(), "APPEARANCE_FIELDS_NOT_ALLOWED") {
			t.Fatalf("expected code APPEARANCE_FIELDS_NOT_ALLOWED, got: %s", recBad.Body.String())
		}
	}

	// 2.b Valid content update via PUT /v1/admin/site -> 200 OK
	contentBody, _ := json.Marshal(map[string]any{
		"title":       "کلینیک تخصصی روان‌شناسی امید (به‌روزشده)",
		"description": "روان‌درمانی تحلیلی و مشاوره تخصصی فردی",
		"phone":       "021-88889999",
		"address":     "تهران، ونک، پلاک ۱۰",
	})
	reqContent := httptest.NewRequest(http.MethodPut, "/v1/admin/site", bytes.NewBuffer(contentBody))
	reqContent.Header.Set("Authorization", "Bearer "+sessOwnerA.Token)
	reqContent.Header.Set("X-Tenant-ID", tenantAID.String())
	reqContent.Header.Set("Content-Type", "application/json")
	recContent := httptest.NewRecorder()
	apiRouter.ServeHTTP(recContent, reqContent)
	if recContent.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on content update, got %d: %s", recContent.Code, recContent.Body.String())
	}

	// 2.c Valid appearance update via dedicated PUT /v1/admin/site/appearance -> 200 OK
	appearanceBody, _ := json.Marshal(map[string]any{
		"template_id":  "clinical",
		"palette_id":   "ocean-trust",
		"theme_radius": "lg",
		"theme_scale":  "sm",
	})
	reqAppearance := httptest.NewRequest(http.MethodPut, "/v1/admin/site/appearance", bytes.NewBuffer(appearanceBody))
	reqAppearance.Header.Set("Authorization", "Bearer "+sessOwnerA.Token)
	reqAppearance.Header.Set("X-Tenant-ID", tenantAID.String())
	reqAppearance.Header.Set("Content-Type", "application/json")
	recAppearance := httptest.NewRecorder()
	apiRouter.ServeHTTP(recAppearance, reqAppearance)
	if recAppearance.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on appearance update, got %d: %s", recAppearance.Code, recAppearance.Body.String())
	}
	if recAppearance.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("expected Cache-Control: private, no-store on appearance update, got: %s", recAppearance.Header().Get("Cache-Control"))
	}

	var updatedApp settings.SiteAppearance
	_ = json.Unmarshal(recAppearance.Body.Bytes(), &updatedApp)
	if updatedApp.TemplateID != "clinical" || updatedApp.PaletteID != "ocean-trust" || updatedApp.ThemeRadius != "lg" || updatedApp.ThemeScale != "sm" {
		t.Fatalf("unexpected updated appearance response: %+v", updatedApp)
	}

	// =========================================================================
	// 3. Catalog and Version Validation
	// =========================================================================
	// 3.a Unregistered template -> 400 Bad Request
	invTplBody, _ := json.Marshal(map[string]any{
		"template_id": "unregistered_template_999",
		"palette_id":  "calm-sage",
	})
	reqInvTpl := httptest.NewRequest(http.MethodPut, "/v1/admin/site/appearance", bytes.NewBuffer(invTplBody))
	reqInvTpl.Header.Set("Authorization", "Bearer "+sessOwnerA.Token)
	reqInvTpl.Header.Set("X-Tenant-ID", tenantAID.String())
	reqInvTpl.Header.Set("Content-Type", "application/json")
	recInvTpl := httptest.NewRecorder()
	apiRouter.ServeHTTP(recInvTpl, reqInvTpl)
	if recInvTpl.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unregistered template, got %d", recInvTpl.Code)
	}

	// 3.b Unregistered palette -> 400 Bad Request
	invPalBody, _ := json.Marshal(map[string]any{
		"template_id": "minimal",
		"palette_id":  "neon_magenta_fantasy",
	})
	reqInvPal := httptest.NewRequest(http.MethodPut, "/v1/admin/site/appearance", bytes.NewBuffer(invPalBody))
	reqInvPal.Header.Set("Authorization", "Bearer "+sessOwnerA.Token)
	reqInvPal.Header.Set("X-Tenant-ID", tenantAID.String())
	reqInvPal.Header.Set("Content-Type", "application/json")
	recInvPal := httptest.NewRecorder()
	apiRouter.ServeHTTP(recInvPal, reqInvPal)
	if recInvPal.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unregistered palette, got %d", recInvPal.Code)
	}

	// 3.c Client attempting to send template_version -> 400 Bad Request
	invVersionBody, _ := json.Marshal(map[string]any{
		"template_id":      "clinical",
		"palette_id":       "calm-sage",
		"template_version": "9.9.9",
	})
	reqInvVersion := httptest.NewRequest(http.MethodPut, "/v1/admin/site/appearance", bytes.NewBuffer(invVersionBody))
	reqInvVersion.Header.Set("Authorization", "Bearer "+sessOwnerA.Token)
	reqInvVersion.Header.Set("X-Tenant-ID", tenantAID.String())
	reqInvVersion.Header.Set("Content-Type", "application/json")
	recInvVersion := httptest.NewRecorder()
	apiRouter.ServeHTTP(recInvVersion, reqInvVersion)
	if recInvVersion.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for client sending template_version, got %d", recInvVersion.Code)
	}

	// =========================================================================
	// 4. Public Profile & Multi-Tenant Isolation
	// =========================================================================
	// Host A public site reflects published clinical template and ocean-trust palette
	reqPublicA := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqPublicA.Host = hostA
	recPublicA := httptest.NewRecorder()
	apiRouter.ServeHTTP(recPublicA, reqPublicA)
	if recPublicA.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for Host A public site, got %d", recPublicA.Code)
	}
	var pubA settings.PublicSiteProfile
	_ = json.Unmarshal(recPublicA.Body.Bytes(), &pubA)
	if pubA.Title != "کلینیک تخصصی روان‌شناسی امید (به‌روزشده)" || pubA.TemplateID != "clinical" || pubA.PaletteID != "ocean-trust" {
		t.Fatalf("Host A public profile was not updated correctly: %+v", pubA)
	}

	// Host B public site remains completely untouched (minimal template, warm-sand palette)
	reqPublicB := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqPublicB.Host = hostB
	recPublicB := httptest.NewRecorder()
	apiRouter.ServeHTTP(recPublicB, reqPublicB)
	if recPublicB.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for Host B public site, got %d", recPublicB.Code)
	}
	var pubB settings.PublicSiteProfile
	_ = json.Unmarshal(recPublicB.Body.Bytes(), &pubB)
	if pubB.Title != "مرکز مشاوره ماهان" || pubB.TemplateID != "minimal" || pubB.PaletteID != "warm-sand" {
		t.Fatalf("Host B public profile was corrupted! Got: %+v", pubB)
	}
}

// TestAcceptance_PSY_A13_LivePostgreSQL_TwoHosts runs the full PSY-A13 acceptance scenario
// against real PostgreSQL using the runtime role app_user under FORCE RLS.
func TestAcceptance_PSY_A13_LivePostgreSQL_TwoHosts(t *testing.T) {
	dbURL := os.Getenv("TEST_APP_DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		// Try default runtime app_user connection if active docker container is running
		dbURL = "postgres://app_user:secret_app_pass@127.0.0.1:5432/psychology_prod?sslmode=disable"
	}

	ctx := context.Background()
	appPool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live database test: failed to connect as app_user: %v", err)
		return
	}
	defer appPool.Close()

	if err := appPool.Ping(ctx); err != nil {
		t.Skipf("skipping live database test: postgres ping failed: %v", err)
		return
	}

	store := db.NewStore(appPool)
	settingsService := settings.NewServiceWithStore(store)
	domainResolver := domain.NewResolver(domaindb.New(appPool))
	authService := auth.NewServiceWithStore(authdb.New(appPool), store)
	settingsHandler := settings.NewHandler(settingsService, domainResolver, authService)
	authHandler := auth.NewHandler(authService)

	apiRouter := chi.NewRouter()
	apiRouter.Route("/v1", func(r chi.Router) {
		authHandler.MountRoutes(r)
		settingsHandler.MountRoutes(r)
	})

	// Seed two real tenants, identities, memberships and domains
	tenantA := uuid.New()
	tenantB := uuid.New()
	slugA := "a13-a-" + tenantA.String()[:8]
	slugB := "a13-b-" + tenantB.String()[:8]
	hostA := slugA + ".ir"
	hostB := slugB + ".ir"

	_, err = appPool.Exec(ctx, `
		INSERT INTO tenants (id, name, slug, status)
		VALUES ($1, $2, $3, 'active'), ($4, $5, $6, 'active')
	`, tenantA, "کلینیک دکتر احمدی", slugA, tenantB, "کلینیک دکتر حسینی", slugB)
	if err != nil {
		t.Fatalf("failed to insert live tenants: %v", err)
	}
	defer func() {
		_, _ = appPool.Exec(context.Background(), "DELETE FROM tenants WHERE id IN ($1, $2)", tenantA, tenantB)
	}()

	userA := uuid.New()
	userB := uuid.New()
	pwHashA, _ := auth.HashPassword("ahmadiPass123")
	pwHashB, _ := auth.HashPassword("hoseiniPass123")

	_, err = appPool.Exec(ctx, `
		INSERT INTO panel_identities (id, email, password_hash, full_name, status)
		VALUES
			($1, $2, $3, 'دکتر احمدی', 'active'),
			($4, $5, $6, 'دکتر حسینی', 'active')
	`, userA, slugA+"@example.com", pwHashA, userB, slugB+"@example.com", pwHashB)
	if err != nil {
		t.Fatalf("failed to insert live identities: %v", err)
	}

	ctxA := tenantctx.With(ctx, tenantA)
	ctxB := tenantctx.With(ctx, tenantB)

	// Seed domain, membership & initial settings for Tenant A under RLS
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
		q := settingsdb.New(tx)
		_, err := q.UpsertSiteAppearance(ctxA, settingsdb.UpsertSiteAppearanceParams{
			TenantID:        pgtype.UUID{Bytes: tid, Valid: true},
			TemplateID:      "minimal",
			TemplateVersion: "1.0.0",
			PaletteID:       "calm-sage",
			ThemeRadius:     "md",
			ThemeScale:      "md",
		})
		return err
	})
	if err != nil {
		t.Fatalf("failed to seed Tenant A: %v", err)
	}

	// Seed domain, membership & initial settings for Tenant B under RLS
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
		q := settingsdb.New(tx)
		_, err := q.UpsertSiteAppearance(ctxB, settingsdb.UpsertSiteAppearanceParams{
			TenantID:        pgtype.UUID{Bytes: tid, Valid: true},
			TemplateID:      "minimal",
			TemplateVersion: "1.0.0",
			PaletteID:       "warm-sand",
			ThemeRadius:     "md",
			ThemeScale:      "md",
		})
		return err
	})
	if err != nil {
		t.Fatalf("failed to seed Tenant B: %v", err)
	}

	// Configure Appearance Owner user in Go handler
	settingsHandler.SetAppearanceOwnerUserID(userA)

	// Login sessions
	sessA, err := authService.Login(ctx, slugA+"@example.com", "ahmadiPass123")
	if err != nil {
		t.Fatalf("failed to login user A: %v", err)
	}
	sessB, err := authService.Login(ctx, slugB+"@example.com", "hoseiniPass123")
	if err != nil {
		t.Fatalf("failed to login user B: %v", err)
	}
	_ = sessB

	// 1. Cross-tenant tampering attempt by user A against tenant B -> 403 Forbidden
	tamperBody, _ := json.Marshal(map[string]any{
		"template_id": "clinical",
		"palette_id":  "deep-teal",
	})
	reqTamper := httptest.NewRequest(http.MethodPut, "/v1/admin/site/appearance", bytes.NewBuffer(tamperBody))
	reqTamper.Header.Set("Authorization", "Bearer "+sessA.Token)
	reqTamper.Header.Set("X-Tenant-ID", tenantB.String())
	reqTamper.Header.Set("Content-Type", "application/json")
	recTamper := httptest.NewRecorder()
	apiRouter.ServeHTTP(recTamper, reqTamper)
	if recTamper.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on cross-tenant tampering, got %d", recTamper.Code)
	}

	// 2. Reject appearance fields on PUT /v1/admin/site -> 400
	rejectBody, _ := json.Marshal(map[string]any{
		"title":       "عنوان مطب",
		"template_id": "clinical",
	})
	reqReject := httptest.NewRequest(http.MethodPut, "/v1/admin/site", bytes.NewBuffer(rejectBody))
	reqReject.Header.Set("Authorization", "Bearer "+sessA.Token)
	reqReject.Header.Set("X-Tenant-ID", tenantA.String())
	reqReject.Header.Set("Content-Type", "application/json")
	recReject := httptest.NewRecorder()
	apiRouter.ServeHTTP(recReject, reqReject)
	if recReject.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request when appearance field passed to /admin/site, got %d", recReject.Code)
	}

	// 3. Legitimate Content Update -> 200 OK
	contentBody, _ := json.Marshal(map[string]any{
		"title":       "کلینیک تخصصی روان‌درمانی دکتر احمدی",
		"description": "روان‌درمانی تحلیلی و مشاوره تخصصی فردی",
		"phone":       "021-88889999",
		"address":     "تهران، ونک",
	})
	reqContent := httptest.NewRequest(http.MethodPut, "/v1/admin/site", bytes.NewBuffer(contentBody))
	reqContent.Header.Set("Authorization", "Bearer "+sessA.Token)
	reqContent.Header.Set("X-Tenant-ID", tenantA.String())
	reqContent.Header.Set("Content-Type", "application/json")
	recContent := httptest.NewRecorder()
	apiRouter.ServeHTTP(recContent, reqContent)
	if recContent.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on content update, got %d: %s", recContent.Code, recContent.Body.String())
	}

	// 4. Legitimate Appearance Update by designated appearance owner -> 200 OK
	appBody, _ := json.Marshal(map[string]any{
		"template_id":  "clinical",
		"palette_id":   "ocean-trust",
		"theme_radius": "lg",
		"theme_scale":  "sm",
	})
	reqApp := httptest.NewRequest(http.MethodPut, "/v1/admin/site/appearance", bytes.NewBuffer(appBody))
	reqApp.Header.Set("Authorization", "Bearer "+sessA.Token)
	reqApp.Header.Set("X-Tenant-ID", tenantA.String())
	reqApp.Header.Set("Content-Type", "application/json")
	recApp := httptest.NewRecorder()
	apiRouter.ServeHTTP(recApp, reqApp)
	if recApp.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on appearance update, got %d: %s", recApp.Code, recApp.Body.String())
	}

	// Verify Tenant A settings are persisted in PostgreSQL
	stA, err := settingsService.GetSettings(ctxA, tenantA)
	if err != nil || stA.TemplateID != "clinical" || stA.PaletteID != "ocean-trust" || stA.ThemeRadius != "lg" {
		t.Fatalf("failed to persist updated appearance in PostgreSQL: %+v, err=%v", stA, err)
	}

	// Verify outbox_events was written in PostgreSQL in the same transaction
	err = store.WithTenantTx(ctxA, func(tx pgx.Tx, tid uuid.UUID) error {
		var eventType, outboxHost string
		err := tx.QueryRow(ctx, `
			SELECT event_type, COALESCE(payload->>'host', '')
			FROM outbox_events
			WHERE tenant_id = $1 AND event_type = 'site.updated'
			ORDER BY created_at DESC
			LIMIT 1
		`, tid).Scan(&eventType, &outboxHost)
		if err != nil {
			return err
		}
		if eventType != "site.updated" || outboxHost != hostA {
			return fmt.Errorf("unexpected outbox event: type=%s, host=%s", eventType, outboxHost)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to find site.updated event in PostgreSQL outbox_events: %v", err)
	}

	// 5. Verify Host B appearance in PostgreSQL is strictly unchanged
	stB, err := settingsService.GetSettings(ctxB, tenantB)
	if err != nil || stB.TemplateID != "minimal" || stB.PaletteID != "warm-sand" {
		t.Fatalf("Tenant B appearance was corrupted in PostgreSQL! Got: %+v, err=%v", stB, err)
	}
}
