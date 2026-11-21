package cache

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type CachedResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

// EdgeCacheProxy simulates and executes edge / reverse-proxy HTTP caching with host-scoped cache keys,
// strictly conforming to rules:
// - Cache key = Host + Path
// - Query strings and private paths BYPASS shared cache
// - Responses with no-store or private are NOT stored
// - Purge affects ONLY the specified host's paths (strict tenant isolation)
type EdgeCacheProxy struct {
	mu         sync.RWMutex
	entries    map[string]CachedResponse
	defaultTTL time.Duration
}

func NewEdgeCacheProxy(defaultTTL time.Duration) *EdgeCacheProxy {
	if defaultTTL <= 0 {
		defaultTTL = 1 * time.Hour
	}
	return &EdgeCacheProxy{
		entries:    make(map[string]CachedResponse),
		defaultTTL: defaultTTL,
	}
}

// Middleware returns an HTTP middleware wrapping the upstream handler with edge caching behavior.
func (p *EdgeCacheProxy) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if host == "" {
			host = r.Header.Get("Host")
		}

		// 1. Check for BYPASS
		bypass, reason := ShouldBypassCache(r)
		if bypass {
			w.Header().Set("X-Cache", "BYPASS")
			w.Header().Set("X-Cache-Bypass-Reason", reason)
			next.ServeHTTP(w, r)
			return
		}

		key := GenerateCacheKey(host, r.URL.Path)

		// 2. Check for HIT
		p.mu.RLock()
		cached, found := p.entries[key]
		p.mu.RUnlock()

		now := time.Now()
		if found && now.Before(cached.ExpiresAt) {
			// Cache HIT
			for k, values := range cached.Headers {
				for _, v := range values {
					w.Header().Add(k, v)
				}
			}
			age := int(now.Sub(cached.CreatedAt).Seconds())
			w.Header().Set("Age", fmt.Sprintf("%d", age))
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(cached.StatusCode)
			_, _ = w.Write(cached.Body)
			return
		}

		// 3. Cache MISS - capture response from upstream
		rec := &responseRecorder{
			ResponseWriter: w,
			body:           &bytes.Buffer{},
			headers:        make(http.Header),
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(rec, r)

		// Determine if response is cacheable
		cc := strings.ToLower(rec.headers.Get("Cache-Control"))
		isPrivateOrNoStore := strings.Contains(cc, "no-store") ||
			strings.Contains(cc, "private") ||
			strings.Contains(cc, "no-cache")

		canCacheResponse := rec.statusCode == http.StatusOK && !isPrivateOrNoStore

		if canCacheResponse {
			p.mu.Lock()
			// Copy headers
			headersCopy := make(http.Header)
			for k, v := range rec.headers {
				headersCopy[k] = append([]string(nil), v...)
			}

			p.entries[key] = CachedResponse{
				StatusCode: rec.statusCode,
				Headers:    headersCopy,
				Body:       rec.body.Bytes(),
				CreatedAt:  now,
				ExpiresAt:  now.Add(p.defaultTTL),
			}
			p.mu.Unlock()
		}

		// Write captured response to client
		for k, values := range rec.headers {
			for _, v := range values {
				w.Header().Add(k, v)
			}
		}
		w.Header().Set("X-Cache", "MISS")
		w.WriteHeader(rec.statusCode)
		_, _ = io.Copy(w, rec.body)
	})
}

// Purge removes cached entries for a specific host and path list.
// If paths contains "*", all entries for that host are purged.
// Strict multi-tenant isolation: entries for other hosts are NEVER touched.
func (p *EdgeCacheProxy) Purge(host string, paths []string) int {
	cleanHost := strings.TrimSpace(strings.ToLower(host))
	if cleanHost == "" {
		return 0
	}
	if colonIdx := strings.Index(cleanHost, ":"); colonIdx != -1 {
		cleanHost = cleanHost[:colonIdx]
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	purgedCount := 0

	purgeAllHost := false
	for _, path := range paths {
		if path == "*" {
			purgeAllHost = true
			break
		}
	}

	if purgeAllHost {
		prefix := cleanHost + "/"
		for k := range p.entries {
			if strings.HasPrefix(k, prefix) || k == cleanHost {
				delete(p.entries, k)
				purgedCount++
			}
		}
		return purgedCount
	}

	for _, path := range paths {
		key := GenerateCacheKey(cleanHost, path)
		if _, exists := p.entries[key]; exists {
			delete(p.entries, key)
			purgedCount++
		}
	}

	return purgedCount
}

// GetEntry returns a cached response if present.
func (p *EdgeCacheProxy) GetEntry(host, path string) (CachedResponse, bool) {
	key := GenerateCacheKey(host, path)
	p.mu.RLock()
	defer p.mu.RUnlock()
	entry, found := p.entries[key]
	return entry, found
}

// Size returns the total count of cached items.
func (p *EdgeCacheProxy) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.entries)
}

type responseRecorder struct {
	http.ResponseWriter
	body       *bytes.Buffer
	headers    http.Header
	statusCode int
}

func (r *responseRecorder) Header() http.Header {
	return r.headers
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	return r.body.Write(b)
}
