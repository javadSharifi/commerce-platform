package cache

import (
	"net/http"
	"net/url"
	"strings"
)

// GenerateCacheKey generates a canonical cache key composed strictly of Host + Path.
// The host is normalized (trimmed, lowercased, and port stripped if present).
// The path is cleaned to ensure leading slash and no redundant trailing slash (except root "/").
// Query parameters are deliberately EXCLUDED from the cache key because requests with query strings
// are bypassed from shared edge caching according to AP-10 and D-14.
func GenerateCacheKey(host, path string) string {
	cleanHost := strings.TrimSpace(strings.ToLower(host))
	if cleanHost == "" {
		cleanHost = "unknown"
	}
	// Strip port if present (e.g., "omid-psych.ir:443" -> "omid-psych.ir")
	if colonIdx := strings.Index(cleanHost, ":"); colonIdx != -1 {
		cleanHost = cleanHost[:colonIdx]
	}

	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" || cleanPath == "/" {
		return cleanHost + "/"
	}

	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}

	// Remove trailing slash if length > 1
	if len(cleanPath) > 1 && strings.HasSuffix(cleanPath, "/") {
		cleanPath = strings.TrimSuffix(cleanPath, "/")
	}

	return cleanHost + cleanPath
}

// IsCacheablePath returns true if the request path points to an allowed public HTML page:
// - "/" (Homepage)
// - "/therapists", "/booking" (Public directory / profiles)
// - "/blog", "/blog/*" (Published blog articles)
// - "/workshops", "/workshops/*" (Public workshop listings)
// - "/courses", "/courses/*" (Public course listings)
func IsCacheablePath(path string) bool {
	clean := strings.TrimSpace(path)
	if clean == "" || clean == "/" {
		return true
	}
	if !strings.HasPrefix(clean, "/") {
		clean = "/" + clean
	}
	if len(clean) > 1 && strings.HasSuffix(clean, "/") {
		clean = strings.TrimSuffix(clean, "/")
	}

	allowedPrefixes := []string{
		"/blog",
		"/therapists",
		"/booking",
		"/workshops",
		"/courses",
	}

	for _, prefix := range allowedPrefixes {
		if clean == prefix || strings.HasPrefix(clean, prefix+"/") {
			return true
		}
	}

	return false
}

// ShouldBypassCache inspects an HTTP request and determines if it must BYPASS the shared edge cache.
// Returns (true, reason) if the request should bypass the cache, or (false, "") if cacheable.
// Rules enforced:
// 1. Only GET and HEAD methods are cacheable (POST, PUT, DELETE, PATCH bypass).
// 2. Any query string bypasses shared cache (AP-10: query string bypasses shared edge cache).
// 3. Private and dynamic routes (/admin*, /account*, /appointments*, /checkout*, /callbacks*, /buyer*, /public/media/*/stream*) bypass.
// 4. Authenticated requests (Authorization header or admin/buyer session cookies) bypass.
// 5. Client no-cache headers (Cache-Control: no-cache, Pragma: no-cache) bypass.
// 6. Paths not explicitly listed in IsCacheablePath bypass.
func ShouldBypassCache(req *http.Request) (bool, string) {
	// 1. Method check
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		return true, "method_not_cacheable"
	}

	// 2. Query string check
	if req.URL.RawQuery != "" {
		return true, "query_string_present"
	}

	path := req.URL.Path

	// 3. Private route checks
	privatePrefixes := []string{
		"/admin",
		"/account",
		"/appointments",
		"/checkout",
		"/callbacks",
		"/buyer",
		"/v1/auth",
		"/public/workshops/registrations",
		"/public/appointments",
	}
	for _, priv := range privatePrefixes {
		if strings.HasPrefix(path, priv) {
			return true, "private_route"
		}
	}

	// Private media stream routes
	if strings.Contains(path, "/stream") {
		return true, "private_media_stream"
	}

	// 4. Authentication checks
	if req.Header.Get("Authorization") != "" {
		return true, "authorization_header_present"
	}

	// Check cookies for active sessions
	for _, cookie := range req.Cookies() {
		name := strings.ToLower(cookie.Name)
		if strings.Contains(name, "session") || strings.Contains(name, "token") || strings.Contains(name, "auth") {
			if strings.TrimSpace(cookie.Value) != "" {
				return true, "session_cookie_present"
			}
		}
	}

	// 5. Client Cache-Control headers
	cc := strings.ToLower(req.Header.Get("Cache-Control"))
	if strings.Contains(cc, "no-cache") || strings.Contains(cc, "no-store") {
		return true, "client_no_cache"
	}
	if strings.ToLower(req.Header.Get("Pragma")) == "no-cache" {
		return true, "client_no_cache"
	}

	// 6. Whitelist of cacheable paths
	if !IsCacheablePath(path) {
		return true, "path_not_cacheable"
	}

	return false, ""
}

// CanonicalURL combines host and path into a full canonical URL string.
func CanonicalURL(host, path string) string {
	u := url.URL{
		Scheme: "https",
		Host:   strings.TrimSpace(strings.ToLower(host)),
		Path:   path,
	}
	return u.String()
}
