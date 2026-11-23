// Package dbfixture guards connections made by the disposable PostgreSQL test lane.
package dbfixture

import (
	"context"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	AdminDSN     = "TEST_ADMIN_DATABASE_URL"
	AppDSN       = "TEST_APP_DATABASE_URL"
	WorkerDSN    = "TEST_WORKER_DATABASE_URL"
	MigrationDSN = "TEST_MIGRATION_DATABASE_URL"
)

var databaseName = regexp.MustCompile(`^psy_phase1_[0-9a-f]{24}$`)

// Connect checks the DSN and confirms the server-side fixture marker and role
// before callers can write. Unconfigured ordinary unit runs remain possible.
func Connect(t *testing.T, envKey, expectedRole string) *pgxpool.Pool {
	t.Helper()
	return connect(t, envKey, expectedRole, "PSYCHOLOGY_TEST_DB_NAME", true)
}

// ConnectMigration verifies the separately provisioned database before its first migration.
func ConnectMigration(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if name := os.Getenv("PSYCHOLOGY_TEST_MIGRATION_DB_NAME"); name != "" && name == os.Getenv("PSYCHOLOGY_TEST_DB_NAME") {
		t.Fatal("migration database must differ from the runtime fixture")
	}
	return connect(t, MigrationDSN, "phase1_admin", "PSYCHOLOGY_TEST_MIGRATION_DB_NAME", false)
}

func connect(t *testing.T, envKey, expectedRole, nameKey string, requireMigrated bool) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv(envKey)
	if dsn == "" && os.Getenv("PSYCHOLOGY_TEST_DB_REQUIRED") != "1" {
		t.Skipf("disposable PostgreSQL test requires %s; run make test-db", envKey)
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme != "postgres" || u.Hostname() != "127.0.0.1" || u.Port() == "" ||
		u.User == nil || u.User.Username() != expectedRole || u.RawQuery != "sslmode=disable" || u.Fragment != "" ||
		!databaseName.MatchString(strings.TrimPrefix(u.Path, "/")) {
		t.Fatalf("%s must identify a loopback Phase 1 fixture as %s", envKey, expectedRole)
	}
	name := u.Path[1:]
	if name != os.Getenv(nameKey) {
		t.Fatalf("%s does not match %s", envKey, nameKey)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("cannot parse Phase 1 connection from %s", envKey)
	}
	t.Cleanup(pool.Close)
	var identityOK, forceRLS bool
	err = pool.QueryRow(ctx, `
		SELECT current_database() = $1 AND current_user = $2 AND session_user = $2
		       AND coalesce(shobj_description(d.oid, 'pg_database'), '') = 'psychology-phase1:' || $1
		       AND r.rolsuper = $3 AND ($3 OR NOT r.rolbypassrls),
		       (SELECT count(*) = 2 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		        WHERE n.nspname = 'public' AND c.relname IN ('site_settings', 'outbox_events')
		        AND c.relrowsecurity AND c.relforcerowsecurity)
		FROM pg_database d CROSS JOIN pg_roles r
		WHERE d.datname = current_database() AND r.rolname = current_user`,
		name, expectedRole, expectedRole == "phase1_admin").Scan(&identityOK, &forceRLS)
	if err != nil || !identityOK || (requireMigrated && !forceRLS) {
		t.Fatalf("%s failed fixture identity, role, or forced RLS checks", envKey)
	}
	t.Logf("verified disposable database %s, role %s, core forced RLS present=%t", name, expectedRole, forceRLS)
	return pool
}
