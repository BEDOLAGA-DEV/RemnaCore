package plugin

import (
	"sync"
	"time"
)

// httpRateLimiter is a simple sliding-window rate limiter that tracks call
// timestamps within a fixed window. It is safe for concurrent use.
type httpRateLimiter struct {
	mu     sync.Mutex
	calls  []time.Time
	limit  int
	window time.Duration
}

// newHTTPRateLimiter creates a rate limiter that allows at most limit calls
// within the given window duration.
func newHTTPRateLimiter(limit int, window time.Duration) *httpRateLimiter {
	return &httpRateLimiter{
		limit:  limit,
		window: window,
		calls:  make([]time.Time, 0, limit),
	}
}

// Allow returns true if a new call is permitted within the rate limit window.
// If allowed, the call is recorded. If the limit has been reached, it returns
// false without recording the call.
func (l *httpRateLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	// Compact: keep only calls within the window.
	valid := l.calls[:0]
	for _, t := range l.calls {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= l.limit {
		l.calls = valid
		return false
	}

	l.calls = append(valid, now)
	return true
}
