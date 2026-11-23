package migrations_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

		// Security Checks on upContent
		if strings.Contains(upContent, "tenant_id") {
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
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("skipping live database test: DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live migration test (database connection failed: %v)", err)
		return
	}
	defer conn.Close(ctx)

	schemaName := "test_mig_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+schemaName+"; SET search_path TO "+schemaName+", public;"); err != nil {
		t.Skipf("skipping live migration test (failed to create test schema: %v)", err)
		return
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schemaName+" CASCADE;")
	}()

	files, err := filepath.Glob("*.up.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("no up migration files found")
	}

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

		// 1. UP
		if _, err := conn.Exec(ctx, string(upContent)); err != nil {
			t.Fatalf("UP failed on %s: %v", upFile, err)
		}

		// 2. DOWN
		if _, err := conn.Exec(ctx, string(downContent)); err != nil {
			t.Fatalf("DOWN failed on %s: %v", downFile, err)
		}

		// 3. UP again
		if _, err := conn.Exec(ctx, string(upContent)); err != nil {
			t.Fatalf("UP again failed on %s: %v", upFile, err)
		}
	}
}
