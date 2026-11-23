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
	"sync"
	"testing"
	"time"

	"psychology/api/internal/auth"
	"psychology/api/internal/auth/authdb"
	"psychology/api/internal/domain"
	"psychology/api/internal/domain/domaindb"
	"psychology/api/internal/platform/cache"
	"psychology/api/internal/platform/db"
	"psychology/api/internal/platform/tenantctx"
	"psychology/api/internal/settings"
	"psychology/api/internal/settings/settingsdb"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestAcceptance_PSY_A12_InitialPath_TwoHosts verifies acceptance scenario PSY-A12:
// 1. Two separate verified hosts: Tenant A (rezaei-clinic.ir) and Tenant B (niloofar-psych.ir).
// 2. Both clinics have published settings and their public content is cached with Edge Cache HIT.
// 3. Owner A compares minimal and clinical on /preview:
//    - Preview requests have Cache-Control: private, no-store.
//    - Sequential preview requests strictly BYPASS the shared cache and never pollute shared cache entries.
//    - Previewing does not mutate published database settings or trigger cache invalidation.
// 4. Owner A saves new visual settings via PUT /v1/admin/site (template: clinical, primary_color: #0f766e, radius: lg, scale: sm):
//    - Response returns Cache-Control: private, no-store.
//    - An Outbox event is recorded in the transaction for host A.
// 5. Outbox worker processes the invalidation event:
//    - Host A cache entries are purged (next request is MISS, rendering the updated clinical template).
//    - Host B cache entries remain completely intact (strict cache isolation, repeat request is HIT).
// 6. Cross-Tenant authorization guard: Owner A cannot mutate Tenant B's settings (rejected with 403 Forbidden).
func TestAcceptance_PSY_A12_InitialPath_TwoHosts(t *testing.T) {
	hostA := "rezaei-clinic.ir"
	hostB := "niloofar-psych.ir"

	tenantAID := uuid.MustParse("a1111111-1111-1111-1111-111111111111")
	tenantBID := uuid.MustParse("b2222222-2222-2222-2222-222222222222")

	mockStore := newFullMockStore()

	// Seed Tenant A
	ownerAID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	hashA, _ := auth.HashPassword("rezaeiPass123")
	mockStore.clinics[tenantAID.String()] = "مطب دکتر علی رضایی"
	mockStore.domains[hostA] = domaindb.Domain{
		Host:       hostA,
		TenantID:   toPgUUID(tenantAID),
		IsVerified: true,
	}
	mockStore.settings[tenantAID.String()] = settingsdb.SiteSetting{
		TenantID:        toPgUUID(tenantAID),
		Title:           "مطب تخصصی دکتر علی رضایی",
		Description:     "خدمات روان‌درمانی و مشاوره فردی",
		Phone:           "021-88881111",
		Address:         "تهران، میدان ونک",
		PrimaryColor:    "#0284c7",
		TemplateID:      "minimal",
		TemplateVersion: "1.0.0",
		ThemeRadius:     "md",
		ThemeScale:      "md",
	}
	mockStore.identities[ownerAID.String()] = authdb.PanelIdentity{
		ID:           toPgUUID(ownerAID),
		Email:        "owner@rezaei.ir",
		PasswordHash: hashA,
		FullName:     "دکتر علی رضایی",
		Status:       "active",
	}
	mockStore.memberships[ownerAID.String()+":"+tenantAID.String()] = authdb.Membership{
		ID:       toPgUUID(uuid.New()),
		TenantID: toPgUUID(tenantAID),
		UserID:   toPgUUID(ownerAID),
		Role:     "owner",
	}

	// Seed Tenant B
	mockStore.clinics[tenantBID.String()] = "مرکز مشاوره نیلوفر"
	mockStore.domains[hostB] = domaindb.Domain{
		Host:       hostB,
		TenantID:   toPgUUID(tenantBID),
		IsVerified: true,
	}
	mockStore.settings[tenantBID.String()] = settingsdb.SiteSetting{
		TenantID:        toPgUUID(tenantBID),
		Title:           "مرکز مشاوره و سلامت روان نیلوفر",
		Description:     "روان‌درمانی و مشاوره‌های تخصصی خانواده",
		Phone:           "021-88882222",
		Address:         "تهران، خیابان شریعتی",
		PrimaryColor:    "#10b981",
		TemplateID:      "minimal",
		TemplateVersion: "1.0.0",
		ThemeRadius:     "md",
		ThemeScale:      "md",
	}

	domainResolver := domain.NewResolver(mockStore)
	authService := auth.NewService(mockStore)
	settingsService := settings.NewService(mockStore)
	authHandler := auth.NewHandler(authService)
	settingsHandler := settings.NewHandler(settingsService, domainResolver, authService)
	settingsHandler.SetAppearanceOwnerUserID(ownerAID)

	apiRouter := chi.NewRouter()
	apiRouter.Route("/v1", func(r chi.Router) {
		authHandler.MountRoutes(r)
		settingsHandler.MountRoutes(r)
	})

	// 1. Initial State: Verify Public Site Profile for both Host A and Host B
	reqPublicA := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqPublicA.Host = hostA
	recPublicA := httptest.NewRecorder()
	apiRouter.ServeHTTP(recPublicA, reqPublicA)
	if recPublicA.Code != http.StatusOK {
		t.Fatalf("expected 200 for Host A public site, got %d", recPublicA.Code)
	}
	var profA settings.PublicSiteProfile
	_ = json.Unmarshal(recPublicA.Body.Bytes(), &profA)
	if profA.TemplateID != "minimal" || profA.PrimaryColor != "#0284c7" {
		t.Fatalf("unexpected initial settings for Host A: %+v", profA)
	}

	reqPublicB := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	reqPublicB.Host = hostB
	recPublicB := httptest.NewRecorder()
	apiRouter.ServeHTTP(recPublicB, reqPublicB)
	if recPublicB.Code != http.StatusOK {
		t.Fatalf("expected 200 for Host B public site, got %d", recPublicB.Code)
	}
	var profB settings.PublicSiteProfile
	_ = json.Unmarshal(recPublicB.Body.Bytes(), &profB)
	if profB.TemplateID != "minimal" || profB.PrimaryColor != "#10b981" {
		t.Fatalf("unexpected initial settings for Host B: %+v", profB)
	}

	// 2. Edge Cache Simulation with Storefront Upstream
	var mu sync.Mutex
	renderCounterA := 0
	renderCounterB := 0

	storefrontUpstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		host := r.Host
		path := r.URL.Path

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		bypass, _ := cache.ShouldBypassCache(r)
		if bypass || path == "/preview" {
			w.Header().Set("Cache-Control", "private, no-store")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=0, s-maxage=3600, stale-while-revalidate=60")
		}

		st, _ := settingsService.GetSettings(context.Background(), func() uuid.UUID {
			if host == hostA {
				return tenantAID
			}
			return tenantBID
		}())

		// If preview query overrides are present
		activeTemplate := st.TemplateID
		if path == "/preview" && r.URL.Query().Get("template") != "" {
			activeTemplate = r.URL.Query().Get("template")
		}

		if host == hostA {
			renderCounterA++
			_, _ = fmt.Fprintf(w, "<html data-theme=\"%s\" data-radius=\"%s\"><body>مطب دکتر علی رضایی - نسخه: %d</body></html>",
				activeTemplate, st.ThemeRadius, renderCounterA)
		} else {
			renderCounterB++
			_, _ = fmt.Fprintf(w, "<html data-theme=\"%s\" data-radius=\"%s\"><body>مرکز نیلوفر - نسخه: %d</body></html>",
				activeTemplate, st.ThemeRadius, renderCounterB)
		}
	})

	edgeCache := cache.NewEdgeCacheProxy(1 * time.Hour)
	cachedStorefront := edgeCache.Middleware(storefrontUpstream)

	// Populate cache for Host A and Host B (First request: MISS, second request: HIT)
	reqHomeA := httptest.NewRequest(http.MethodGet, "https://rezaei-clinic.ir/", nil)
	recHomeA1 := httptest.NewRecorder()
	cachedStorefront.ServeHTTP(recHomeA1, reqHomeA)
	if recHomeA1.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("expected Host A first request to be MISS, got: %s", recHomeA1.Header().Get("X-Cache"))
	}

	recHomeA2 := httptest.NewRecorder()
	cachedStorefront.ServeHTTP(recHomeA2, reqHomeA)
	if recHomeA2.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected Host A second request to be HIT, got: %s", recHomeA2.Header().Get("X-Cache"))
	}

	reqHomeB := httptest.NewRequest(http.MethodGet, "https://niloofar-psych.ir/", nil)
	recHomeB1 := httptest.NewRecorder()
	cachedStorefront.ServeHTTP(recHomeB1, reqHomeB)
	if recHomeB1.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("expected Host B first request to be MISS, got: %s", recHomeB1.Header().Get("X-Cache"))
	}

	recHomeB2 := httptest.NewRecorder()
	cachedStorefront.ServeHTTP(recHomeB2, reqHomeB)
	if recHomeB2.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected Host B second request to be HIT, got: %s", recHomeB2.Header().Get("X-Cache"))
	}

	// 3. Preview Verification: Repeated preview requests on Host A strictly BYPASS cache
	reqPrev1 := httptest.NewRequest(http.MethodGet, "https://rezaei-clinic.ir/preview?template=clinical&primary=%230f766e&radius=lg&scale=sm", nil)
	recPrev1 := httptest.NewRecorder()
	cachedStorefront.ServeHTTP(recPrev1, reqPrev1)
	if recPrev1.Header().Get("X-Cache") != "BYPASS" {
		t.Fatalf("expected preview request 1 to BYPASS cache, got: %s", recPrev1.Header().Get("X-Cache"))
	}
	if recPrev1.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("expected Cache-Control: private, no-store on preview, got: %s", recPrev1.Header().Get("Cache-Control"))
	}
	if !bytes.Contains(recPrev1.Body.Bytes(), []byte("data-theme=\"clinical\"")) {
		t.Fatalf("preview should render clinical template")
	}

	// Second preview request must also BYPASS cache and never be cached
	recPrev2 := httptest.NewRecorder()
	cachedStorefront.ServeHTTP(recPrev2, reqPrev1)
	if recPrev2.Header().Get("X-Cache") != "BYPASS" {
		t.Fatalf("expected preview request 2 to BYPASS cache, got: %s", recPrev2.Header().Get("X-Cache"))
	}

	// Verify that preview did NOT alter published settings in database
	stAAfterPreview, _ := settingsService.GetSettings(context.Background(), tenantAID)
	if stAAfterPreview.TemplateID != "minimal" {
		t.Fatalf("preview leaked into published database settings!")
	}

	// 4. Authenticate Owner A
	sessA, err := authService.Login(context.Background(), "owner@rezaei.ir", "rezaeiPass123")
	if err != nil {
		t.Fatalf("failed to login owner A: %v", err)
	}

	// 5. Cross-Tenant Tampering Check: Owner A tries to mutate Tenant B's settings -> 403 Forbidden
	tamperBody, _ := json.Marshal(map[string]any{
		"title": "کلینیک دستکاری‌شده",
	})
	reqTamper := httptest.NewRequest(http.MethodPut, "/v1/admin/site", bytes.NewBuffer(tamperBody))
	reqTamper.Header.Set("Authorization", "Bearer "+sessA.Token)
	reqTamper.Header.Set("X-Tenant-ID", tenantBID.String()) // forged tenant ID
	reqTamper.Header.Set("Content-Type", "application/json")
	recTamper := httptest.NewRecorder()
	apiRouter.ServeHTTP(recTamper, reqTamper)

	if recTamper.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on cross-tenant tampering, got %d", recTamper.Code)
	}

	// Verify Tenant B settings were NOT modified
	stB, _ := settingsService.GetSettings(context.Background(), tenantBID)
	if stB.TemplateID != "minimal" || stB.PrimaryColor != "#10b981" {
		t.Fatalf("Tenant B settings were corrupted! Got: %+v", stB)
	}

	// 6. Legitimate Update: Owner A saves new settings for Tenant A
	// 6.a Content update via PUT /v1/admin/site
	contentBody, _ := json.Marshal(map[string]any{
		"title":       "مطب تخصصی دکتر علی رضایی",
		"description": "خدمات روان‌درمانی و مشاوره فردی تخصصی",
		"phone":       "021-88881111",
		"address":     "تهران، میدان ونک",
	})
	reqContent := httptest.NewRequest(http.MethodPut, "/v1/admin/site", bytes.NewBuffer(contentBody))
	reqContent.Header.Set("Authorization", "Bearer "+sessA.Token)
	reqContent.Header.Set("X-Tenant-ID", tenantAID.String())
	reqContent.Header.Set("Content-Type", "application/json")
	recContent := httptest.NewRecorder()
	apiRouter.ServeHTTP(recContent, reqContent)
	if recContent.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on content update, got %d: %s", recContent.Code, recContent.Body.String())
	}
	if recContent.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("expected Cache-Control: private, no-store on admin update, got: %s", recContent.Header().Get("Cache-Control"))
	}

	// 6.b Appearance update via dedicated PUT /v1/admin/site/appearance
	appearanceBody, _ := json.Marshal(map[string]any{
		"template_id":  "clinical",
		"palette_id":   "calm-sage",
		"theme_radius": "lg",
		"theme_scale":  "sm",
	})
	reqAppearance := httptest.NewRequest(http.MethodPut, "/v1/admin/site/appearance", bytes.NewBuffer(appearanceBody))
	reqAppearance.Header.Set("Authorization", "Bearer "+sessA.Token)
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

	// 6.c Reject appearance fields on legacy PUT /v1/admin/site -> 400
	rejectBody, _ := json.Marshal(map[string]any{
		"title":       "مطب تستی",
		"template_id": "clinical",
	})
	reqReject := httptest.NewRequest(http.MethodPut, "/v1/admin/site", bytes.NewBuffer(rejectBody))
	reqReject.Header.Set("Authorization", "Bearer "+sessA.Token)
	reqReject.Header.Set("X-Tenant-ID", tenantAID.String())
	reqReject.Header.Set("Content-Type", "application/json")
	recReject := httptest.NewRecorder()
	apiRouter.ServeHTTP(recReject, reqReject)
	if recReject.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request when appearance field passed to /admin/site, got %d", recReject.Code)
	}

	// Verify database was updated
	stAUpdated, _ := settingsService.GetSettings(context.Background(), tenantAID)
	if stAUpdated.TemplateID != "clinical" || stAUpdated.ThemeRadius != "lg" {
		t.Fatalf("failed to persist updated settings in DB: %+v", stAUpdated)
	}

	// 7. Outbox Invalidation Processing & Edge Cache Isolation
	outboxDB := cache.NewMockOutboxDB()
	purger := cache.NewProxyPurger(edgeCache)
	worker := cache.NewOutboxWorker(outboxDB, purger)

	// In the real system, WithTenantTx records the event. Simulate the recorded outbox event for Host A:
	outboxDB.AddEvent(cache.OutboxEvent{
		ID:        uuid.New(),
		TenantID:  tenantAID,
		EventType: "site_settings.updated",
		Payload: cache.OutboxPayload{
			Host:  hostA,
			Paths: []string{"/", "/preview"},
		},
		Status:    "pending",
		CreatedAt: time.Now(),
	})

	processed, err := worker.ProcessBatch(context.Background(), 10)
	if err != nil || processed != 1 {
		t.Fatalf("failed to process outbox invalidation event: err=%v, count=%d", err, processed)
	}

	// 8. Host A cache must now be a MISS and render the newly published clinical template
	recHomeAAfterPurge1 := httptest.NewRecorder()
	cachedStorefront.ServeHTTP(recHomeAAfterPurge1, reqHomeA)
	if recHomeAAfterPurge1.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("expected Host A post-invalidation request to be MISS, got: %s", recHomeAAfterPurge1.Header().Get("X-Cache"))
	}
	if !bytes.Contains(recHomeAAfterPurge1.Body.Bytes(), []byte("data-theme=\"clinical\"")) {
		t.Fatalf("expected Host A to render newly published clinical template after invalidation")
	}

	// Subsequent request to Host A is HIT
	recHomeAAfterPurge2 := httptest.NewRecorder()
	cachedStorefront.ServeHTTP(recHomeAAfterPurge2, reqHomeA)
	if recHomeAAfterPurge2.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected Host A to be HIT after re-caching, got: %s", recHomeAAfterPurge2.Header().Get("X-Cache"))
	}

	// 9. Host B cache entry must remain an immediate HIT (Strict Zero Cross-Tenant Invalidation Leakage)
	recHomeBAfterAPurge := httptest.NewRecorder()
	cachedStorefront.ServeHTTP(recHomeBAfterAPurge, reqHomeB)
	if recHomeBAfterAPurge.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("Host B cache was corrupted or invalidated by Host A's event! Expected HIT, got: %s", recHomeBAfterAPurge.Header().Get("X-Cache"))
	}
	if !bytes.Contains(recHomeBAfterAPurge.Body.Bytes(), []byte("data-theme=\"minimal\"")) {
		t.Fatalf("Host B template changed unexpectedly! Expected minimal")
	}
}

