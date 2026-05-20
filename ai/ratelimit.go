package ai

import (
	"sync"
	"time"
)

// rateLimiter implements a per-key sliding window rate limiter.
type rateLimiter struct {
	mu      sync.Mutex
	windows map[string][]time.Time
	limit   int
	window  time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		windows: map[string][]time.Time{},
		limit:   limit,
		window:  window,
	}
}

// Allow returns true and records the event if the key is within its limit.
func (r *rateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	ts := r.windows[key]
	// drop timestamps outside the window
	start := 0
	for start < len(ts) && ts[start].Before(cutoff) {
		start++
	}
	ts = ts[start:]

	if len(ts) >= r.limit {
		r.windows[key] = ts
		return false
	}

	r.windows[key] = append(ts, now)
	return true
}

// 10 requests per minute per user by default.
var defaultLimiter = newRateLimiter(10, time.Minute)
