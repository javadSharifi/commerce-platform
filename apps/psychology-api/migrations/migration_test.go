package migrations_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"psychology/api/internal/testsupport/dbfixture"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMigrationFilesPairingAndIntegrity(t *testing.T) {
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}

	upFiles := make(map[string]string)
	downFiles := make(map[string]string)

	for _, f := range files {
		name := f.Name()
		if strings.HasSuffix(name, ".up.sql") {
			base := strings.TrimSuffix(name, ".up.sql")
			content, err := os.ReadFile(name)
			if err != nil {
				t.Fatalf("failed to read %s: %v", name, err)
			}
			upFiles[base] = string(content)
		} else if strings.HasSuffix(name, ".down.sql") {
			base := strings.TrimSuffix(name, ".down.sql")
			content, err := os.ReadFile(name)
			if err != nil {
				t.Fatalf("failed to read %s: %v", name, err)
			}
			downFiles[base] = string(content)
		}
	}

	if len(upFiles) == 0 {
		t.Fatal("expected at least one migration file")
	}

	for base, upContent := range upFiles {
		downContent, ok := downFiles[base]
		if !ok {
			t.Errorf("missing matching down migration for %s.up.sql", base)
			continue
		}

		// Existing-table metadata and foreign-key changes can mention tenant_id
		// without declaring a new table or replacing its RLS policy.
		definesTenantScope := strings.Contains(upContent, "CREATE TABLE") || strings.Contains(upContent, "CREATE POLICY")
		if strings.Contains(upContent, "tenant_id") && definesTenantScope {
			if !strings.Contains(upContent, "ENABLE ROW LEVEL SECURITY") {
				t.Errorf("migration %s contains tenant_id but missing ENABLE ROW LEVEL SECURITY", base)
			}
			if !strings.Contains(upContent, "FORCE  ROW LEVEL SECURITY") && !strings.Contains(upContent, "FORCE ROW LEVEL SECURITY") {
				t.Errorf("migration %s contains tenant_id but missing FORCE ROW LEVEL SECURITY", base)
			}
			if !strings.Contains(upContent, "current_setting('app.tenant_id', true)") {
				t.Errorf("migration %s missing current_setting('app.tenant_id', true)", base)
			}
		}
		for _, migration := range []string{upContent, downContent} {
			if strings.Contains(migration, "DISABLE ROW LEVEL SECURITY") || strings.Contains(migration, "NO FORCE ROW LEVEL SECURITY") {
				t.Errorf("migration %s disables tenant RLS", base)
			}
		}

		if len(strings.TrimSpace(downContent)) == 0 {
			t.Errorf("down migration for %s is empty", base)
		}
	}
}

func TestAppointmentTablesRLSAndConstraints(t *testing.T) {
	upContentBytes, err := os.ReadFile("000005_create_appointments.up.sql")
	if err != nil {
		t.Fatalf("failed to read 000005_create_appointments.up.sql: %v", err)
	}
	upContent := string(upContentBytes)

	requiredTables := []string{
		"therapists",
		"therapist_working_hours",
		"therapist_schedule_exceptions",
		"appointment_slots",
		"appointments",
		"appointment_status_history",
	}

	for _, tbl := range requiredTables {
		if !strings.Contains(upContent, "CREATE TABLE "+tbl) {
			t.Errorf("expected 000005 to create table %s", tbl)
		}
		if !strings.Contains(upContent, "ALTER TABLE "+tbl+" ENABLE ROW LEVEL SECURITY;") {
			t.Errorf("table %s missing ENABLE ROW LEVEL SECURITY", tbl)
		}
		if !strings.Contains(upContent, "ALTER TABLE "+tbl+" FORCE  ROW LEVEL SECURITY;") &&
			!strings.Contains(upContent, "ALTER TABLE "+tbl+" FORCE ROW LEVEL SECURITY;") {
			t.Errorf("table %s missing FORCE ROW LEVEL SECURITY", tbl)
		}
	}

	// Composite FKs
	expectedFKs := []string{
		"FOREIGN KEY (tenant_id, therapist_id) REFERENCES therapists(tenant_id, id)",
		"FOREIGN KEY (tenant_id, slot_id) REFERENCES appointment_slots(tenant_id, id)",
		"FOREIGN KEY (tenant_id, appointment_id) REFERENCES appointments(tenant_id, id)",
	}

	for _, fk := range expectedFKs {
		if !strings.Contains(upContent, fk) {
			t.Errorf("missing composite foreign key: %s", fk)
		}
	}

	// Active slot uniqueness
	if !strings.Contains(upContent, "appointments_active_slot_uq") {
		t.Errorf("missing appointments_active_slot_uq index")
	}
}

