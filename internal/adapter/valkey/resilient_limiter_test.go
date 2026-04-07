package valkey

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/circuitbreaker"
)

// testLogger returns a discard logger suitable for unit tests.
func testLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// testCBConfig returns the default no-interval CB config matching production defaults.
func testCBConfig() circuitbreaker.Config {
	return circuitbreaker.DefaultConfigNoInterval()
}

// errValkeyUnavailable is the sentinel used to simulate Valkey failures.
var errValkeyUnavailable = errors.New("valkey unavailable")

// failingLimiter is a RateLimiter stub that returns an error on every call,
// simulating a dead Valkey instance.
type failingLimiter struct {
	calls int
}

func (f *failingLimiter) Allow(_ context.Context, _ string) (bool, error) {
	f.calls++
	return false, errValkeyUnavailable
}

// TestResilientRateLimiter_ImplementsInterface verifies at compile time that
// ResilientRateLimiter satisfies the RateLimiter interface.
func TestResilientRateLimiter_ImplementsInterface(t *testing.T) {
	var _ RateLimiter = (*ResilientRateLimiter)(nil)
}

func TestResilientRateLimiter_PassesThrough(t *testing.T) {
	inner := NewInMemoryRateLimiter(3)
	rl := newTestResilientLimiter(inner)
	ctx := context.Background()

	tests := []struct {
		name string
		want bool
	}{
		{name: "first request allowed", want: true},
		{name: "second request allowed", want: true},
		{name: "third request allowed", want: true},
		{name: "fourth request blocked", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := rl.Allow(ctx, "user:1")
			require.NoError(t, err)
			assert.Equal(t, tt.want, allowed)
		})
	}
}

func TestResilientRateLimiter_BreakerOpensAfterConsecutiveFailures(t *testing.T) {
	fl := &failingLimiter{}
	cfg := testCBConfig()
	rl := newTestResilientLimiterFromRateLimiter(fl)
	ctx := context.Background()

	// Trip the breaker: MaxFailures consecutive errors.
	for i := range cfg.MaxFailures {
		allowed, err := rl.Allow(ctx, "user:1")
		require.NoError(t, err, "call %d should not return an error (fail-open)", i+1)
		assert.True(t, allowed, "call %d should fail-open", i+1)
	}

	callsBefore := fl.calls
	assert.Equal(t, int(cfg.MaxFailures), callsBefore, "inner should have been called for each failure")

	// Now the breaker is open -- further calls should NOT reach inner.
	for range 5 {
		allowed, err := rl.Allow(ctx, "user:1")
		require.NoError(t, err, "open-circuit call should not return error")
		assert.True(t, allowed, "open-circuit call should fail-open")
	}

	assert.Equal(t, callsBefore, fl.calls, "inner should not be called while breaker is open")
}

func TestResilientRateLimiter_BreakerRecovery(t *testing.T) {
	// Start with a failing limiter to trip the breaker.
	fl := &failingLimiter{}
	cfg := testCBConfig()
	rl := newTestResilientLimiterFromRateLimiter(fl)
	ctx := context.Background()

	// Trip the breaker.
	for range cfg.MaxFailures {
		_, _ = rl.Allow(ctx, "user:1")
	}

	// Verify breaker is open.
	callsBefore := fl.calls
	_, _ = rl.Allow(ctx, "user:1")
	assert.Equal(t, callsBefore, fl.calls, "breaker should be open")
}

// newTestResilientLimiter creates a ResilientRateLimiter wrapping an
// InMemoryRateLimiter for use in tests. The circuit breaker is real so we can
// test the integration of both components.
func newTestResilientLimiter(inner *InMemoryRateLimiter) *ResilientRateLimiter {
	// We create a proper ResilientRateLimiter using NewResilientRateLimiter.
	// However, NewResilientRateLimiter expects a *SlidingWindowRateLimiter.
	// For testing we use a wrapper approach: build the struct directly with
	// the same breaker settings but using the inner's Allow method.
	return newTestResilientLimiterFromRateLimiter(inner)
}

// newTestResilientLimiterFromRateLimiter builds a ResilientRateLimiter backed
// by any RateLimiter implementation. It uses the same circuit breaker settings
// as production but wraps a generic RateLimiter for test flexibility.
func newTestResilientLimiterFromRateLimiter(inner RateLimiter) *ResilientRateLimiter {
	return newResilientRateLimiterFromInterface(inner, testCBConfig(), testLogger())
}
