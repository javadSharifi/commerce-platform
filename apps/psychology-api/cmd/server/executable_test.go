package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"psychology/api/internal/testsupport/dbfixture"

	"github.com/google/uuid"
)

func startFixtureServer(t *testing.T, appearanceOwnerID string) (string, func(http.ResponseWriter, *http.Request)) {
	t.Helper()
	dir := t.TempDir()
	binary := filepath.Join(dir, "psychology-api")
	buildCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build API executable: %v\n%s", err, output)
	}

	path := filepath.Join(dir, "server.log")
	logFile, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := logFile.Close(); err != nil {
			t.Error(err)
		}
	})
	cmd := exec.Command(binary)
	// Only the disposable fixture connections enter the executable environment.
	cmd.Env = []string{
		"ENV=test", "HOST=127.0.0.1", "PORT=0",
		"DATABASE_URL=" + os.Getenv(dbfixture.AppDSN),
		"WORKER_DATABASE_URL=" + os.Getenv(dbfixture.WorkerDSN),
		"APPEARANCE_OWNER_USER_ID=" + appearanceOwnerID,
		"S3_ENDPOINT=https://media.example.test",
		"S3_BUCKET=psychology-fixture",
		"S3_ACCESS_KEY=" + uuid.NewString(),
		"S3_SECRET_KEY=" + uuid.NewString(),
	}
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err := cmd.Start(); err != nil {
		t.Fatalf("start API executable: %v", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	t.Cleanup(func() {
		select {
		case err := <-exited:
			if err != nil {
				t.Errorf("API executable exited before cleanup: %v", err)
			}
			return
		default:
		}
		if err := cmd.Process.Signal(os.Interrupt); err != nil && err != os.ErrProcessDone {
			t.Error(err)
		}
		select {
		case err := <-exited:
			if err != nil {
				t.Errorf("API executable shutdown: %v", err)
			}
		case <-time.After(5 * time.Second):
			if err := cmd.Process.Kill(); err != nil && err != os.ErrProcessDone {
				t.Error(err)
			}
			<-exited
			t.Error("API executable did not shut down within five seconds")
		}
	})

	var address string
	deadline := time.Now().Add(5 * time.Second)
	for address == "" && time.Now().Before(deadline) {
		output, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(output), "\n") {
			var record struct{ Address string }
			if json.Unmarshal([]byte(line), &record) == nil && strings.HasPrefix(record.Address, "127.0.0.1:") {
				address = record.Address
				break
			}
		}
		select {
		case err := <-exited:
			exited <- err
			t.Fatalf("API executable exited at startup: %v\n%s", err, output)
		default:
		}
		if address == "" {
			time.Sleep(25 * time.Millisecond)
		}
	}
	if address == "" {
		output, err := os.ReadFile(path)
		t.Fatalf("API executable did not report a loopback listener: %v\n%s", err, output)
	}

	revision := "unavailable"
	if output, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
		revision = strings.TrimSpace(string(output))
	}
	t.Logf("API executable pid=%d address=%s checkout=%s", cmd.Process.Pid, address, revision)

	client := &http.Client{
		Timeout:       3 * time.Second,
		Transport:     &http.Transport{},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	t.Cleanup(client.CloseIdleConnections)
	return address, func(w http.ResponseWriter, request *http.Request) {
		t.Helper()
		outbound, err := http.NewRequestWithContext(request.Context(), request.Method, "http://"+address+request.URL.RequestURI(), request.Body)
		if err != nil {
			t.Fatal(err)
		}
		outbound.Host, outbound.Header = request.Host, request.Header.Clone()
		response, err := client.Do(outbound)
		if err != nil {
			t.Fatalf("request %s: %v", request.URL.Path, err)
		}
		defer response.Body.Close()
		for key, values := range response.Header {
			w.Header()[key] = values
		}
		w.WriteHeader(response.StatusCode)
		if _, err := io.Copy(w, response.Body); err != nil {
			t.Fatal(err)
		}
	}
}

func runFixtureBrowser(t *testing.T, address, hostA, titleA, hostB, titleB, sessionToken, tenantID string) {
	t.Helper()
	if os.Getenv("PSYCHOLOGY_TEST_BROWSER") != "1" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	browser := exec.CommandContext(ctx, "node", "../../../../scripts/test-browser.mjs",
		address, hostA, titleA, hostB, titleB)
	// Let the browser supervisor close its subprocess groups and profile on timeout.
	browser.Cancel = func() error { return browser.Process.Signal(syscall.SIGTERM) }
	browser.WaitDelay = 8 * time.Second
	browser.Env = []string{
		"CHROME_BIN=" + os.Getenv("CHROME_BIN"),
		"TMPDIR=" + os.TempDir(),
		"PHASE1_BROWSER_FAIL_PROBE=" + os.Getenv("PHASE1_BROWSER_FAIL_PROBE"),
		"PSYCHOLOGY_FIXTURE_SESSION=" + sessionToken,
		"PSYCHOLOGY_FIXTURE_TENANT=" + tenantID,
	}
	output, err := browser.CombinedOutput()
	if err != nil {
		t.Fatalf("browser checks: %v\n%s", err, output)
	}
	t.Logf("browser checks: %s", output)
}

func assertUnknownPublicHost(t *testing.T, serve func(http.ResponseWriter, *http.Request), host string) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/v1/public/site", nil)
	request.Host = host
	recorder := httptest.NewRecorder()
	serve(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected unknown or unverified Host to return 404, got %d", recorder.Code)
	}
}
