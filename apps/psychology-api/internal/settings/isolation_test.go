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
	"psychology/api/internal/testsupport/dbfixture"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// TestSiteSettings_SchemaRLSGuard verifies that the database schema mandates
// FORCE ROW LEVEL SECURITY and the tenant_isolation policy on site_settings.
func TestSiteSettings_SchemaRLSGuard(t *testing.T) {
	candidates := []string{
		"../../migrations/000001_create_core_schema.up.sql",
		"../migrations/000001_create_core_schema.up.sql",
		"migrations/000001_create_core_schema.up.sql",
	}

	var content string
	var err error
	for _, c := range candidates {
		data, readErr := os.ReadFile(c)
		if readErr == nil {
			content = string(data)
			break
		}
		err = readErr
	}
	if content == "" {
		t.Fatalf("could not read 000001_create_core_schema.up.sql: %v", err)
	}

	requiredDirectives := []string{
		"CREATE TABLE site_settings",
		"ALTER TABLE site_settings ENABLE ROW LEVEL SECURITY;",
		"ALTER TABLE site_settings FORCE  ROW LEVEL SECURITY;",
		"CREATE POLICY tenant_isolation ON site_settings",
		"USING      (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)",
		"WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::uuid)",
	}

	for _, directive := range requiredDirectives {
		if !strings.Contains(content, directive) {
			t.Errorf("site_settings missing required RLS directive: %q", directive)
		}
	}
}

