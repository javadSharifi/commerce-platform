package settings_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"psychology/api/internal/platform/db"
	"psychology/api/internal/platform/tenantctx"
	"psychology/api/internal/settings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
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
// DATABASE_URL or TEST_APP_DATABASE_URL is provided, asserting runtime role isolation.
func TestSiteSettings_LiveTenantIsolation(t *testing.T) {
	dbURL := os.Getenv("TEST_APP_DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		t.Skip("skipping live database isolation test: TEST_APP_DATABASE_URL or DATABASE_URL not set")
		return
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live database isolation test (failed to connect: %v)", err)
		return
	}
	defer pool.Close()

	store := db.NewStore(pool)
	svc := settings.NewServiceWithStore(store)

	// Create two distinct tenant IDs
	tenantA := uuid.New()
	tenantB := uuid.New()

	ctxA := tenantctx.With(ctx, tenantA)
	ctxB := tenantctx.With(ctx, tenantB)

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
}
