package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestEdgeCache_HIT_and_MISS(t *testing.T) {
	callCount := 0
	upstreamHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=0, s-maxage=3600")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "<html><body>محتوای کارگاه - شماره فراخوانی: %d</body></html>", callCount)
	})

	proxy := NewEdgeCacheProxy(1 * time.Hour)
	handler := proxy.Middleware(upstreamHandler)

	req := httptest.NewRequest(http.MethodGet, "https://omid-psych.ir/blog/article-1", nil)

	// 1. First Request: Must be MISS
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req)

	if rec1.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("expected first request to be MISS, got: %s", rec1.Header().Get("X-Cache"))
	}
	if callCount != 1 {
		t.Fatalf("expected upstream to be called once, got %d", callCount)
	}

	// 2. Second Request: Must be HIT
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req)

	if rec2.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected second request to be HIT, got: %s", rec2.Header().Get("X-Cache"))
	}
	if callCount != 1 {
		t.Fatalf("expected upstream call count to remain 1 on HIT, got %d", callCount)
	}
	if rec1.Body.String() != rec2.Body.String() {
		t.Errorf("cached body does not match initial response: %s vs %s", rec1.Body.String(), rec2.Body.String())
	}
}

func TestEdgeCache_BYPASS_Scenarios(t *testing.T) {
	upstreamHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "<html><body>Upstream Response</body></html>")
	})

	proxy := NewEdgeCacheProxy(1 * time.Hour)
	handler := proxy.Middleware(upstreamHandler)

	testCases := []struct {
		name           string
		method         string
		url            string
		headers        map[string]string
		cookies        []*http.Cookie
		expectedBypass bool
		expectedReason string
	}{
		{
			name:           "Public homepage canonical - Cacheable",
			method:         http.MethodGet,
			url:            "https://omid-psych.ir/",
			expectedBypass: false,
		},
		{
			name:           "Public blog canonical - Cacheable",
			method:         http.MethodGet,
			url:            "https://omid-psych.ir/blog",
			expectedBypass: false,
		},
		{
			name:           "Query string present (?search=anxiety) - BYPASS",
			method:         http.MethodGet,
			url:            "https://omid-psych.ir/blog?search=anxiety",
			expectedBypass: true,
			expectedReason: "query_string_present",
		},
		{
			name:           "Pagination query (?page=2) - BYPASS",
			method:         http.MethodGet,
			url:            "https://omid-psych.ir/workshops?page=2",
			expectedBypass: true,
			expectedReason: "query_string_present",
		},
		{
			name:           "Private appointment tracking route - BYPASS",
			method:         http.MethodGet,
			url:            "https://omid-psych.ir/appointments/123",
			expectedBypass: true,
			expectedReason: "private_route",
		},
		{
			name:           "Admin dashboard route - BYPASS",
			method:         http.MethodGet,
			url:            "https://omid-psych.ir/admin/blog",
			expectedBypass: true,
			expectedReason: "private_route",
		},
		{
			name:           "Buyer account route - BYPASS",
			method:         http.MethodGet,
			url:            "https://omid-psych.ir/account/courses",
			expectedBypass: true,
			expectedReason: "private_route",
		},
		{
			name:           "Private video stream route - BYPASS",
			method:         http.MethodGet,
			url:            "https://omid-psych.ir/buyer/courses/c1/lessons/l1/stream",
			expectedBypass: true,
			expectedReason: "private_route",
		},
		{
			name:           "Authorization header present - BYPASS",
			method:         http.MethodGet,
			url:            "https://omid-psych.ir/blog",
			headers:        map[string]string{"Authorization": "Bearer some-token"},
			expectedBypass: true,
			expectedReason: "authorization_header_present",
		},
		{
			name:   "Buyer session cookie present - BYPASS",
			method: http.MethodGet,
			url:    "https://omid-psych.ir/",
			cookies: []*http.Cookie{
				{Name: "buyer_session_token", Value: "active-session-id"},
			},
			expectedBypass: true,
			expectedReason: "session_cookie_present",
		},
		{
			name:   "Admin session cookie present - BYPASS",
			method: http.MethodGet,
			url:    "https://omid-psych.ir/blog",
			cookies: []*http.Cookie{
				{Name: "admin_session", Value: "admin-auth-jwt"},
			},
			expectedBypass: true,
			expectedReason: "session_cookie_present",
		},
		{
			name:           "HTTP POST method - BYPASS",
			method:         http.MethodPost,
			url:            "https://omid-psych.ir/blog",
			expectedBypass: true,
			expectedReason: "method_not_cacheable",
		},
		{
			name:           "Client Cache-Control: no-cache - BYPASS",
			method:         http.MethodGet,
			url:            "https://omid-psych.ir/",
			headers:        map[string]string{"Cache-Control": "no-cache"},
			expectedBypass: true,
			expectedReason: "client_no_cache",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.url, nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			for _, c := range tc.cookies {
				req.AddCookie(c)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			xCache := rec.Header().Get("X-Cache")
			if tc.expectedBypass {
				if xCache != "BYPASS" {
					t.Fatalf("expected X-Cache BYPASS, got: %s", xCache)
				}
				reason := rec.Header().Get("X-Cache-Bypass-Reason")
				if tc.expectedReason != "" && reason != tc.expectedReason {
					t.Errorf("expected bypass reason %s, got: %s", tc.expectedReason, reason)
				}
			} else {
				if xCache != "MISS" && xCache != "HIT" {
					t.Fatalf("expected cacheable request to be MISS or HIT, got: %s", xCache)
				}
			}
		})
	}
}