// TestSiteSettings_LiveTenantIsolation executes against real PostgreSQL when
// the disposable database lane is configured, asserting runtime role isolation.
func TestSiteSettings_LiveTenantIsolation(t *testing.T) {
	ctx := context.Background()
	pool := dbfixture.Connect(t, dbfixture.AppDSN, "app_user")

	store := db.NewStore(pool)
	svc := settings.NewServiceWithStore(store)

	// Create two distinct tenant IDs
	tenantA := uuid.New()
	tenantB := uuid.New()

	ctxA := tenantctx.With(ctx, tenantA)
	ctxB := tenantctx.With(ctx, tenantB)

	// PSY-5.12: Seed initial tenants, panel identities, domains, and memberships
	// so foreign keys are satisfied under FORCE RLS and app_user role.
	slugA := "clinic-a-" + tenantA.String()[:8]
	slugB := "clinic-b-" + tenantB.String()[:8]
	_, err := pool.Exec(ctx, `
		INSERT INTO tenants (id, name, slug, status)
		VALUES ($1, $2, $3, 'active'), ($4, $5, $6, 'active')
	`, tenantA, "کلینیک امید", slugA, tenantB, "کلینیک آرامش", slugB)
	if err != nil {
		t.Fatalf("failed to insert initial tenants: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM tenants WHERE id IN ($1, $2)", tenantA, tenantB)
	}()

	userA := uuid.New()
	userB := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO panel_identities (id, email, password_hash, full_name, status)
		VALUES ($1, $2, 'hash', 'دکتر امیدی', 'active'), ($3, $4, 'hash', 'دکتر آرامش', 'active')
	`, userA, slugA+"@clinic.ir", userB, slugB+"@clinic.ir")
	if err != nil {
		t.Fatalf("failed to insert initial panel identities: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM panel_identities WHERE id IN ($1, $2)", userA, userB)
	}()

	// Seed Tenant A domain and membership under Tenant A context (respecting RLS)
	err = store.WithTenantTx(ctxA, func(tx pgx.Tx, tid uuid.UUID) error {
		hostA := slugA + ".ir"
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
		t.Fatalf("failed to insert domain/membership for Tenant A: %v", err)
	}

	// Seed Tenant B domain and membership under Tenant B context (respecting RLS)
	err = store.WithTenantTx(ctxB, func(tx pgx.Tx, tid uuid.UUID) error {
		hostB := slugB + ".ir"
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
		t.Fatalf("failed to insert domain/membership for Tenant B: %v", err)
	}

	// 1. Tenant A updates settings
	paramsA := settings.UpdateSettingsParams{
		Title:        "کلینیک تخصصی امید",
		Description:  "روان‌شناسی بالینی و مشاوره فردی",
		Phone:        "02188888888",
		Address:      "تهران، میدان ونک",
		PrimaryColor: "#0284c7",
	}

	savedA, err := svc.UpdateSettings(ctxA, tenantA, paramsA)
	if err != nil {
		t.Fatalf("failed to update settings for Tenant A: %v", err)
	}
	if savedA.Title != paramsA.Title || savedA.PrimaryColor != paramsA.PrimaryColor {
		t.Fatalf("unexpected saved settings for Tenant A: %+v", savedA)
	}

	// 2. Tenant B cannot read Tenant A's settings (must return empty/default settings)
	settingsB, err := svc.GetSettings(ctxB, tenantB)
	if err != nil {
		t.Fatalf("Tenant B get settings failed: %v", err)
	}
	if settingsB.Title != "" {
		t.Fatalf("Tenant B must not see Tenant A's title! Got: %q", settingsB.Title)
	}

	// 3. Raw SQL attack: Tenant B attempts to mutate Tenant A's row -> 0 rows affected
	var updatedRows, deletedRows int64
	err = store.WithTenantTx(ctxB, func(tx pgx.Tx, _ uuid.UUID) error {
		tag, err := tx.Exec(ctx, "UPDATE site_settings SET title = 'hacked' WHERE tenant_id = $1", tenantA)
		if err != nil {
			return err
		}
		updatedRows = tag.RowsAffected()

		delTag, err := tx.Exec(ctx, "DELETE FROM site_settings WHERE tenant_id = $1", tenantA)
		if err != nil {
			return err
		}
		deletedRows = delTag.RowsAffected()
		return nil
	})
	if err != nil {
		t.Fatalf("raw attack transaction failed: %v", err)
	}
	if updatedRows != 0 || deletedRows != 0 {
		t.Fatalf("Tenant B successfully modified/deleted Tenant A settings! updated=%d, deleted=%d", updatedRows, deletedRows)
	}

	// 4. Raw SQL attack: Tenant B attempts INSERT with Tenant A's tenant_id -> Rejected with RLS 42501
	err = store.WithTenantTx(ctxB, func(tx pgx.Tx, _ uuid.UUID) error {
		_, err := tx.Exec(ctx,
			"INSERT INTO site_settings (tenant_id, title, primary_color) VALUES ($1, 'evil', '#ff0000')",
			tenantA,
		)
		return err
	})
	if err == nil {
		t.Fatalf("expected RLS check violation when Tenant B inserts row with Tenant A ID, got nil")
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code != "42501" {
		t.Fatalf("expected PostgreSQL error 42501 (insufficient_privilege/RLS violation), got: %s", pgErr.Code)
	}

	// 5. Tenant B successfully updates its own settings
	paramsB := settings.UpdateSettingsParams{
		Title:        "کلینیک تخصصی آرامش",
		Description:  "مشاوره خانواده و زوج‌درمانی",
		Phone:        "02177777777",
		Address:      "تهران، پاسداران",
		PrimaryColor: "#10b981",
	}
	savedB, err := svc.UpdateSettings(ctxB, tenantB, paramsB)
	if err != nil {
		t.Fatalf("failed to update settings for Tenant B: %v", err)
	}
	if savedB.Title != paramsB.Title || savedB.PrimaryColor != paramsB.PrimaryColor {
		t.Fatalf("unexpected saved settings for Tenant B: %+v", savedB)
	}

	// 6. Tenant A verifies Tenant A's settings remain intact and unchanged by Tenant B
	readA, err := svc.GetSettings(ctxA, tenantA)
	if err != nil {
		t.Fatalf("Tenant A get settings failed: %v", err)
	}
	if readA.Title != paramsA.Title || readA.PrimaryColor != paramsA.PrimaryColor {
		t.Fatalf("Tenant A settings mutated by Tenant B! Got: %+v", readA)
	}
}

// TestOutbox_LiveTransactionalWorkflowAndPurge tests task PSY-5.14 on real PostgreSQL:
// - Atomic recording of changes and Outbox event in the same transaction
// - Rollback of settings and events on transaction failure
// - Worker queue processing with restricted worker_user permissions
// - Strict cache purging for the affected host only (zero tenant leak)
func TestOutbox_LiveTransactionalWorkflowAndPurge(t *testing.T) {
	ctx := context.Background()
	appPool := dbfixture.Connect(t, dbfixture.AppDSN, "app_user")
	workerPool := dbfixture.Connect(t, dbfixture.WorkerDSN, "worker_user")

	store := db.NewStore(appPool)
	svc := settings.NewServiceWithStore(store)

	// 1. Seed two tenants, panel identities, domains and memberships
	tenantA := uuid.New()
	tenantB := uuid.New()
	slugA := "outbox-a-" + tenantA.String()[:8]
	slugB := "outbox-b-" + tenantB.String()[:8]
	hostA := slugA + ".ir"
	hostB := slugB + ".ir"

	_, err := appPool.Exec(ctx, `
		INSERT INTO tenants (id, name, slug, status)
		VALUES ($1, $2, $3, 'active'), ($4, $5, $6, 'active')
	`, tenantA, "کلینیک امید", slugA, tenantB, "کلینیک فردوس", slugB)
	if err != nil {
		t.Fatalf("failed to insert live tenants: %v", err)
	}
	defer func() {
		_, _ = appPool.Exec(context.Background(), "DELETE FROM tenants WHERE id IN ($1, $2)", tenantA, tenantB)
	}()

	userA := uuid.New()
	userB := uuid.New()
	_, err = appPool.Exec(ctx, `
		INSERT INTO panel_identities (id, email, password_hash, full_name, status)
		VALUES ($1, $2, 'hash', 'دکتر امیدی', 'active'), ($3, $4, 'hash', 'دکتر فردوسی', 'active')
	`, userA, slugA+"@clinic.ir", userB, slugB+"@clinic.ir")
	if err != nil {
		t.Fatalf("failed to insert live identities: %v", err)
	}
	defer func() {
		_, _ = appPool.Exec(context.Background(), "DELETE FROM panel_identities WHERE id IN ($1, $2)", userA, userB)
	}()

	ctxA := tenantctx.With(ctx, tenantA)
	ctxB := tenantctx.With(ctx, tenantB)

	// Seed domain, membership, initial settings for Tenant A
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
	if _, err := svc.UpdateSettings(ctxA, tenantA, settings.UpdateSettingsParams{
		Title:        "کلینیک اولیه امید",
		PrimaryColor: "#0ea5e9",
		TemplateID:   "minimal",
	}); err != nil {
		t.Fatalf("failed to set initial settings for Tenant A: %v", err)
	}

	// Seed domain, membership, initial settings for Tenant B
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
	if _, err := svc.UpdateSettings(ctxB, tenantB, settings.UpdateSettingsParams{
		Title:        "کلینیک اولیه فردوس",
		PrimaryColor: "#10b981",
		TemplateID:   "minimal",
	}); err != nil {
		t.Fatalf("failed to set initial settings for Tenant B: %v", err)
	}

	// Drain any existing outbox events created during setup
	_, _ = workerPool.Exec(ctx, "UPDATE outbox_events SET status = 'completed' WHERE status = 'pending'")

	// 2. Setup EdgeCacheProxy and populate cache for both hosts
	proxy := cache.NewEdgeCacheProxy(1 * time.Hour)
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, s-maxage=3600")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("cached-" + r.Host + r.URL.Path))
	})
	proxyHandler := proxy.Middleware(upstream)

	for _, p := range []string{"/", "/courses", "/blog"} {
		reqA := httptest.NewRequest(http.MethodGet, "https://"+hostA+p, nil)
		recA := httptest.NewRecorder()
		proxyHandler.ServeHTTP(recA, reqA)

		reqB := httptest.NewRequest(http.MethodGet, "https://"+hostB+p, nil)
		recB := httptest.NewRecorder()
		proxyHandler.ServeHTTP(recB, reqB)
	}

	// Verify all entries are cached initially
	for _, p := range []string{"/", "/courses", "/blog"} {
		if _, ok := proxy.GetEntry(hostA, p); !ok {
			t.Fatalf("expected %s%s to be cached initially", hostA, p)
		}
		if _, ok := proxy.GetEntry(hostB, p); !ok {
			t.Fatalf("expected %s%s to be cached initially", hostB, p)
		}
	}

	// 3. Test Transaction Rollback on Failure:
	// Verify that if a failure occurs during the transaction, site_settings changes roll back
	// and no outbox event is committed to outbox_events.
	err = store.WithTenantTx(ctxA, func(tx pgx.Tx, tid uuid.UUID) error {
		if _, err := tx.Exec(ctx, `
			UPDATE site_settings SET title = 'تایتل موقت که باید رول‌بک شود' WHERE tenant_id = $1
		`, tid); err != nil {
			return err
		}
		if err := cache.InsertCacheInvalidationEvent(ctx, tx, tid, "site.updated", hostA, []string{"/test-fail"}); err != nil {
			return err
		}
		return errors.New("simulated transaction failure for rollback test")
	})
	if err == nil {
		t.Fatalf("expected error from transaction, got nil")
	}

	// Verify site_settings was NOT changed
	curSettingsA, err := svc.GetSettings(ctxA, tenantA)
	if err != nil {
		t.Fatalf("failed to get settings: %v", err)
	}
	if curSettingsA.Title == "تایتل موقت که باید رول‌بک شود" {
		t.Fatalf("ROLLBACK FAILED: site_settings retained title from aborted transaction")
	}

	// Verify outbox_events does NOT contain /test-fail event
	var failEventCount int
	err = workerPool.QueryRow(ctx, `
		SELECT COUNT(*) FROM outbox_events 
		WHERE tenant_id = $1 AND payload->'paths' ? '/test-fail'
	`, tenantA).Scan(&failEventCount)
	if err != nil {
		t.Fatalf("failed to query outbox_events: %v", err)
	}
	if failEventCount != 0 {
		t.Fatalf("ROLLBACK FAILED: outbox event was committed despite transaction rollback")
	}

	// 4. Test Successful Settings Update and Outbox Event Insertion in SAME transaction:
	paramsUpdatedA := settings.UpdateSettingsParams{
		Title:        "کلینیک تخصصی امید (به‌روزشده نهایی)",
		PrimaryColor: "#0284c7",
		TemplateID:   "minimal",
	}
	updatedA, err := svc.UpdateSettings(ctxA, tenantA, paramsUpdatedA)
	if err != nil {
		t.Fatalf("failed to update settings for Tenant A: %v", err)
	}
	if updatedA.Title != paramsUpdatedA.Title {
		t.Fatalf("expected updated title %q, got %q", paramsUpdatedA.Title, updatedA.Title)
	}

	// Query outbox_events as worker_user to verify event was recorded
	var eventID uuid.UUID
	var eventStatus, eventHost string
	err = workerPool.QueryRow(ctx, `
		SELECT id, status, payload->>'host'
		FROM outbox_events
		WHERE tenant_id = $1 AND status = 'pending'
		ORDER BY created_at DESC
		LIMIT 1
	`, tenantA).Scan(&eventID, &eventStatus, &eventHost)
	if err != nil {
		t.Fatalf("failed to find pending outbox event for Tenant A: %v", err)
	}
	if eventStatus != "pending" {
		t.Fatalf("expected event status 'pending', got %q", eventStatus)
	}
	if eventHost != hostA {
		t.Fatalf("expected event host %q, got %q", hostA, eventHost)
	}

	// 5. Test Worker Queue Processing with Restricted Privileges (worker_user):
	purger := cache.NewProxyPurger(proxy)
	worker := cache.NewOutboxWorker(workerPool, purger)
	n, err := worker.ProcessBatch(ctx, 10)
	if err != nil {
		t.Fatalf("worker.ProcessBatch failed: %v", err)
	}
	if n == 0 {
		t.Fatalf("expected at least 1 processed event, got 0")
	}

	// Verify event status in outbox_events is now completed
	var completedStatus string
	var processedAt *time.Time
	err = workerPool.QueryRow(ctx, `
		SELECT status, processed_at
		FROM outbox_events
		WHERE id = $1
	`, eventID).Scan(&completedStatus, &processedAt)
	if err != nil {
		t.Fatalf("failed to query processed event: %v", err)
	}
	if completedStatus != "completed" {
		t.Fatalf("expected completed status, got %q", completedStatus)
	}
	if processedAt == nil {
		t.Fatalf("expected processed_at timestamp to be set")
	}

	// 6. Test Cache Purge Isolation:
	// Host A paths must be purged from cache
	for _, p := range []string{"/", "/courses", "/blog"} {
		if _, ok := proxy.GetEntry(hostA, p); ok {
			t.Fatalf("expected %s%s to be purged, but was still in cache", hostA, p)
		}
	}

	// Host B paths must NOT be purged (zero cross-tenant leak!)
	for _, p := range []string{"/", "/courses", "/blog"} {
		entry, ok := proxy.GetEntry(hostB, p)
		if !ok {
			t.Fatalf("CRITICAL CACHE LEAK: hostB %s was incorrectly purged by hostA event!", p)
		}
		if string(entry.Body) != "cached-"+hostB+p {
			t.Fatalf("unexpected cached content for hostB %s: %s", p, string(entry.Body))
		}
	}

	// 7. Verify worker_user Privilege Restrictions:
	// worker_user cannot access site_settings, cannot INSERT, and cannot DELETE from outbox_events.
	if _, err := workerPool.Exec(ctx, "SELECT * FROM site_settings"); err == nil {
		t.Fatalf("SECURITY BREACH: worker_user was able to SELECT from site_settings")
	}
	if _, err := workerPool.Exec(ctx, "INSERT INTO outbox_events (tenant_id, event_type) VALUES ($1, 'hack')", tenantA); err == nil {
		t.Fatalf("SECURITY BREACH: worker_user was able to INSERT into outbox_events")
	}
	if _, err := workerPool.Exec(ctx, "DELETE FROM outbox_events"); err == nil {
		t.Fatalf("SECURITY BREACH: worker_user was able to DELETE from outbox_events")
	}
}

// TestSiteSettings_PaletteIdMigrationAndSeedIsolation verifies PSY-6.4:
// Migration 000011 adds palette_id with valid default 'calm-sage', check constraints,
// seeds mock clinics under runtime role app_user with FORCE RLS, and asserts
// strict cross-tenant isolation and constraint enforcement.
func TestSiteSettings_PaletteIdMigrationAndSeedIsolation(t *testing.T) {
	ctx := context.Background()
	pool := dbfixture.Connect(t, dbfixture.AppDSN, "app_user")

	// 1. Verify FORCE RLS and column existence on site_settings
	var relrowsecurity, relforcerowsecurity bool
	err := pool.QueryRow(ctx, `
		SELECT c.relrowsecurity, c.relforcerowsecurity
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relname = 'site_settings'
	`).Scan(&relrowsecurity, &relforcerowsecurity)
	if err != nil {
		t.Fatalf("failed to query site_settings RLS status: %v", err)
	}
	if !relrowsecurity || !relforcerowsecurity {
		t.Fatalf("site_settings must have both relrowsecurity=true and relforcerowsecurity=true, got rls=%v force=%v",
			relrowsecurity, relforcerowsecurity)
	}

	var colDefault, isNullable string
	err = pool.QueryRow(ctx, `
		SELECT column_default, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'site_settings' AND column_name = 'palette_id'
	`).Scan(&colDefault, &isNullable)
	if err != nil {
		t.Fatalf("palette_id column missing from site_settings: %v", err)
	}
	if !strings.Contains(colDefault, "calm-sage") {
		t.Fatalf("expected palette_id default to contain 'calm-sage', got %q", colDefault)
	}
	if isNullable != "NO" {
		t.Fatalf("palette_id must be NOT NULL, got is_nullable=%q", isNullable)
	}

	// 2. Seed two mock clinics (Tenant A and Tenant B)
	tenantA := uuid.New()
	tenantB := uuid.New()
	slugA := "seed-palette-a-" + tenantA.String()[:8]
	slugB := "seed-palette-b-" + tenantB.String()[:8]

	_, err = pool.Exec(ctx, `
		INSERT INTO tenants (id, name, slug, status)
		VALUES ($1, $2, $3, 'active'), ($4, $5, $6, 'active')
	`, tenantA, "کلینیک مریم‌گلی", slugA, tenantB, "کلینیک اقیانوس", slugB)
	if err != nil {
		t.Fatalf("failed to insert test tenants: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM tenants WHERE id IN ($1, $2)", tenantA, tenantB)
	}()

	store := db.NewStore(pool)
	ctxA := tenantctx.With(ctx, tenantA)
	ctxB := tenantctx.With(ctx, tenantB)

	// Seed Tenant A site_settings with the Stitch palette under Tenant A context.
	err = store.WithTenantTx(ctxA, func(tx pgx.Tx, tid uuid.UUID) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO site_settings (tenant_id, title, palette_id)
			VALUES ($1, 'کلینیک الف', $2)
			ON CONFLICT (tenant_id) DO UPDATE SET palette_id = EXCLUDED.palette_id
		`, tid, "stitch-editorial")
		return err
	})
	if err != nil {
		t.Fatalf("failed to seed Tenant A site_settings: %v", err)
	}

	// Seed Tenant B site_settings with ocean-trust under Tenant B context
	err = store.WithTenantTx(ctxB, func(tx pgx.Tx, tid uuid.UUID) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO site_settings (tenant_id, title, palette_id)
			VALUES ($1, 'کلینیک ب', 'ocean-trust')
			ON CONFLICT (tenant_id) DO UPDATE SET palette_id = 'ocean-trust'
		`, tid)
		return err
	})
	if err != nil {
		t.Fatalf("failed to seed Tenant B site_settings: %v", err)
	}

	// 3. Verify Tenant A reads its Stitch palette and cannot read Tenant B.
	err = store.WithTenantTx(ctxA, func(tx pgx.Tx, tid uuid.UUID) error {
		var palA string
		if err := tx.QueryRow(ctx, "SELECT palette_id FROM site_settings WHERE tenant_id = $1", tid).Scan(&palA); err != nil {
			return err
		}
		if palA != "stitch-editorial" {
			t.Fatalf("expected Tenant A palette stitch-editorial, got %q", palA)
		}

		// Cross-tenant read must return 0 rows
		var palB string
		err := tx.QueryRow(ctx, "SELECT palette_id FROM site_settings WHERE tenant_id = $1", tenantB).Scan(&palB)
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("expected ErrNoRows when Tenant A reads Tenant B, got %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Tenant A isolation check failed: %v", err)
	}

	// 4. Verify Tenant B reads ocean-trust and cannot read Tenant A
	err = store.WithTenantTx(ctxB, func(tx pgx.Tx, tid uuid.UUID) error {
		var palB string
		if err := tx.QueryRow(ctx, "SELECT palette_id FROM site_settings WHERE tenant_id = $1", tid).Scan(&palB); err != nil {
			return err
		}
		if palB != "ocean-trust" {
			t.Fatalf("expected Tenant B palette ocean-trust, got %q", palB)
		}

		// Cross-tenant read must return 0 rows
		var palA string
		err := tx.QueryRow(ctx, "SELECT palette_id FROM site_settings WHERE tenant_id = $1", tenantA).Scan(&palA)
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("expected ErrNoRows when Tenant B reads Tenant A, got %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Tenant B isolation check failed: %v", err)
	}

	// 5. Cross-tenant mutation attacks:
	// Tenant B attempts to UPDATE Tenant A's palette_id
	err = store.WithTenantTx(ctxB, func(tx pgx.Tx, _ uuid.UUID) error {
		tag, err := tx.Exec(ctx, "UPDATE site_settings SET palette_id = 'warm-sand' WHERE tenant_id = $1", tenantA)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 0 {
			t.Fatalf("SECURITY VIOLATION: Tenant B updated Tenant A row! rows=%d", tag.RowsAffected())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("cross-tenant update attack failed with unexpected error: %v", err)
	}

	// Tenant B attempts to INSERT a site_settings row for Tenant A
	err = store.WithTenantTx(ctxB, func(tx pgx.Tx, _ uuid.UUID) error {
		_, err := tx.Exec(ctx, "INSERT INTO site_settings (tenant_id, title, palette_id) VALUES ($1, 'evil', 'royal-plum')", tenantA)
		return err
	})
	if err == nil {
		t.Fatalf("expected RLS error when Tenant B inserts row for Tenant A, got nil")
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code != "42501" {
		t.Fatalf("expected PostgreSQL error 42501 (insufficient_privilege/RLS violation), got: %s", pgErr.Code)
	}

	// 6. Check constraint validation: invalid palette fails with 23514
	err = store.WithTenantTx(ctxA, func(tx pgx.Tx, tid uuid.UUID) error {
		_, err := tx.Exec(ctx, "UPDATE site_settings SET palette_id = 'neon-invalid' WHERE tenant_id = $1", tid)
		return err
	})
	if err == nil {
		t.Fatalf("expected CHECK constraint violation for invalid palette, got nil")
	}
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("expected PostgreSQL error 23514 (check_violation), got: %v", err)
	}
}

// TestSiteSettings_SeparatedQueriesAndOutboxRollback verifies PSY-6.5.1:
// 1. Separate sqlc queries for profile (content) and appearance.
// 2. Modifying appearance does not alter profile fields (title, description, phone, address, primary_color).
// 3. Modifying profile does not alter appearance fields (template_id, palette_id, radius, scale).
// 4. Concurrent execution of profile and appearance updates preserves both without clobbering.
// 5. Transactional rollback when Outbox cache invalidation insertion fails.
func TestSiteSettings_SeparatedQueriesAndOutboxRollback(t *testing.T) {
	ctx := context.Background()
	pool := dbfixture.Connect(t, dbfixture.AppDSN, "app_user")

	// Seed two test clinics (Tenant A and Tenant B)
	tenantA := uuid.New()
	tenantB := uuid.New()
	slugA := "sep-tenant-a-" + tenantA.String()[:8]
	slugB := "sep-tenant-b-" + tenantB.String()[:8]
	hostA := slugA + ".psychology.ir"
	hostB := slugB + ".psychology.ir"

	_, err := pool.Exec(ctx, `
		INSERT INTO tenants (id, name, slug, status)
		VALUES ($1, $2, $3, 'active'), ($4, $5, $6, 'active')
	`, tenantA, "کلینیک الف", slugA, tenantB, "کلینیک ب", slugB)
	if err != nil {
		t.Fatalf("failed to insert test tenants: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM tenants WHERE id IN ($1, $2)", tenantA, tenantB)
	}()

	store := db.NewStore(pool)
	svc := settings.NewServiceWithStore(store)
	ctxA := tenantctx.With(ctx, tenantA)
	ctxB := tenantctx.With(ctx, tenantB)

	// Seed verified domain for Tenant A and Tenant B
	err = store.WithTenantTx(ctxA, func(tx pgx.Tx, tid uuid.UUID) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO domains (id, tenant_id, host, is_primary, is_verified, verified_at)
			VALUES ($1, $2, $3, true, true, now())
		`, uuid.New(), tid, hostA)
		return err
	})
	if err != nil {
		t.Fatalf("failed to seed domain for Tenant A: %v", err)
	}

	err = store.WithTenantTx(ctxB, func(tx pgx.Tx, tid uuid.UUID) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO domains (id, tenant_id, host, is_primary, is_verified, verified_at)
			VALUES ($1, $2, $3, true, true, now())
		`, uuid.New(), tid, hostB)
		return err
	})
	if err != nil {
		t.Fatalf("failed to seed domain for Tenant B: %v", err)
	}

	// 1. Initial setup using monolithic/legacy UpdateSettings:
	initTitle := "کلینیک امید الف"
	initPhone := "02111111"
	initAddress := "تهران، خیابان ولیعصر"
	initColor := "#0284c7"
	initTpl := "minimal"
	initPalette := "calm-sage"
	initRadius := "md"
	initScale := "md"

	_, err = svc.UpdateSettings(ctxA, tenantA, settings.UpdateSettingsParams{
		Title:        initTitle,
		Phone:        initPhone,
		Address:      initAddress,
		PrimaryColor: initColor,
		TemplateID:   initTpl,
		PaletteID:    initPalette,
		ThemeRadius:  initRadius,
		ThemeScale:   initScale,
	})
	if err != nil {
		t.Fatalf("failed to initialize settings: %v", err)
	}

	// 2. Test Appearance Update does NOT alter Profile fields:
	appRes, err := svc.UpdateAppearance(ctxA, tenantA, settings.UpdateAppearanceParams{
		TemplateID:  "mobile-app",
		PaletteID:   "ocean-trust",
		ThemeRadius: "lg",
		ThemeScale:  "sm",
	})
	if err != nil {
		t.Fatalf("UpdateAppearance failed: %v", err)
	}
	if appRes.TemplateID != "mobile-app" || appRes.PaletteID != "ocean-trust" || appRes.ThemeRadius != "lg" || appRes.ThemeScale != "sm" {
		t.Fatalf("unexpected appearance response: %+v", appRes)
	}

	// Verify profile fields remain untouched in DB
	curSettings, err := svc.GetSettings(ctxA, tenantA)
	if err != nil {
		t.Fatalf("GetSettings failed: %v", err)
	}
	if curSettings.Title != initTitle || curSettings.Phone != initPhone || curSettings.Address != initAddress || curSettings.PrimaryColor != initColor {
		t.Fatalf("CRITICAL REGRESSION: UpdateAppearance altered profile fields! got title=%q, phone=%q, address=%q, color=%q",
			curSettings.Title, curSettings.Phone, curSettings.Address, curSettings.PrimaryColor)
	}
	if curSettings.TemplateID != "mobile-app" || curSettings.PaletteID != "ocean-trust" || curSettings.ThemeRadius != "lg" || curSettings.ThemeScale != "sm" {
		t.Fatalf("appearance fields not updated correctly in GetSettings: %+v", curSettings)
	}

	// 3. Test Profile Update does NOT alter Appearance fields:
	newTitle := "کلینیک تخصصی روان درمانی امید"
	newPhone := "02188888"
	newAddress := "تهران، میدان ونک"
	newColor := "#10b981"
	profRes, err := svc.UpdateProfile(ctxA, tenantA, settings.UpdateProfileParams{
		Title:        newTitle,
		Phone:        newPhone,
		Address:      newAddress,
		PrimaryColor: newColor,
	})
	if err != nil {
		t.Fatalf("UpdateProfile failed: %v", err)
	}
	if profRes.Title != newTitle || profRes.Phone != newPhone || profRes.Address != newAddress || profRes.PrimaryColor != newColor {
		t.Fatalf("unexpected profile response: %+v", profRes)
	}

	// Verify appearance fields remain untouched
	curAppearance, err := svc.GetAppearance(ctxA, tenantA)
	if err != nil {
		t.Fatalf("GetAppearance failed: %v", err)
	}
	if curAppearance.TemplateID != "mobile-app" || curAppearance.PaletteID != "ocean-trust" || curAppearance.ThemeRadius != "lg" || curAppearance.ThemeScale != "sm" {
		t.Fatalf("CRITICAL REGRESSION: UpdateProfile altered appearance fields! got template=%q, palette=%q, radius=%q, scale=%q",
			curAppearance.TemplateID, curAppearance.PaletteID, curAppearance.ThemeRadius, curAppearance.ThemeScale)
	}

	// 4. Test Concurrent updates of Profile and Appearance:
	var wg sync.WaitGroup
	var errProf, errApp error
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, errProf = svc.UpdateProfile(ctxA, tenantA, settings.UpdateProfileParams{
			Title:        "کلینیک امید همگام",
			Phone:        "02155555",
			Address:      "تهران، شهرک غرب",
			PrimaryColor: "#6366f1",
		})
	}()

	go func() {
		defer wg.Done()
		_, errApp = svc.UpdateAppearance(ctxA, tenantA, settings.UpdateAppearanceParams{
			TemplateID:  "clinical",
			PaletteID:   "warm-sand",
			ThemeRadius: "md",
			ThemeScale:  "md",
		})
	}()

	wg.Wait()
	if errProf != nil {
		t.Fatalf("concurrent UpdateProfile failed: %v", errProf)
	}
	if errApp != nil {
		t.Fatalf("concurrent UpdateAppearance failed: %v", errApp)
	}

	// Verify final state has BOTH concurrent updates merged cleanly
	finalSettings, err := svc.GetSettings(ctxA, tenantA)
	if err != nil {
		t.Fatalf("failed to get final settings: %v", err)
	}
	if finalSettings.Title != "کلینیک امید همگام" || finalSettings.Phone != "02155555" || finalSettings.PrimaryColor != "#6366f1" {
		t.Fatalf("concurrent update lost profile changes: %+v", finalSettings)
	}
	if finalSettings.TemplateID != "clinical" || finalSettings.PaletteID != "warm-sand" {
		t.Fatalf("concurrent update lost appearance changes: %+v", finalSettings)
	}

	// 5. Test Outbox Rollback on Appearance Update Failure:
	err = store.WithTenantTx(ctxA, func(tx pgx.Tx, tid uuid.UUID) error {
		q := settingsdb.New(tx)
		_, err := q.UpsertSiteAppearance(ctx, settingsdb.UpsertSiteAppearanceParams{
			TenantID:        toPgUUID(tid),
			TemplateID:      "mobile-app",
			TemplateVersion: "1.0.0",
			PaletteID:       "royal-plum",
			ThemeRadius:     "sm",
			ThemeScale:      "sm",
		})
		if err != nil {
			return err
		}
		// Simulate outbox failure
		return errors.New("simulated outbox failure during appearance update")
	})
	if err == nil {
		t.Fatalf("expected error from aborted transaction, got nil")
	}

	// Check DB: palette must STILL be 'warm-sand', NOT 'royal-plum'
	checkApp, err := svc.GetAppearance(ctxA, tenantA)
	if err != nil {
		t.Fatalf("GetAppearance failed: %v", err)
	}
	if checkApp.PaletteID == "royal-plum" {
		t.Fatalf("ROLLBACK FAILED: appearance retained 'royal-plum' after aborted transaction!")
	}
	if checkApp.PaletteID != "warm-sand" {
		t.Fatalf("unexpected palette after rollback: got %q, expected 'warm-sand'", checkApp.PaletteID)
	}

	// 6. Test Outbox Rollback on Profile Update Failure:
	err = store.WithTenantTx(ctxA, func(tx pgx.Tx, tid uuid.UUID) error {
		q := settingsdb.New(tx)
		_, err := q.UpsertSiteProfile(ctx, settingsdb.UpsertSiteProfileParams{
			TenantID:     toPgUUID(tid),
			Title:        "عنوان نامعتبر که باید رول‌بک شود",
			Description:  "توضیحات",
			Phone:        "02100000",
			Address:      "آدرس",
			PrimaryColor: "#111111",
		})
		if err != nil {
			return err
		}
		// Simulate outbox failure
		return errors.New("simulated outbox failure during profile update")
	})
	if err == nil {
		t.Fatalf("expected error from aborted transaction, got nil")
	}

	// Check DB: title must STILL be 'کلینیک امید همگام', NOT 'عنوان نامعتبر که باید رول‌بک شود'
	checkProf, err := svc.GetSettings(ctxA, tenantA)
	if err != nil {
		t.Fatalf("GetSettings failed: %v", err)
	}
	if checkProf.Title == "عنوان نامعتبر که باید رول‌بک شود" {
		t.Fatalf("ROLLBACK FAILED: profile retained aborted title after transaction rollback!")
	}
	if checkProf.Title != "کلینیک امید همگام" {
		t.Fatalf("unexpected title after rollback: got %q, expected 'کلینیک امید همگام'", checkProf.Title)
	}

	// 7. Verify cross-tenant isolation: Tenant B cannot read or alter Tenant A settings
	err = store.WithTenantTx(ctxB, func(tx pgx.Tx, _ uuid.UUID) error {
		tag, err := tx.Exec(ctx, "UPDATE site_settings SET palette_id = 'calm-sage' WHERE tenant_id = $1", tenantA)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 0 {
			t.Fatalf("SECURITY VIOLATION: Tenant B updated Tenant A row! rows=%d", tag.RowsAffected())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("cross-tenant update attack failed with unexpected error: %v", err)
	}
}

// TestAdminAppearance_LiveHttpGuardAndIsolation tests PSY-6.5.2:
// Connects GET and PUT /admin/site/appearance to session authentication, tenant membership guard,
// and APPEARANCE_OWNER_USER_ID verification in Go HTTP layer over real PostgreSQL with FORCE RLS.
func TestAdminAppearance_LiveHttpGuardAndIsolation(t *testing.T) {
	ctx := context.Background()
	pool := dbfixture.Connect(t, dbfixture.AppDSN, "app_user")

	tenantA := uuid.New()
	tenantB := uuid.New()
	ownerUser := uuid.New()
	regularUser := uuid.New()

	slugA := "live-app-a-" + tenantA.String()[:8]
	slugB := "live-app-b-" + tenantB.String()[:8]
	hostA := slugA + ".psychology.ir"
	hostB := slugB + ".psychology.ir"

	// Insert tenants
	_, err := pool.Exec(ctx, `
		INSERT INTO tenants (id, name, slug, status)
		VALUES ($1, $2, $3, 'active'), ($4, $5, $6, 'active')
	`, tenantA, "کلینیک الف", slugA, tenantB, "کلینیک ب", slugB)
	if err != nil {
		t.Fatalf("failed to insert test tenants: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM tenants WHERE id IN ($1, $2)", tenantA, tenantB)
		_, _ = pool.Exec(context.Background(), "DELETE FROM panel_identities WHERE id IN ($1, $2)", ownerUser, regularUser)
	}()

	// Insert panel identities
	_, err = pool.Exec(ctx, `
		INSERT INTO panel_identities (id, email, password_hash, full_name, status)
		VALUES ($1, $2, 'hash', 'مالک پلتفرم', 'active'), ($3, $4, 'hash', 'کاربر عادی', 'active')
	`, ownerUser, "platform-owner-"+ownerUser.String()[:8]+"@test.ir", regularUser, "regular-user-"+regularUser.String()[:8]+"@test.ir")
	if err != nil {
		t.Fatalf("failed to insert panel identities: %v", err)
	}

	// Insert sessions
	tokenOwner := "token-live-owner-" + uuid.NewString()
	tokenRegular := "token-live-regular-" + uuid.NewString()
	hashOwner := auth.HashToken(tokenOwner)
	hashRegular := auth.HashToken(tokenRegular)

	_, err = pool.Exec(ctx, `
		INSERT INTO panel_sessions (id, user_id, token_hash, expires_at)
		VALUES (gen_random_uuid(), $1, $2, now() + interval '1 day'),
		       (gen_random_uuid(), $3, $4, now() + interval '1 day')
	`, ownerUser, hashOwner, regularUser, hashRegular)
	if err != nil {
		t.Fatalf("failed to insert panel sessions: %v", err)
	}

	// Seed domain & membership for Tenant A (ownerUser is owner)
	store := db.NewStore(pool)
	ctxA := tenantctx.With(ctx, tenantA)
	err = store.WithTenantTx(ctxA, func(tx pgx.Tx, tid uuid.UUID) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO domains (id, tenant_id, host, is_primary, is_verified, verified_at)
			VALUES ($1, $2, $3, true, true, now())
		`, uuid.New(), tid, hostA); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO memberships (id, tenant_id, user_id, role)
			VALUES (gen_random_uuid(), $1, $2, 'owner')
		`, tid, ownerUser); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to seed domain/membership for Tenant A: %v", err)
	}

	// Seed domain & membership for Tenant B (regularUser is owner)
	ctxB := tenantctx.With(ctx, tenantB)
	err = store.WithTenantTx(ctxB, func(tx pgx.Tx, tid uuid.UUID) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO domains (id, tenant_id, host, is_primary, is_verified, verified_at)
			VALUES ($1, $2, $3, true, true, now())
		`, uuid.New(), tid, hostB); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO memberships (id, tenant_id, user_id, role)
			VALUES (gen_random_uuid(), $1, $2, 'owner')
		`, tid, regularUser); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to seed domain/membership for Tenant B: %v", err)
	}

	// Assemble handler with live services
	domainResolver := domain.NewResolver(domaindb.New(pool))
	authService := auth.NewServiceWithStore(authdb.New(pool), store)
	settingsService := settings.NewServiceWithStore(store)
	settingsHandler := settings.NewHandler(settingsService, domainResolver, authService)
	settingsHandler.SetAppearanceOwnerUserID(ownerUser)

	router := chi.NewRouter()
	settingsHandler.MountRoutes(router)

	// 1. GET /admin/site/appearance for Tenant A by ownerUser -> 200 OK with default appearance
	reqGet := httptest.NewRequest(http.MethodGet, "/admin/site/appearance", nil)
	reqGet.Header.Set("Authorization", "Bearer "+tokenOwner)
	reqGet.Header.Set("X-Tenant-ID", tenantA.String())
	recGet := httptest.NewRecorder()
	router.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 on live GET appearance, got %d: %s", recGet.Code, recGet.Body.String())
	}
	if recGet.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("expected Cache-Control: private, no-store, got %s", recGet.Header().Get("Cache-Control"))
	}

	// 2. PUT /admin/site/appearance for Tenant A by ownerUser -> 200 OK
	putBody, _ := json.Marshal(map[string]string{
		"template_id": "mobile-app",
		"palette_id":  "ocean-trust",
	})
	reqPut := httptest.NewRequest(http.MethodPut, "/admin/site/appearance", bytes.NewBuffer(putBody))
	reqPut.Header.Set("Authorization", "Bearer "+tokenOwner)
	reqPut.Header.Set("X-Tenant-ID", tenantA.String())
	reqPut.Header.Set("Content-Type", "application/json")
	recPut := httptest.NewRecorder()
	router.ServeHTTP(recPut, reqPut)
	if recPut.Code != http.StatusOK {
		t.Fatalf("expected 200 on live PUT appearance, got %d: %s", recPut.Code, recPut.Body.String())
	}

	// Verify Outbox event site.updated was committed for Tenant A (using WithTenantTx so RLS policy passes)
	var outboxCount int
	err = store.WithTenantTx(ctxA, func(tx pgx.Tx, tid uuid.UUID) error {
		return tx.QueryRow(ctx, "SELECT COUNT(*) FROM outbox_events WHERE tenant_id = $1 AND event_type = 'site.updated'", tid).Scan(&outboxCount)
	})
	if err != nil {
		t.Fatalf("failed to query outbox: %v", err)
	}
	if outboxCount == 0 {
		t.Fatalf("expected outbox event site.updated to be created for Tenant A")
	}

	// 3. Attack: regularUser (not appearance owner) attempts to access /admin/site/appearance -> 403 Forbidden
	reqAttack1 := httptest.NewRequest(http.MethodPut, "/admin/site/appearance", bytes.NewBuffer(putBody))
	reqAttack1.Header.Set("Authorization", "Bearer "+tokenRegular)
	reqAttack1.Header.Set("X-Tenant-ID", tenantB.String())
	reqAttack1.Header.Set("Content-Type", "application/json")
	recAttack1 := httptest.NewRecorder()
	router.ServeHTTP(recAttack1, reqAttack1)
	if recAttack1.Code != http.StatusForbidden {
		t.Fatalf("SECURITY BREACH: non-appearance owner obtained code %d instead of 403", recAttack1.Code)
	}

	// 4. Attack: ownerUser attempts to mutate Tenant B (no membership) -> 403 Forbidden
	reqAttack2 := httptest.NewRequest(http.MethodPut, "/admin/site/appearance", bytes.NewBuffer(putBody))
	reqAttack2.Header.Set("Authorization", "Bearer "+tokenOwner)
	reqAttack2.Header.Set("X-Tenant-ID", tenantB.String())
	reqAttack2.Header.Set("Content-Type", "application/json")
	recAttack2 := httptest.NewRecorder()
	router.ServeHTTP(recAttack2, reqAttack2)
	if recAttack2.Code != http.StatusForbidden {
		t.Fatalf("SECURITY BREACH: owner mutated Tenant B without membership, got %d instead of 403", recAttack2.Code)
	}

	// 5. CSRF Check: Sec-Fetch-Site cross-site -> 403 Forbidden
	reqCSRF := httptest.NewRequest(http.MethodPut, "/admin/site/appearance", bytes.NewBuffer(putBody))
	reqCSRF.Header.Set("Authorization", "Bearer "+tokenOwner)
	reqCSRF.Header.Set("X-Tenant-ID", tenantA.String())
	reqCSRF.Header.Set("Content-Type", "application/json")
	reqCSRF.Header.Set("Sec-Fetch-Site", "cross-site")
	recCSRF := httptest.NewRecorder()
	router.ServeHTTP(recCSRF, reqCSRF)
	if recCSRF.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on CSRF attack, got %d", recCSRF.Code)
	}

	// 6. Strict validation: sending template_version -> 400 Bad Request
	bodyWithVer, _ := json.Marshal(map[string]string{
		"template_id":      "mobile-app",
		"palette_id":       "ocean-trust",
		"template_version": "1.0.0",
	})
	reqBadField := httptest.NewRequest(http.MethodPut, "/admin/site/appearance", bytes.NewBuffer(bodyWithVer))
	reqBadField.Header.Set("Authorization", "Bearer "+tokenOwner)
	reqBadField.Header.Set("X-Tenant-ID", tenantA.String())
	reqBadField.Header.Set("Content-Type", "application/json")
	recBadField := httptest.NewRecorder()
	router.ServeHTTP(recBadField, reqBadField)
	if recBadField.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when sending template_version, got %d: %s", recBadField.Code, recBadField.Body.String())
	}
}
