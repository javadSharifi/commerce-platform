package migrations_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestMigrationUpDownUpLive(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("skipping live database test: DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping live migration test (database connection failed: %v)", err)
		return
	}
	defer conn.Close(ctx)

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