func TestCourseTablesRLSAndConstraints(t *testing.T) {
	upContentBytes, err := os.ReadFile("000006_create_courses.up.sql")
	if err != nil {
		t.Fatalf("failed to read 000006_create_courses.up.sql: %v", err)
	}
	upContent := string(upContentBytes)

	requiredTables := []string{
		"buyer_accounts",
		"buyer_sessions",
		"buyer_otp_codes",
		"courses",
		"course_lessons",
		"course_entitlements",
	}

	for _, tbl := range requiredTables {
		if !strings.Contains(upContent, "CREATE TABLE "+tbl) {
			t.Errorf("expected 000006 to create table %s", tbl)
		}
		if !strings.Contains(upContent, "ALTER TABLE "+tbl+" ENABLE ROW LEVEL SECURITY;") {
			t.Errorf("table %s missing ENABLE ROW LEVEL SECURITY", tbl)
		}
		if !strings.Contains(upContent, "ALTER TABLE "+tbl+" FORCE  ROW LEVEL SECURITY;") &&
			!strings.Contains(upContent, "ALTER TABLE "+tbl+" FORCE ROW LEVEL SECURITY;") {
			t.Errorf("table %s missing FORCE ROW LEVEL SECURITY", tbl)
		}
	}

	// Composite FKs
	expectedFKs := []string{
		"FOREIGN KEY (tenant_id, buyer_id) REFERENCES buyer_accounts(tenant_id, id)",
		"FOREIGN KEY (tenant_id, course_id) REFERENCES courses(tenant_id, id)",
	}

	for _, fk := range expectedFKs {
		if !strings.Contains(upContent, fk) {
			t.Errorf("missing composite foreign key: %s", fk)
		}
	}

	// Active entitlement uniqueness
	if !strings.Contains(upContent, "course_entitlements_active_uq") {
		t.Errorf("missing course_entitlements_active_uq index")
	}
}

func TestOutboxEventsTableRLSAndConstraints(t *testing.T) {
	upContentBytes, err := os.ReadFile("000007_create_outbox_events.up.sql")
	if err != nil {
		t.Fatalf("failed to read 000007_create_outbox_events.up.sql: %v", err)
	}
	upContent := string(upContentBytes)

	if !strings.Contains(upContent, "CREATE TABLE IF NOT EXISTS outbox_events") {
		t.Errorf("expected 000007 to create table outbox_events")
	}
	if !strings.Contains(upContent, "ALTER TABLE outbox_events ENABLE ROW LEVEL SECURITY;") {
		t.Errorf("table outbox_events missing ENABLE ROW LEVEL SECURITY")
	}
	if !strings.Contains(upContent, "ALTER TABLE outbox_events FORCE ROW LEVEL SECURITY;") {
		t.Errorf("table outbox_events missing FORCE ROW LEVEL SECURITY")
	}
	if !strings.Contains(upContent, "current_setting('app.tenant_id', true)") {
		t.Errorf("table outbox_events missing current_setting('app.tenant_id', true)")
	}
	if !strings.Contains(upContent, "idx_outbox_events_pending") {
		t.Errorf("table outbox_events missing idx_outbox_events_pending")
	}
}

func TestSiteTemplateSettingsMigration(t *testing.T) {
	upContentBytes, err := os.ReadFile("000008_add_site_template_settings.up.sql")
	if err != nil {
		t.Fatalf("failed to read 000008_add_site_template_settings.up.sql: %v", err)
	}
	upContent := string(upContentBytes)

	requiredColumns := []string{
		"template_id",
		"template_version",
		"theme_radius",
		"theme_scale",
	}

	for _, col := range requiredColumns {
		if !strings.Contains(upContent, col) {
			t.Errorf("expected 000008 to add column %s", col)
		}
	}

	if !strings.Contains(upContent, "CHECK (theme_radius IN ('sm', 'md', 'lg'))") {
		t.Errorf("000008 missing CHECK constraint for theme_radius")
	}
	if !strings.Contains(upContent, "CHECK (theme_scale IN ('sm', 'md', 'lg'))") {
		t.Errorf("000008 missing CHECK constraint for theme_scale")
	}
	if !strings.Contains(upContent, "DEFAULT 'minimal'") {
		t.Errorf("000008 missing DEFAULT 'minimal' for template_id")
	}
	if !strings.Contains(upContent, "DEFAULT '1.0.0'") {
		t.Errorf("000008 missing DEFAULT '1.0.0' for template_version")
	}
}