func TestEdgeCache_MultiTenant_Host_Isolation(t *testing.T) {
	proxy := NewEdgeCacheProxy(1 * time.Hour)

	callTracker := make(map[string]int)
	var mu sync.Mutex

	handler := proxy.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callTracker[r.Host]++
		count := callTracker[r.Host]
		mu.Unlock()

		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Cache-Control", "public, max-age=0, s-maxage=3600")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "<html><body>کلینیک: %s - نسخه: %d</body></html>", r.Host, count)
	}))

	reqA := httptest.NewRequest(http.MethodGet, "https://omid-psych.ir/blog", nil)
	reqB := httptest.NewRequest(http.MethodGet, "https://mahan-clinic.ir/blog", nil)

	// 1. Initial requests for both clinics -> MISS
	recA1 := httptest.NewRecorder()
	handler.ServeHTTP(recA1, reqA)
	if recA1.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("expected A1 MISS, got %s", recA1.Header().Get("X-Cache"))
	}

	recB1 := httptest.NewRecorder()
	handler.ServeHTTP(recB1, reqB)
	if recB1.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("expected B1 MISS, got %s", recB1.Header().Get("X-Cache"))
	}

	// 2. Second requests for both -> HIT
	recA2 := httptest.NewRecorder()
	handler.ServeHTTP(recA2, reqA)
	if recA2.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected A2 HIT, got %s", recA2.Header().Get("X-Cache"))
	}

	recB2 := httptest.NewRecorder()
	handler.ServeHTTP(recB2, reqB)
	if recB2.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected B2 HIT, got %s", recB2.Header().Get("X-Cache"))
	}

	// 3. Purge ONLY Clinic A (omid-psych.ir)
	purged := proxy.Purge("omid-psych.ir", []string{"/blog"})
	if purged != 1 {
		t.Fatalf("expected 1 entry purged for Clinic A, got %d", purged)
	}

	// 4. Third request: Clinic A MUST be MISS again
	recA3 := httptest.NewRecorder()
	handler.ServeHTTP(recA3, reqA)
	if recA3.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("expected A3 to be MISS after purge, got %s", recA3.Header().Get("X-Cache"))
	}

	// 5. Third request: Clinic B MUST STILL BE HIT! (Strict Isolation)
	recB3 := httptest.NewRecorder()
	handler.ServeHTTP(recB3, reqB)
	if recB3.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected B3 to STILL BE HIT after Clinic A purge, got %s", recB3.Header().Get("X-Cache"))
	}

	if recB3.Body.String() != recB1.Body.String() {
		t.Errorf("Clinic B body changed unexpectedly")
	}
}

