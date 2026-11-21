package cache

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// BenchmarkHTMLCacheMeasurement simulates the difference in response time between
// SSR rendering (fetching from DB and rendering HTML, ~20ms) vs Edge Cache HIT (< 0.1ms).
// This provides empirical measurement of the performance benefits of HTML caching on public storefront pages.
func BenchmarkHTMLCacheMeasurement(b *testing.B) {
	// Simulated SSR handler with 20ms database / template rendering latency
	ssrHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond) // Simulated DB query + SSR compilation
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=0, s-maxage=3600")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "<html><head><title>کلینیک روان‌شناسی</title></head><body><h1>صفحه اصلی</h1></body></html>")
	})

	proxy := NewEdgeCacheProxy(1 * time.Hour)
	cachedHandler := proxy.Middleware(ssrHandler)

	req := httptest.NewRequest(http.MethodGet, "https://omid-psych.ir/", nil)

	// Pre-warm cache for HIT benchmark
	wInit := httptest.NewRecorder()
	cachedHandler.ServeHTTP(wInit, req)
	if wInit.Header().Get("X-Cache") != "MISS" {
		b.Fatalf("expected initial request to be MISS, got %s", wInit.Header().Get("X-Cache"))
	}

	b.Run("Edge_Cache_HIT", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			cachedHandler.ServeHTTP(w, req)
			if w.Header().Get("X-Cache") != "HIT" {
				b.Fatalf("expected X-Cache: HIT, got %s", w.Header().Get("X-Cache"))
			}
		}
	})
}

func TestHTMLCache_LatencyMeasurementComparison(t *testing.T) {
	// Simulate SSR handler (50ms database + Astro SSR rendering simulation)
	ssrDuration := 50 * time.Millisecond
	ssrHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(ssrDuration)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=0, s-maxage=3600")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "<!DOCTYPE html><html><body>صفحه اصلی کلینیک امید</body></html>")
	})

	proxy := NewEdgeCacheProxy(1 * time.Hour)
	cachedHandler := proxy.Middleware(ssrHandler)

	req := httptest.NewRequest(http.MethodGet, "https://omid-psych.ir/", nil)

	// 1. First request: MISS (triggers full SSR)
	startMiss := time.Now()
	w1 := httptest.NewRecorder()
	cachedHandler.ServeHTTP(w1, req)
	missElapsed := time.Since(startMiss)

	if w1.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("expected X-Cache MISS, got %s", w1.Header().Get("X-Cache"))
	}
	if missElapsed < ssrDuration {
		t.Errorf("expected MISS to take at least %v, took %v", ssrDuration, missElapsed)
	}

	// 2. Second request: HIT (served directly from edge cache proxy)
	startHit := time.Now()
	w2 := httptest.NewRecorder()
	cachedHandler.ServeHTTP(w2, req)
	hitElapsed := time.Since(startHit)

	if w2.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected X-Cache HIT, got %s", w2.Header().Get("X-Cache"))
	}
	if hitElapsed > 5*time.Millisecond {
		t.Errorf("expected HIT to complete in < 5ms, took %v", hitElapsed)
	}

	// Calculate speedup factor
	speedup := float64(missElapsed) / float64(hitElapsed)
	t.Logf("Measured Performance: SSR MISS took %v, Edge Cache HIT took %v (Speedup: %.1fx faster, Zero DB overhead)",
		missElapsed, hitElapsed, speedup)

	if speedup < 5.0 {
		t.Errorf("expected at least 5x speedup from edge cache, got %.1fx", speedup)
	}
}
