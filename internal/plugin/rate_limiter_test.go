package plugin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHTTPRateLimiter_AllowWithinLimit(t *testing.T) {
	limiter := newHTTPRateLimiter(3, time.Minute)

	assert.True(t, limiter.Allow(), "call 1 should be allowed")
	assert.True(t, limiter.Allow(), "call 2 should be allowed")
	assert.True(t, limiter.Allow(), "call 3 should be allowed")
}

func TestHTTPRateLimiter_BlockAfterLimit(t *testing.T) {
	const limit = 3
	limiter := newHTTPRateLimiter(limit, time.Minute)

	for range limit {
		assert.True(t, limiter.Allow())
	}

	assert.False(t, limiter.Allow(), "call beyond limit should be blocked")
	assert.False(t, limiter.Allow(), "subsequent calls should also be blocked")
}

func TestHTTPRateLimiter_WindowExpiry(t *testing.T) {
	// Use a very short window so the test completes quickly.
	const limit = 2
	window := 50 * time.Millisecond
	limiter := newHTTPRateLimiter(limit, window)

	assert.True(t, limiter.Allow())
	assert.True(t, limiter.Allow())
	assert.False(t, limiter.Allow(), "should be blocked at limit")

	// Wait for the window to expire.
	time.Sleep(window + 10*time.Millisecond)

	assert.True(t, limiter.Allow(), "should be allowed after window expiry")
}

func TestHTTPRateLimiter_ZeroLimit(t *testing.T) {
	limiter := newHTTPRateLimiter(0, time.Minute)
	// A zero limit means no calls are allowed.
	assert.False(t, limiter.Allow())
}