func TestDomainLookupPolicyMigration(t *testing.T) {
	upContentBytes, err := os.ReadFile("000009_domain_lookup_policy.up.sql")
	if err != nil {
		t.Fatalf("failed to read 000009_domain_lookup_policy.up.sql: %v", err)
	}
	upContent := string(upContentBytes)

	if !strings.Contains(upContent, "CREATE POLICY domain_lookup ON domains") {
		t.Errorf("expected 000009 to create policy domain_lookup on domains")
	}
	if !strings.Contains(upContent, "FOR SELECT") {
		t.Errorf("000009 missing FOR SELECT")
	}
	if !strings.Contains(upContent, "is_verified = true") {
		t.Errorf("000009 missing is_verified = true condition")
	}

	downContentBytes, err := os.ReadFile("000009_domain_lookup_policy.down.sql")
	if err != nil {
		t.Fatalf("failed to read 000009_domain_lookup_policy.down.sql: %v", err)
	}
	downContent := string(downContentBytes)
	if !strings.Contains(downContent, "DROP POLICY IF EXISTS domain_lookup ON domains") {
		t.Errorf("000009 down missing DROP POLICY IF EXISTS domain_lookup ON domains")
	}
}

func TestOutboxWorkerPermissionsMigration(t *testing.T) {
	upContentBytes, err := os.ReadFile("000010_outbox_worker_permissions.up.sql")
	if err != nil {
		t.Fatalf("failed to read 000010_outbox_worker_permissions.up.sql: %v", err)
	}
	upContent := string(upContentBytes)

	if !strings.Contains(upContent, "worker_user") {
		t.Errorf("000010 missing worker_user")
	}
	if !strings.Contains(upContent, "GRANT SELECT, UPDATE (status, retry_count, last_error, processed_at) ON outbox_events TO worker_user") {
		t.Errorf("000010 missing column-level update grant for worker_user")
	}
	if !strings.Contains(upContent, "CREATE POLICY outbox_worker_policy ON outbox_events") {
		t.Errorf("000010 missing outbox_worker_policy")
	}
	if !strings.Contains(upContent, "CREATE POLICY outbox_tenant_isolation ON outbox_events") {
		t.Errorf("000010 missing outbox_tenant_isolation")
	}

	downContentBytes, err := os.ReadFile("000010_outbox_worker_permissions.down.sql")
	if err != nil {
		t.Fatalf("failed to read 000010_outbox_worker_permissions.down.sql: %v", err)
	}
	downContent := string(downContentBytes)
	if !strings.Contains(downContent, "DROP POLICY IF EXISTS outbox_worker_policy ON outbox_events") {
		t.Errorf("000010 down missing drop outbox_worker_policy")
	}
}

func TestPaletteIdMigration(t *testing.T) {
	upContentBytes, err := os.ReadFile("000011_add_palette_id_to_site_settings.up.sql")
	if err != nil {
		t.Fatalf("failed to read 000011_add_palette_id_to_site_settings.up.sql: %v", err)
	}
	upContent := string(upContentBytes)

	if !strings.Contains(upContent, "ADD COLUMN palette_id text NOT NULL DEFAULT 'calm-sage'") {
		t.Errorf("000011 missing ADD COLUMN palette_id text NOT NULL DEFAULT 'calm-sage'")
	}
	if !strings.Contains(upContent, "CHECK (palette_id IN ('calm-sage', 'ocean-trust', 'warm-sand', 'royal-plum'))") {
		t.Errorf("000011 missing CHECK constraint for palette_id")
	}

	downContentBytes, err := os.ReadFile("000011_add_palette_id_to_site_settings.down.sql")
	if err != nil {
		t.Fatalf("failed to read 000011_add_palette_id_to_site_settings.down.sql: %v", err)
	}
	downContent := string(downContentBytes)

	if !strings.Contains(downContent, "DROP COLUMN IF EXISTS palette_id") {
		t.Errorf("000011 down missing DROP COLUMN IF EXISTS palette_id")
	}
}