// ---------------- Mock Outbox DB and Worker Tests ----------------

type mockCDNPurger struct {
	mu         sync.Mutex
	purgedURLs []string
	shouldFail bool
}

func (m *mockCDNPurger) Purge(ctx context.Context, host string, paths []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail {
		return errors.New("simulated cdn network timeout")
	}

	for _, p := range paths {
		m.purgedURLs = append(m.purgedURLs, CanonicalURL(host, p))
	}
	return nil
}

func TestOutboxWorker_TransactionalPurge(t *testing.T) {
	db := NewMockOutboxDB()
	purger := &mockCDNPurger{}
	worker := NewOutboxWorker(db, purger)

	tenantID := uuid.New()
	eventID := uuid.New()

	// Enqueue an outbox event emitted from a blog post update transaction
	db.AddEvent(OutboxEvent{
		ID:        eventID,
		TenantID:  tenantID,
		EventType: "blog.published",
		Payload: OutboxPayload{
			Host:  "omid-psych.ir",
			Paths: []string{"/", "/blog", "/blog/post-1"},
		},
		Status:    "pending",
		CreatedAt: time.Now(),
	})

	// Run worker
	processed, err := worker.ProcessBatch(context.Background(), 10)
	if err != nil {
		t.Fatalf("worker failed: %v", err)
	}
	if processed != 1 {
		t.Fatalf("expected 1 event processed, got %d", processed)
	}

	// Verify CDN purger received exact URLs
	if len(purger.purgedURLs) != 3 {
		t.Fatalf("expected 3 URLs purged, got %d: %v", len(purger.purgedURLs), purger.purgedURLs)
	}
	expectedURLs := []string{
		"https://omid-psych.ir/",
		"https://omid-psych.ir/blog",
		"https://omid-psych.ir/blog/post-1",
	}
	for i, u := range expectedURLs {
		if purger.purgedURLs[i] != u {
			t.Errorf("expected URL %s, got %s", u, purger.purgedURLs[i])
		}
	}

	// Verify DB event status updated to completed
	e, ok := db.GetEvent(eventID)
	if !ok || e.Status != "completed" {
		t.Errorf("expected event status completed, got %s", e.Status)
	}
	if e.ProcessedAt == nil {
		t.Errorf("expected processed_at timestamp to be set")
	}
}

func TestOutboxWorker_RetryAndFailure(t *testing.T) {
	db := NewMockOutboxDB()
	purger := &mockCDNPurger{shouldFail: true}
	worker := NewOutboxWorker(db, purger)

	tenantID := uuid.New()
	eventID := uuid.New()

	db.AddEvent(OutboxEvent{
		ID:        eventID,
		TenantID:  tenantID,
		EventType: "therapist.updated",
		Payload: OutboxPayload{
			Host:  "mahan-clinic.ir",
			Paths: []string{"/therapists", "/booking"},
		},
		Status:    "pending",
		CreatedAt: time.Now(),
	})

	// First attempt -> retry 1
	processed, _ := worker.ProcessBatch(context.Background(), 10)
	if processed != 0 {
		t.Errorf("expected 0 success on failure, got %d", processed)
	}
	e, _ := db.GetEvent(eventID)
	if e.Status != "pending" || e.RetryCount != 1 {
		t.Errorf("expected pending retry 1, got status %s retry %d", e.Status, e.RetryCount)
	}

	// Second attempt -> retry 2
	_, _ = worker.ProcessBatch(context.Background(), 10)
	e, _ = db.GetEvent(eventID)
	if e.Status != "pending" || e.RetryCount != 2 {
		t.Errorf("expected pending retry 2, got status %s retry %d", e.Status, e.RetryCount)
	}

	// Third attempt -> retry 3 (exceeds maxRetries 3 -> failed)
	_, _ = worker.ProcessBatch(context.Background(), 10)
	e, _ = db.GetEvent(eventID)
	if e.Status != "failed" || e.RetryCount != 3 {
		t.Errorf("expected failed status on retry 3, got status %s retry %d", e.Status, e.RetryCount)
	}
	if e.LastError == nil || *e.LastError == "" {
		t.Errorf("expected last_error to be recorded")
	}
}

