package plugin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

func TestHTTPRateLimiter_AllowWithinLimit(t *testing.T) {
	clk := clock.NewMock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	limiter := newHTTPRateLimiter(3, time.Minute, clk)

	assert.True(t, limiter.Allow(), "call 1 should be allowed")
	assert.True(t, limiter.Allow(), "call 2 should be allowed")
	assert.True(t, limiter.Allow(), "call 3 should be allowed")
}

func TestHTTPRateLimiter_BlockAfterLimit(t *testing.T) {
	const limit = 3
	clk := clock.NewMock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	limiter := newHTTPRateLimiter(limit, time.Minute, clk)

	for range limit {
		assert.True(t, limiter.Allow())
	}

	assert.False(t, limiter.Allow(), "call beyond limit should be blocked")
	assert.False(t, limiter.Allow(), "subsequent calls should also be blocked")
}

func TestHTTPRateLimiter_WindowExpiry(t *testing.T) {
	const limit = 2
	window := time.Minute
	clk := clock.NewMock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	limiter := newHTTPRateLimiter(limit, window, clk)

	assert.True(t, limiter.Allow())
	assert.True(t, limiter.Allow())
	assert.False(t, limiter.Allow(), "should be blocked at limit")

	// Advance the mock clock past the window to expire all recorded calls.
	clk.Advance(window + time.Second)

	assert.True(t, limiter.Allow(), "should be allowed after window expiry")
}

func TestHTTPRateLimiter_ZeroLimit(t *testing.T) {
	clk := clock.NewMock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	limiter := newHTTPRateLimiter(0, time.Minute, clk)
	// A zero limit means no calls are allowed.
	assert.False(t, limiter.Allow())
}

func TestHTTPRateLimiter_PartialWindowExpiry(t *testing.T) {
	const limit = 3
	window := time.Minute
	clk := clock.NewMock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	limiter := newHTTPRateLimiter(limit, window, clk)

	// Fill two of three slots at t=0.
	assert.True(t, limiter.Allow())
	assert.True(t, limiter.Allow())

	// Advance to t=30s and fill the third slot.
	clk.Advance(30 * time.Second)
	assert.True(t, limiter.Allow())
	assert.False(t, limiter.Allow(), "should be blocked at limit")

	// Advance to t=61s — the first two calls (at t=0) expire, but the
	// third (at t=30s) is still within the window.
	clk.Advance(31 * time.Second)
	assert.True(t, limiter.Allow(), "first two calls expired, should allow")
	assert.True(t, limiter.Allow(), "second slot freed")
	assert.False(t, limiter.Allow(), "third slot still active, should block")
}