func TestMigrationUpDownUpLive(t *testing.T) {
	pool := dbfixture.ConnectMigration(t)
	runtime := dbfixture.Connect(t, dbfixture.AdminDSN, "phase1_admin")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()

	var database, schema string
	if err := conn.QueryRow(ctx, "SELECT current_database(), current_schema()").Scan(&database, &schema); err != nil {
		t.Fatal(err)
	}
	if schema != "public" || database == os.Getenv("PSYCHOLOGY_TEST_DB_NAME") {
		t.Fatalf("migration round trip requires public in its own disposable database, got %s.%s", database, schema)
	}
	assertPublicCoreRemoved(t, ctx, conn.Conn())
	var runtimeSettingsOID string
	if err := runtime.QueryRow(ctx, `SELECT 'public.site_settings'::regclass::oid::text`).Scan(&runtimeSettingsOID); err != nil {
		t.Fatal(err)
	}

	files, err := filepath.Glob("*.up.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("no up migration files found")
	}

	type migration struct{ base, up, down string }
	migrations := make([]migration, 0, len(files))
	for _, upFile := range files {
		base := strings.TrimSuffix(upFile, ".up.sql")
		downFile := base + ".down.sql"

		upContent, err := os.ReadFile(upFile)
		if err != nil {
			t.Fatalf("failed to read %s: %v", upFile, err)
		}
		downContent, err := os.ReadFile(downFile)
		if err != nil {
			t.Fatalf("failed to read %s: %v", downFile, err)
		}
		migrations = append(migrations, migration{base, string(upContent), string(downContent)})
	}

	apply := func(description, sql string) {
		t.Helper()
		if _, err := conn.Exec(ctx, sql); err != nil {
			t.Fatalf("%s failed: %v", description, err)
		}
	}
	for _, m := range migrations {
		apply(m.base+" up", m.up)
		apply(m.base+" down", m.down)
		if m.base == "000010_outbox_worker_permissions" {
			var reverted bool
			err := conn.QueryRow(ctx, `SELECT
				NOT has_table_privilege('worker_user', 'public.outbox_events', 'SELECT') AND
				NOT has_column_privilege('worker_user', 'public.outbox_events', 'status', 'UPDATE') AND
				EXISTS (SELECT FROM pg_policies WHERE schemaname = 'public' AND tablename = 'outbox_events'
					AND policyname = 'tenant_isolation_outbox_events') AND
				NOT EXISTS (SELECT FROM pg_policies WHERE schemaname = 'public' AND tablename = 'outbox_events'
					AND policyname IN ('outbox_tenant_isolation', 'outbox_worker_policy'))`).Scan(&reverted)
			if err != nil || !reverted {
				t.Fatalf("outbox rollback did not restore the original RLS policy and revoke worker access: %v", err)
			}
		}
		apply(m.base+" reapply", m.up)
	}
	assertPublicRuntimePermissions(t, ctx, conn.Conn())
	for i := len(migrations) - 1; i >= 0; i-- {
		m := migrations[i]
		apply(m.base+" full rollback", m.down)
	}
	assertPublicCoreRemoved(t, ctx, conn.Conn())
	for _, m := range migrations {
		apply(m.base+" full reapply", m.up)
	}
	assertPublicRuntimePermissions(t, ctx, conn.Conn())
	assertPublicTenantIsolation(t, ctx, conn.Conn())
	var afterOID string
	if err := runtime.QueryRow(ctx, `SELECT 'public.site_settings'::regclass::oid::text`).Scan(&afterOID); err != nil || afterOID != runtimeSettingsOID {
		t.Fatalf("runtime public schema changed during migration round trip: %v", err)
	}

	for _, m := range migrations {
		if m.base == "000012_allow_stitch_editorial_palette" {
			const palette = "stitch-editorial"
			var tenantID string
			err := conn.QueryRow(ctx, `
				WITH fixture AS (
					INSERT INTO tenants (name, slug) VALUES ('Rollback fixture', 'rollback-stitch') RETURNING id
				)
				INSERT INTO site_settings (tenant_id, palette_id) SELECT id, $1 FROM fixture
				RETURNING tenant_id::text
			`, palette).Scan(&tenantID)
			if err != nil {
				t.Fatalf("store Stitch palette after reapplying migration: %v", err)
			}
			var constraintErr *pgconn.PgError
			if _, err := conn.Exec(ctx, m.down); !errors.As(err, &constraintErr) || constraintErr.Code != "23514" {
				t.Fatalf("rollback should reject an existing Stitch palette, got %v", err)
			}
			var saved string
			if err := conn.QueryRow(ctx, "SELECT palette_id FROM site_settings WHERE tenant_id = $1", tenantID).Scan(&saved); err != nil {
				t.Fatal(err)
			}
			if saved != palette {
				t.Fatalf("failed rollback changed the saved palette to %q", saved)
			}
		}
	}
}