func TestOutboxWorker_RunLoop(t *testing.T) {
	db := NewMockOutboxDB()
	purger := &mockCDNPurger{}
	worker := NewOutboxWorker(db, purger)

	tenantID := uuid.New()
	eventID := uuid.New()

	db.AddEvent(OutboxEvent{
		ID:        eventID,
		TenantID:  tenantID,
		EventType: "article.published",
		Payload: OutboxPayload{
			Host:  "clinic.ir",
			Paths: []string{"/blog"},
		},
		Status:    "pending",
		CreatedAt: time.Now(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		worker.Run(ctx, 10*time.Millisecond)
		close(done)
	}()

	// Wait for event to be processed by ticker
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("worker Run did not exit after context cancellation")
	}

	e, ok := db.GetEvent(eventID)
	if !ok || e.Status != "completed" {
		t.Errorf("expected event status completed by Run loop, got %v", e.Status)
	}
}

type querySpyDB struct {
	OutboxDBTX
	lastSQL string
}

func (s *querySpyDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	s.lastSQL = sql
	return s.OutboxDBTX.Query(ctx, sql, args...)
}

func TestFetchPendingEvents_UsesForUpdateSkipLocked(t *testing.T) {
	mockDB := NewMockOutboxDB()
	spy := &querySpyDB{OutboxDBTX: mockDB}

	_, err := FetchPendingEvents(context.Background(), spy, 10)
	if err != nil {
		t.Fatalf("unexpected error fetching events: %v", err)
	}

	if !strings.Contains(spy.lastSQL, "FOR UPDATE SKIP LOCKED") {
		t.Fatalf("expected SQL query to contain 'FOR UPDATE SKIP LOCKED', got:\n%s", spy.lastSQL)
	}
}

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// TestAcceptance_PSY_6_8_RealCDNAdapterAndOutboxPurge verifies acceptance requirement PSY-6.8:
// 1. Tests real CDN adapter (ArvanCloudPurger) with individual URL purge payload and authorization.
// 2. Warms up Home (/) and a detail page (/courses/anxiety-management) for both Host A and Host B.
// 3. Simulates OutboxWorker processing after appearance update on Host A.
// 4. Verifies Host A (Home & detail) are purged (MISS -> HIT with new appearance).
// 5. Verifies Host B (Home & detail) remain strictly HIT (zero cross-tenant invalidation leakage).
// 6. Verifies consecutive /preview requests strictly BYPASS the shared cache with private, no-store.
func TestAcceptance_PSY_6_8_RealCDNAdapterAndOutboxPurge(t *testing.T) {
	// -------------------------------------------------------------------------
	// 1. Unit Test ArvanCloudPurger (Real CDN Adapter)
	// -------------------------------------------------------------------------
	// 1.a Empty API key returns nil (dev/test fallback mode)
	devPurger := NewArvanCloudPurger("", nil)
	if err := devPurger.Purge(context.Background(), "omid-psych.ir", []string{"/", "/courses"}); err != nil {
		t.Fatalf("expected nil error on empty API key, got: %v", err)
	}

	// 1.b Live CDN purge request verification with mock transport
	var capturedReq *http.Request
	var capturedBody map[string]any

	mockClient := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			capturedReq = req
			_ = json.NewDecoder(req.Body).Decode(&capturedBody)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"message":"purge submitted"}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	arvanPurger := NewArvanCloudPurger("Apikey secret-arvan-key", mockClient)
	err := arvanPurger.Purge(context.Background(), "omid-psych.ir", []string{"/", "/courses/anxiety-management"})
	if err != nil {
		t.Fatalf("unexpected error from arvanPurger: %v", err)
	}

	expectedEndpoint := "https://napi.arvancloud.ir/cdn/4.0/domains/omid-psych.ir/caching/purge"
	if capturedReq.URL.String() != expectedEndpoint {
		t.Fatalf("expected endpoint %s, got: %s", expectedEndpoint, capturedReq.URL.String())
	}
	if capturedReq.Header.Get("Authorization") != "Apikey secret-arvan-key" {
		t.Fatalf("expected Authorization header with Apikey, got: %s", capturedReq.Header.Get("Authorization"))
	}
	if capturedReq.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("expected Content-Type application/json, got: %s", capturedReq.Header.Get("Content-Type"))
	}
	if capturedBody["purge"] != "individual" {
		t.Fatalf("expected purge mode 'individual', got: %v", capturedBody["purge"])
	}
	rawUrls, ok := capturedBody["urls"].([]any)
	if !ok || len(rawUrls) != 2 {
		t.Fatalf("expected 2 URLs in purge payload, got: %v", capturedBody["urls"])
	}
	if rawUrls[0] != "https://omid-psych.ir/" || rawUrls[1] != "https://omid-psych.ir/courses/anxiety-management" {
		t.Fatalf("unexpected purge URLs: %v", rawUrls)
	}

	// 1.c Error handling from CDN API
	errorClient := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader(`{"message":"cdn gateway error"}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	failingPurger := NewArvanCloudPurger("Apikey secret-arvan-key", errorClient)
	if err := failingPurger.Purge(context.Background(), "omid-psych.ir", []string{"/"}); err == nil {
		t.Fatalf("expected error when CDN returns 502, got nil")
	}

	// -------------------------------------------------------------------------
	// 2. End-to-End Cache Warmup, Outbox Processing & Invalidation Isolation
	// -------------------------------------------------------------------------
	hostA := "omid-psych.ir"
	hostB := "mahan-clinic.ir"

	callCounts := make(map[string]int)
	var mu sync.Mutex

	// Appearance state for Host A and Host B
	appearanceA := "minimal-calm-sage"
	appearanceB := "minimal-warm-sand"

	upstreamHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		reqKey := r.Host + r.URL.Path
		callCounts[reqKey]++
		count := callCounts[reqKey]
		currentAppearance := appearanceA
		if r.Host == hostB {
			currentAppearance = appearanceB
		}
		mu.Unlock()

		// Preview requests must be private, no-store
		if strings.HasPrefix(r.URL.Path, "/preview") {
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "<html><body>پیش‌نمایش: %s - قالب: %s</body></html>", r.Host, r.URL.Query().Get("template"))
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=0, s-maxage=3600")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "<html><body data-appearance=\"%s\">کلینیک: %s - مسیر: %s (فراخوانی: %d)</body></html>",
			currentAppearance, r.Host, r.URL.Path, count)
	})

	proxy := NewEdgeCacheProxy(1 * time.Hour)
	handler := proxy.Middleware(upstreamHandler)

	outboxDB := NewMockOutboxDB()
	proxyPurger := NewProxyPurger(proxy)
	worker := NewOutboxWorker(outboxDB, proxyPurger)

	// Step 2.a: Warm up Home (/) and Detail Page (/courses/anxiety-management) for Host A
	reqAHome := httptest.NewRequest(http.MethodGet, "https://"+hostA+"/", nil)
	reqACourse := httptest.NewRequest(http.MethodGet, "https://"+hostA+"/courses/anxiety-management", nil)

	// First requests: MISS
	recAHome1 := httptest.NewRecorder()
	handler.ServeHTTP(recAHome1, reqAHome)
	if recAHome1.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("expected Host A Home first request to be MISS, got: %s", recAHome1.Header().Get("X-Cache"))
	}

	recACourse1 := httptest.NewRecorder()
	handler.ServeHTTP(recACourse1, reqACourse)
	if recACourse1.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("expected Host A Course first request to be MISS, got: %s", recACourse1.Header().Get("X-Cache"))
	}

	// Second requests: HIT
	recAHome2 := httptest.NewRecorder()
	handler.ServeHTTP(recAHome2, reqAHome)
	if recAHome2.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected Host A Home second request to be HIT, got: %s", recAHome2.Header().Get("X-Cache"))
	}

	recACourse2 := httptest.NewRecorder()
	handler.ServeHTTP(recACourse2, reqACourse)
	if recACourse2.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected Host A Course second request to be HIT, got: %s", recACourse2.Header().Get("X-Cache"))
	}

	// Step 2.b: Warm up Home (/) and Detail Page (/courses/anxiety-management) for Host B
	reqBHome := httptest.NewRequest(http.MethodGet, "https://"+hostB+"/", nil)
	reqBCourse := httptest.NewRequest(http.MethodGet, "https://"+hostB+"/courses/anxiety-management", nil)

	// First requests: MISS
	recBHome1 := httptest.NewRecorder()
	handler.ServeHTTP(recBHome1, reqBHome)
	if recBHome1.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("expected Host B Home first request to be MISS, got: %s", recBHome1.Header().Get("X-Cache"))
	}

	recBCourse1 := httptest.NewRecorder()
	handler.ServeHTTP(recBCourse1, reqBCourse)
	if recBCourse1.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("expected Host B Course first request to be MISS, got: %s", recBCourse1.Header().Get("X-Cache"))
	}

	// Second requests: HIT
	recBHome2 := httptest.NewRecorder()
	handler.ServeHTTP(recBHome2, reqBHome)
	if recBHome2.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected Host B Home second request to be HIT, got: %s", recBHome2.Header().Get("X-Cache"))
	}

	recBCourse2 := httptest.NewRecorder()
	handler.ServeHTTP(recBCourse2, reqBCourse)
	if recBCourse2.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected Host B Course second request to be HIT, got: %s", recBCourse2.Header().Get("X-Cache"))
	}

	// -------------------------------------------------------------------------
	// 3. Consecutive /preview requests must strictly BYPASS cache
	// -------------------------------------------------------------------------
	previewReq1 := httptest.NewRequest(http.MethodGet, "https://"+hostA+"/preview?template=clinical&palette=ocean-trust", nil)
	previewRec1 := httptest.NewRecorder()
	handler.ServeHTTP(previewRec1, previewReq1)
	if previewRec1.Header().Get("X-Cache") != "BYPASS" {
		t.Fatalf("expected preview request 1 to BYPASS, got: %s", previewRec1.Header().Get("X-Cache"))
	}
	if previewRec1.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("expected preview to have private, no-store, got: %s", previewRec1.Header().Get("Cache-Control"))
	}

	previewReq2 := httptest.NewRequest(http.MethodGet, "https://"+hostA+"/preview?template=clinical&palette=ocean-trust", nil)
	previewRec2 := httptest.NewRecorder()
	handler.ServeHTTP(previewRec2, previewReq2)
	if previewRec2.Header().Get("X-Cache") != "BYPASS" {
		t.Fatalf("expected preview request 2 to BYPASS, got: %s", previewRec2.Header().Get("X-Cache"))
	}

	// -------------------------------------------------------------------------
	// 4. Host A Appearance Published -> Outbox Event Enqueued & Processed
	// -------------------------------------------------------------------------
	mu.Lock()
	appearanceA = "mobile-app-royal-plum" // Appearance changed on Host A
	mu.Unlock()

	outboxEventID := uuid.New()
	outboxDB.AddEvent(OutboxEvent{
		ID:        outboxEventID,
		TenantID:  uuid.New(),
		EventType: "site.appearance.updated",
		Payload: OutboxPayload{
			Host:  hostA,
			Paths: []string{"/", "/courses/anxiety-management"},
		},
		Status:    "pending",
		CreatedAt: time.Now(),
	})

	// Run worker batch
	processed, err := worker.ProcessBatch(context.Background(), 10)
	if err != nil || processed != 1 {
		t.Fatalf("expected worker to process 1 event, processed=%d, err=%v", processed, err)
	}

	// -------------------------------------------------------------------------
	// 5. Verification: Host A is Purged (MISS -> HIT), Host B Remains HIT
	// -------------------------------------------------------------------------
	// 5.a Host A Home is MISS after outbox purge, rendering new appearance
	recAHomeAfterPurge1 := httptest.NewRecorder()
	handler.ServeHTTP(recAHomeAfterPurge1, reqAHome)
	if recAHomeAfterPurge1.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("expected Host A Home after purge to be MISS, got: %s", recAHomeAfterPurge1.Header().Get("X-Cache"))
	}
	if !strings.Contains(recAHomeAfterPurge1.Body.String(), "data-appearance=\"mobile-app-royal-plum\"") {
		t.Fatalf("expected Host A Home to render new appearance after purge, got: %s", recAHomeAfterPurge1.Body.String())
	}

	// Second request to Host A Home is re-cached as HIT
	recAHomeAfterPurge2 := httptest.NewRecorder()
	handler.ServeHTTP(recAHomeAfterPurge2, reqAHome)
	if recAHomeAfterPurge2.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected Host A Home to be HIT after re-caching, got: %s", recAHomeAfterPurge2.Header().Get("X-Cache"))
	}

	// 5.b Host A Course detail is MISS after outbox purge, rendering new appearance
	recACourseAfterPurge1 := httptest.NewRecorder()
	handler.ServeHTTP(recACourseAfterPurge1, reqACourse)
	if recACourseAfterPurge1.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("expected Host A Course after purge to be MISS, got: %s", recACourseAfterPurge1.Header().Get("X-Cache"))
	}
	if !strings.Contains(recACourseAfterPurge1.Body.String(), "data-appearance=\"mobile-app-royal-plum\"") {
		t.Fatalf("expected Host A Course to render new appearance after purge, got: %s", recACourseAfterPurge1.Body.String())
	}

	// 5.c Host B MUST REMAIN STRICTLY HIT on both Home and Detail (Zero Tenant Invalidation Leakage)
	recBHomeAfterPurge := httptest.NewRecorder()
	handler.ServeHTTP(recBHomeAfterPurge, reqBHome)
	if recBHomeAfterPurge.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("CRITICAL ISOLATION BUG: Host B Home was invalidated by Host A event! Expected HIT, got: %s", recBHomeAfterPurge.Header().Get("X-Cache"))
	}
	if !strings.Contains(recBHomeAfterPurge.Body.String(), "data-appearance=\"minimal-warm-sand\"") {
		t.Fatalf("Host B Home appearance changed unexpectedly: %s", recBHomeAfterPurge.Body.String())
	}

	recBCourseAfterPurge := httptest.NewRecorder()
	handler.ServeHTTP(recBCourseAfterPurge, reqBCourse)
	if recBCourseAfterPurge.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("CRITICAL ISOLATION BUG: Host B Course was invalidated by Host A event! Expected HIT, got: %s", recBCourseAfterPurge.Header().Get("X-Cache"))
	}
	if !strings.Contains(recBCourseAfterPurge.Body.String(), "data-appearance=\"minimal-warm-sand\"") {
		t.Fatalf("Host B Course appearance changed unexpectedly: %s", recBCourseAfterPurge.Body.String())
	}
}
