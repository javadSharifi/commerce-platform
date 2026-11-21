package booking

import (
	"sync"
	"time"
)

// RateLimiter provides an in-memory sliding-window rate limiter for guest requests.
type RateLimiter struct {
	mu          sync.Mutex
	limits      map[string][]time.Time
	maxRequests int
	window      time.Duration
}

// NewRateLimiter creates a new sliding-window rate limiter.
func NewRateLimiter(maxRequests int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limits:      make(map[string][]time.Time),
		maxRequests: maxRequests,
		window:      window,
	}
}

// Allow checks if the given key (e.g. client IP or tenant:IP) is allowed to perform a request at `now`.
func (rl *RateLimiter) Allow(key string, now time.Time) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := now.Add(-rl.window)
	timestamps := rl.limits[key]

	// Retain only timestamps within the active sliding window
	valid := timestamps[:0]
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.maxRequests {
		rl.limits[key] = valid
		return false
	}

	rl.limits[key] = append(valid, now)
	return true
}

// Reset clears the rate limit state for a key.
func (rl *RateLimiter) Reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.limits, key)
}

// Cleanup removes stale records to prevent memory growth over time.
func (rl *RateLimiter) Cleanup(now time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := now.Add(-rl.window)
	for key, timestamps := range rl.limits {
		valid := timestamps[:0]
		for _, t := range timestamps {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(rl.limits, key)
		} else {
			rl.limits[key] = valid
		}
	}
}