func assertPublicCoreRemoved(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()
	var empty bool
	err := conn.QueryRow(ctx, `SELECT to_regclass('public.tenants') IS NULL AND
		to_regclass('public.site_settings') IS NULL AND to_regclass('public.outbox_events') IS NULL`).Scan(&empty)
	if err != nil || !empty {
		t.Fatalf("expected clean public schema before reapplication: %v", err)
	}
}

func assertPublicRuntimePermissions(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()
	var forcedRLS, policies, grants bool
	err := conn.QueryRow(ctx, `SELECT
		(SELECT count(*) = 3 FROM pg_class WHERE oid IN ('public.site_settings'::regclass,
			'public.memberships'::regclass, 'public.outbox_events'::regclass) AND relrowsecurity AND relforcerowsecurity),
		(SELECT count(*) = 4 FROM pg_policies WHERE schemaname = 'public' AND
			(tablename IN ('site_settings', 'memberships') AND policyname = 'tenant_isolation' OR
			 tablename = 'outbox_events' AND (policyname = 'outbox_tenant_isolation' AND roles = '{app_user}' OR
			 policyname = 'outbox_worker_policy' AND roles = '{worker_user}'))),
		has_schema_privilege('app_user', 'public', 'USAGE') AND
		has_table_privilege('app_user', 'public.site_settings', 'SELECT') AND
		has_table_privilege('app_user', 'public.site_settings', 'UPDATE') AND
		has_table_privilege('app_user', 'public.outbox_events', 'INSERT') AND
		NOT has_table_privilege('app_user', 'public.outbox_events', 'DELETE') AND
		has_schema_privilege('worker_user', 'public', 'USAGE') AND
		has_table_privilege('worker_user', 'public.outbox_events', 'SELECT') AND
		has_column_privilege('worker_user', 'public.outbox_events', 'status', 'UPDATE') AND
		NOT has_column_privilege('worker_user', 'public.outbox_events', 'tenant_id', 'UPDATE') AND
		NOT has_table_privilege('worker_user', 'public.outbox_events', 'INSERT') AND
		NOT has_table_privilege('worker_user', 'public.site_settings', 'SELECT')`).Scan(&forcedRLS, &policies, &grants)
	if err != nil || !forcedRLS || !policies || !grants {
		t.Fatalf("public migration state: forced RLS=%t policies=%t grants=%t: %v", forcedRLS, policies, grants, err)
	}
}

func assertPublicTenantIsolation(t *testing.T, ctx context.Context, conn *pgx.Conn) {
	t.Helper()
	err := pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		var tenantA string
		if err := tx.QueryRow(ctx, `WITH seeded AS (
			INSERT INTO public.tenants (name, slug) VALUES ('Migration A', 'migration-a'), ('Migration B', 'migration-b')
			RETURNING id, slug
		), settings AS (
			INSERT INTO public.site_settings (tenant_id, title) SELECT id, slug FROM seeded
		), outbox AS (
			INSERT INTO public.outbox_events (tenant_id, event_type) SELECT id, 'site.updated' FROM seeded
		)
		SELECT id::text FROM seeded WHERE slug = 'migration-a'`).Scan(&tenantA); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SET LOCAL ROLE app_user`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantA); err != nil {
			return err
		}
		var isolated bool
		err := tx.QueryRow(ctx, `SELECT
			(SELECT count(*) = 1 AND bool_and(tenant_id::text = $1) FROM public.site_settings) AND
			(SELECT count(*) = 1 AND bool_and(tenant_id::text = $1) FROM public.outbox_events)`, tenantA).Scan(&isolated)
		if err != nil {
			return err
		}
		if !isolated {
			return errors.New("app_user cannot see exactly its own settings and outbox rows")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reapplied public RLS isolation: %v", err)
	}
}