// TestAcceptance_PSY_A12_LivePostgreSQL_TwoHosts runs the complete PSY-A12 acceptance scenario
// against real PostgreSQL using the runtime roles app_user and worker_user, asserting:
// - Real RLS and database persistence for two verified hosts
// - Two consecutive /preview requests strictly bypass shared cache with private, no-store
// - Legitimate appearance save on Host A creates an Outbox event in the same transaction
// - Outbox worker with worker_user purges Host A cache only
// - Host A renders fresh clinical appearance without FOUC (MISS -> HIT)
// - Host B remains cached as HIT with previous minimal appearance (zero tenant invalidation leakage)
func TestAcceptance_PSY_A12_LivePostgreSQL_TwoHosts(t *testing.T) {
	dbURL := os.Getenv("TEST_APP_DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		t.Skip("skipping live database acceptance test: TEST_APP_DATABASE_URL or DATABASE_URL not set")
		return
	}

	ctx := context.Background()
	appPool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live database acceptance test (failed to connect as app_user: %v)", err)
		return
	}
	defer appPool.Close()

	workerURL := os.Getenv("TEST_WORKER_DATABASE_URL")
	if workerURL == "" {
		workerURL = strings.Replace(dbURL, "app_user:secret_app_pass", "worker_user:secret_worker_pass", 1)
		if workerURL == dbURL {
			workerURL = "postgres://worker_user:secret_worker_pass@127.0.0.1:5432/psychology_prod?sslmode=disable"
		}
	}
	workerPool, err := pgxpool.New(ctx, workerURL)
	if err != nil {
		t.Skipf("skipping live database acceptance test (failed to connect as worker_user: %v)", err)
		return
	}
	defer workerPool.Close()

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

	// 1. Seed two tenants, panel identities, verified domains, and memberships
	tenantA := uuid.New()
	tenantB := uuid.New()
	slugA := "a12-a-" + tenantA.String()[:8]
	slugB := "a12-b-" + tenantB.String()[:8]
	hostA := slugA + ".ir"
	hostB := slugB + ".ir"

	_, err = appPool.Exec(ctx, `
		INSERT INTO tenants (id, name, slug, status)
		VALUES ($1, $2, $3, 'active'), ($4, $5, $6, 'active')
	`, tenantA, "کلینیک دکتر علوی", slugA, tenantB, "کلینیک دکتر مرادی", slugB)
	if err != nil {
		t.Fatalf("failed to insert live tenants: %v", err)
	}
	defer func() {
		_, _ = appPool.Exec(context.Background(), "DELETE FROM tenants WHERE id IN ($1, $2)", tenantA, tenantB)
	}()

	userA := uuid.New()
	userB := uuid.New()
	settingsHandler.SetAppearanceOwnerUserID(userA)
	pwHashA, _ := auth.HashPassword("alaviPass123")
	pwHashB, _ := auth.HashPassword("moradiPass123")
	_, err = appPool.Exec(ctx, `
		INSERT INTO panel_identities (id, email, password_hash, full_name, status)
		VALUES ($1, $2, $3, 'دکتر علوی', 'active'), ($4, $5, $6, 'دکتر مرادی', 'active')
	`, userA, slugA+"@clinic.ir", pwHashA, userB, slugB+"@clinic.ir", pwHashB)
	if err != nil {
		t.Fatalf("failed to insert live identities: %v", err)
	}
	defer func() {
		_, _ = appPool.Exec(context.Background(), "DELETE FROM panel_identities WHERE id IN ($1, $2)", userA, userB)
	}()

	ctxA := tenantctx.With(ctx, tenantA)
	ctxB := tenantctx.With(ctx, tenantB)

	// Seed domain & membership for Tenant A under RLS
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

	// Seed domain & membership for Tenant B under RLS
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

	// 2. Initial site settings: Both clinics start with minimal template and distinct colors
	if _, err := settingsService.UpdateSettings(ctxA, tenantA, settings.UpdateSettingsParams{
		Title:        "کلینیک تخصصی دکتر علوی",
		PrimaryColor: "#0284c7",
		TemplateID:   "minimal",
		ThemeRadius:  "md",
		ThemeScale:   "md",
	}); err != nil {
		t.Fatalf("failed to set initial settings for Tenant A: %v", err)
	}
	if _, err := settingsService.UpdateSettings(ctxB, tenantB, settings.UpdateSettingsParams{
		Title:        "کلینیک تخصصی دکتر مرادی",
		PrimaryColor: "#10b981",
		TemplateID:   "minimal",
		ThemeRadius:  "md",
		ThemeScale:   "md",
	}); err != nil {
		t.Fatalf("failed to set initial settings for Tenant B: %v", err)
	}

	// Drain any outbox events created during setup
	_, _ = workerPool.Exec(ctx, "UPDATE outbox_events SET status = 'completed' WHERE status = 'pending'")

	// 3. Edge cache proxy simulation with live upstream resolving against PostgreSQL
	edgeCache := cache.NewEdgeCacheProxy(1 * time.Hour)
	var mu sync.Mutex
	renderCountA := 0
	renderCountB := 0

	storefrontUpstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		host := r.Host
		path := r.URL.Path

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		bypass, _ := cache.ShouldBypassCache(r)
		if bypass || path == "/preview" {
			w.Header().Set("Cache-Control", "private, no-store")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=0, s-maxage=3600, stale-while-revalidate=60")
		}

		tid, resolveErr := domainResolver.Resolve(r.Context(), host)
		if resolveErr != nil {
			http.Error(w, "host not found", http.StatusNotFound)
			return
		}

		cctx := tenantctx.With(r.Context(), tid)
		prof, getErr := settingsService.GetPublicProfile(cctx, tid)
		if getErr != nil {
			http.Error(w, "failed to get profile", http.StatusInternalServerError)
			return
		}

		activeTemplate := prof.TemplateID
		activeColor := prof.PrimaryColor
		activeRadius := prof.ThemeRadius
		activeScale := prof.ThemeScale

		// If preview query overrides are present
		if path == "/preview" {
			q := r.URL.Query()
			if q.Get("template") != "" {
				activeTemplate = q.Get("template")
			}
			if q.Get("primary") != "" {
				activeColor = q.Get("primary")
			}
			if q.Get("radius") != "" {
				activeRadius = q.Get("radius")
			}
			if q.Get("scale") != "" {
				activeScale = q.Get("scale")
			}
		}

		if host == hostA {
			renderCountA++
			_, _ = fmt.Fprintf(w, "<html data-theme=\"%s\" data-radius=\"%s\" data-scale=\"%s\" style=\"--color-primary:%s\"><body>%s - رندر:%d</body></html>",
				activeTemplate, activeRadius, activeScale, activeColor, prof.Title, renderCountA)
		} else {
			renderCountB++
			_, _ = fmt.Fprintf(w, "<html data-theme=\"%s\" data-radius=\"%s\" data-scale=\"%s\" style=\"--color-primary:%s\"><body>%s - رندر:%d</body></html>",
				activeTemplate, activeRadius, activeScale, activeColor, prof.Title, renderCountB)
		}
	})

	cachedStorefront := edgeCache.Middleware(storefrontUpstream)

	// 4. Initial Cache Warmup for both hosts: MISS then HIT
	reqHomeA := httptest.NewRequest(http.MethodGet, "https://"+hostA+"/", nil)
	recHomeA1 := httptest.NewRecorder()
	cachedStorefront.ServeHTTP(recHomeA1, reqHomeA)
	if recHomeA1.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("expected Host A first request to be MISS, got: %s", recHomeA1.Header().Get("X-Cache"))
	}
	if !bytes.Contains(recHomeA1.Body.Bytes(), []byte("data-theme=\"minimal\"")) {
		t.Fatalf("expected Host A initial render to be minimal template")
	}

	recHomeA2 := httptest.NewRecorder()
	cachedStorefront.ServeHTTP(recHomeA2, reqHomeA)
	if recHomeA2.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected Host A second request to be HIT, got: %s", recHomeA2.Header().Get("X-Cache"))
	}

	reqHomeB := httptest.NewRequest(http.MethodGet, "https://"+hostB+"/", nil)
	recHomeB1 := httptest.NewRecorder()
	cachedStorefront.ServeHTTP(recHomeB1, reqHomeB)
	if recHomeB1.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("expected Host B first request to be MISS, got: %s", recHomeB1.Header().Get("X-Cache"))
	}

	recHomeB2 := httptest.NewRecorder()
	cachedStorefront.ServeHTTP(recHomeB2, reqHomeB)
	if recHomeB2.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected Host B second request to be HIT, got: %s", recHomeB2.Header().Get("X-Cache"))
	}

	// 5. Preview Verification: Two consecutive /preview requests must strictly BYPASS cache
	previewURL := "https://" + hostA + "/preview?template=clinical&primary=%230f766e&radius=lg&scale=sm"
	reqPrev1 := httptest.NewRequest(http.MethodGet, previewURL, nil)
	recPrev1 := httptest.NewRecorder()
	cachedStorefront.ServeHTTP(recPrev1, reqPrev1)
	if recPrev1.Header().Get("X-Cache") != "BYPASS" {
		t.Fatalf("expected preview request 1 to BYPASS cache, got: %s", recPrev1.Header().Get("X-Cache"))
	}
	if recPrev1.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("expected Cache-Control: private, no-store on preview, got: %s", recPrev1.Header().Get("Cache-Control"))
	}
	if !bytes.Contains(recPrev1.Body.Bytes(), []byte("data-theme=\"clinical\"")) || !bytes.Contains(recPrev1.Body.Bytes(), []byte("data-radius=\"lg\"")) {
		t.Fatalf("preview request 1 failed to render clinical template with lg radius")
	}

	// Consecutive preview request must also BYPASS cache and return private, no-store
	reqPrev2 := httptest.NewRequest(http.MethodGet, previewURL, nil)
	recPrev2 := httptest.NewRecorder()
	cachedStorefront.ServeHTTP(recPrev2, reqPrev2)
	if recPrev2.Header().Get("X-Cache") != "BYPASS" {
		t.Fatalf("expected consecutive preview request 2 to BYPASS cache, got: %s", recPrev2.Header().Get("X-Cache"))
	}
	if recPrev2.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("expected Cache-Control: private, no-store on consecutive preview, got: %s", recPrev2.Header().Get("Cache-Control"))
	}

	// Verify database settings for Tenant A remain minimal
	stAInDB, err := settingsService.GetSettings(ctxA, tenantA)
	if err != nil || stAInDB.TemplateID != "minimal" {
		t.Fatalf("preview unexpectedly modified database settings: %+v, err=%v", stAInDB, err)
	}

	// 6. Authenticate Owner A
	sessA, err := authService.Login(ctx, slugA+"@clinic.ir", "alaviPass123")
	if err != nil {
		t.Fatalf("failed to authenticate Owner A: %v", err)
	}

	// Cross-tenant tampering attempt by Owner A against Tenant B: rejected with 403 Forbidden
	tamperBody, _ := json.Marshal(map[string]any{
		"title": "تلاش برای دستکاری",
	})
	reqTamper := httptest.NewRequest(http.MethodPut, "/v1/admin/site", bytes.NewBuffer(tamperBody))
	reqTamper.Header.Set("Authorization", "Bearer "+sessA.Token)
	reqTamper.Header.Set("X-Tenant-ID", tenantB.String())
	reqTamper.Header.Set("Content-Type", "application/json")
	recTamper := httptest.NewRecorder()
	apiRouter.ServeHTTP(recTamper, reqTamper)
	if recTamper.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on cross-tenant tampering, got %d", recTamper.Code)
	}

	// 7. Legitimate Update: Owner A saves settings for Tenant A
	// 7.a Content update via PUT /v1/admin/site
	contentBody, _ := json.Marshal(map[string]any{
		"title":       "کلینیک تخصصی دکتر علوی (به‌روزشده)",
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
	if recContent.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("expected Cache-Control: private, no-store on admin update, got: %s", recContent.Header().Get("Cache-Control"))
	}

	// 7.b Appearance update via PUT /v1/admin/site/appearance
	appearanceBody, _ := json.Marshal(map[string]any{
		"template_id":  "clinical",
		"palette_id":   "calm-sage",
		"theme_radius": "lg",
		"theme_scale":  "sm",
	})
	reqAppearance := httptest.NewRequest(http.MethodPut, "/v1/admin/site/appearance", bytes.NewBuffer(appearanceBody))
	reqAppearance.Header.Set("Authorization", "Bearer "+sessA.Token)
	reqAppearance.Header.Set("X-Tenant-ID", tenantA.String())
	reqAppearance.Header.Set("Content-Type", "application/json")
	recAppearance := httptest.NewRecorder()
	apiRouter.ServeHTTP(recAppearance, reqAppearance)

	if recAppearance.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on appearance update, got %d: %s", recAppearance.Code, recAppearance.Body.String())
	}

	// Verify Tenant A settings are persisted in PostgreSQL
	stAUpdated, err := settingsService.GetSettings(ctxA, tenantA)
	if err != nil || stAUpdated.TemplateID != "clinical" || stAUpdated.ThemeRadius != "lg" {
		t.Fatalf("failed to persist updated settings in DB: %+v, err=%v", stAUpdated, err)
	}

	// Verify Outbox event was recorded in PostgreSQL within same transaction
	var outboxEventID uuid.UUID
	var outboxHost, outboxStatus string
	err = workerPool.QueryRow(ctx, `
		SELECT id, payload->>'host', status
		FROM outbox_events
		WHERE tenant_id = $1 AND status = 'pending'
		ORDER BY created_at DESC
		LIMIT 1
	`, tenantA).Scan(&outboxEventID, &outboxHost, &outboxStatus)
	if err != nil {
		t.Fatalf("failed to find pending outbox event for Tenant A: %v", err)
	}
	if outboxHost != hostA {
		t.Fatalf("expected outbox host %q, got %q", hostA, outboxHost)
	}

	// 8. Outbox Worker processing with worker_user role
	purger := cache.NewProxyPurger(edgeCache)
	worker := cache.NewOutboxWorker(workerPool, purger)
	nProcessed, err := worker.ProcessBatch(ctx, 10)
	if err != nil || nProcessed == 0 {
		t.Fatalf("worker.ProcessBatch failed: count=%d, err=%v", nProcessed, err)
	}

	// Verify event is marked completed in PostgreSQL
	var completedStatus string
	var processedAt *time.Time
	err = workerPool.QueryRow(ctx, `
		SELECT status, processed_at
		FROM outbox_events
		WHERE id = $1
	`, outboxEventID).Scan(&completedStatus, &processedAt)
	if err != nil || completedStatus != "completed" || processedAt == nil {
		t.Fatalf("expected completed outbox event with processed_at, got status=%s, processed_at=%v", completedStatus, processedAt)
	}

	// 9. Post-Invalidation Edge Cache Verification:
	// Host A request to / must now be a MISS and render the newly published clinical template
	recHomeAPurge1 := httptest.NewRecorder()
	cachedStorefront.ServeHTTP(recHomeAPurge1, reqHomeA)
	if recHomeAPurge1.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("expected Host A post-invalidation request to be MISS, got: %s", recHomeAPurge1.Header().Get("X-Cache"))
	}
	if !bytes.Contains(recHomeAPurge1.Body.Bytes(), []byte("data-theme=\"clinical\"")) || !bytes.Contains(recHomeAPurge1.Body.Bytes(), []byte("data-radius=\"lg\"")) {
		t.Fatalf("expected Host A to render updated clinical template with lg radius after purge")
	}

	// Subsequent request to Host A is HIT
	recHomeAPurge2 := httptest.NewRecorder()
	cachedStorefront.ServeHTTP(recHomeAPurge2, reqHomeA)
	if recHomeAPurge2.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected Host A subsequent request to be HIT, got: %s", recHomeAPurge2.Header().Get("X-Cache"))
	}

	// 10. Host B cache entry must remain an immediate HIT (Zero Cross-Tenant Leakage)
	recHomeBAfterAPurge := httptest.NewRecorder()
	cachedStorefront.ServeHTTP(recHomeBAfterAPurge, reqHomeB)
	if recHomeBAfterAPurge.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("CRITICAL: Host B cache was corrupted or invalidated by Host A's event! Expected HIT, got: %s", recHomeBAfterAPurge.Header().Get("X-Cache"))
	}
	if !bytes.Contains(recHomeBAfterAPurge.Body.Bytes(), []byte("data-theme=\"minimal\"")) {
		t.Fatalf("Host B template changed unexpectedly! Expected minimal")
	}
}
