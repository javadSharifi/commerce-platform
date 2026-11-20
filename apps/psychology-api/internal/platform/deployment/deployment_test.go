package deployment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func findInfrastructurePath(t *testing.T) string {
	candidates := []string{
		"../../../../../infrastructure/psychology",
		"../../../../infrastructure/psychology",
		"../../../infrastructure/psychology",
		"../../infrastructure/psychology",
		"infrastructure/psychology",
	}

	for _, c := range candidates {
		abs, err := filepath.Abs(c)
		if err == nil {
			if _, err := os.Stat(filepath.Join(abs, "docker-compose.yml")); err == nil {
				return abs
			}
		}
	}

	t.Fatalf("could not locate infrastructure/psychology directory")
	return ""
}

func TestDockerCompose_StructureAndServices(t *testing.T) {
	infraDir := findInfrastructurePath(t)
	composePath := filepath.Join(infraDir, "docker-compose.yml")

	data, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("failed to read docker-compose.yml: %v", err)
	}

	content := string(data)

	// Check required services
	requiredServices := []string{
		"postgres:",
		"api:",
		"admin:",
		"site:",
		"caddy:",
		"backup:",
	}

	for _, svc := range requiredServices {
		if !strings.Contains(content, svc) {
			t.Errorf("docker-compose.yml missing service %s", svc)
		}
	}

	// Check networks and volumes
	if !strings.Contains(content, "psychology_pg_data:") {
		t.Errorf("docker-compose.yml missing persistent postgres volume")
	}
	if !strings.Contains(content, "psychology_net:") {
		t.Errorf("docker-compose.yml missing custom network definition")
	}
}

func TestCaddyfile_CachingAndSecurityHeaders(t *testing.T) {
	infraDir := findInfrastructurePath(t)
	caddyPath := filepath.Join(infraDir, "Caddyfile")

	data, err := os.ReadFile(caddyPath)
	if err != nil {
		t.Fatalf("failed to read Caddyfile: %v", err)
	}

	content := string(data)

	// 1. Static asset immutable CDN caching
	if !strings.Contains(content, "public, max-age=31536000, immutable") {
		t.Errorf("Caddyfile missing immutable cache-control for static assets")
	}

	// 2. Private routes no-store policy
	if !strings.Contains(content, "no-store, no-cache, must-revalidate, private") {
		t.Errorf("Caddyfile missing no-store cache-control for private/dynamic routes")
	}

	// 3. Security headers
	requiredHeaders := []string{
		"X-Content-Type-Options nosniff",
		"X-Frame-Options DENY",
		"Referrer-Policy strict-origin-when-cross-origin",
	}
	for _, h := range requiredHeaders {
		if !strings.Contains(content, h) {
			t.Errorf("Caddyfile missing security header: %s", h)
		}
	}

	// 4. Host preservation for multi-tenancy
	if !strings.Contains(content, "header_up Host {host}") {
		t.Errorf("Caddyfile missing Host header preservation")
	}
}

func TestEnvExample_SeparateCredentials(t *testing.T) {
	infraDir := findInfrastructurePath(t)
	envPath := filepath.Join(infraDir, ".env.example")

	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("failed to read .env.example: %v", err)
	}

	content := string(data)

	requiredConfigs := []string{
		"POSTGRES_USER",
		"APP_DB_USER",
		"DATABASE_URL",
		"PARSPACK_S3_ENDPOINT",
		"PARSPACK_S3_BUCKET",
		"PARSPACK_ACCESS_KEY",
		"PARSPACK_SECRET_KEY",
		"ZARINPAL_MERCHANT_ID",
		"SESSION_SECRET",
	}

	for _, cfg := range requiredConfigs {
		if !strings.Contains(content, cfg) {
			t.Errorf(".env.example missing configuration parameter: %s", cfg)
		}
	}
}

func TestBackupAndRestoreScripts_Integrity(t *testing.T) {
	infraDir := findInfrastructurePath(t)

	backupPath := filepath.Join(infraDir, "scripts", "backup.sh")
	dataBkp, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backup.sh: %v", err)
	}
	contentBkp := string(dataBkp)

	if !strings.Contains(contentBkp, "pg_dump") || !strings.Contains(contentBkp, "-Fc") {
		t.Errorf("backup.sh does not execute pg_dump -Fc")
	}
	if !strings.Contains(contentBkp, "RETENTION_DAYS") {
		t.Errorf("backup.sh missing retention cleanup")
	}

	restorePath := filepath.Join(infraDir, "scripts", "restore.sh")
	dataRst, err := os.ReadFile(restorePath)
	if err != nil {
		t.Fatalf("failed to read restore.sh: %v", err)
	}
	contentRst := string(dataRst)

	if !strings.Contains(contentRst, "pg_restore") {
		t.Errorf("restore.sh does not execute pg_restore")
	}
	if !strings.Contains(contentRst, "relrowsecurity = true") || !strings.Contains(contentRst, "relforcerowsecurity = true") {
		t.Errorf("restore.sh does not verify forced RLS enforcement post-restore")
	}
}
