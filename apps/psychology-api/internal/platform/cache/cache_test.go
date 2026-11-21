package cache

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
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
