package valkey

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter_AllowsWithinLimit(t *testing.T) {
	rl := NewInMemoryRateLimiter(10)
	ctx := context.Background()

	for i := range 10 {
		allowed, err := rl.Allow(ctx, "user:1")
		require.NoError(t, err)
		assert.True(t, allowed, "request %d should be allowed", i+1)
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := NewInMemoryRateLimiter(3)
	ctx := context.Background()

	for i := range 3 {
		allowed, err := rl.Allow(ctx, "user:1")
		require.NoError(t, err)
		assert.True(t, allowed, "request %d should be allowed", i+1)
	}

	allowed, err := rl.Allow(ctx, "user:1")
	require.NoError(t, err)
	assert.False(t, allowed, "4th request should be blocked")
}

func TestRateLimiter_SeparateKeys(t *testing.T) {
	rl := NewInMemoryRateLimiter(2)
	ctx := context.Background()

	// Exhaust limit for key A
	for range 2 {
		allowed, err := rl.Allow(ctx, "user:a")
		require.NoError(t, err)
		assert.True(t, allowed)
	}
	blocked, err := rl.Allow(ctx, "user:a")
	require.NoError(t, err)
	assert.False(t, blocked, "user:a should be blocked after limit")

	// Key B should still be independent
	allowed, err := rl.Allow(ctx, "user:b")
	require.NoError(t, err)
	assert.True(t, allowed, "user:b should be allowed independently")
}

func TestRateLimiter_ZeroLimit(t *testing.T) {
	rl := NewInMemoryRateLimiter(0)
	ctx := context.Background()

	allowed, err := rl.Allow(ctx, "user:1")
	require.NoError(t, err)
	assert.False(t, allowed, "zero limit should block everything")
}

// TestSlidingWindowRateLimiter_ImplementsInterface verifies at compile time
// that SlidingWindowRateLimiter satisfies the RateLimiter interface.
func TestSlidingWindowRateLimiter_ImplementsInterface(t *testing.T) {
	var _ RateLimiter = (*SlidingWindowRateLimiter)(nil)
}

// TestValkeyRateLimiter_ImplementsInterface verifies at compile time that the
// original ValkeyRateLimiter still satisfies the RateLimiter interface.
func TestValkeyRateLimiter_ImplementsInterface(t *testing.T) {
	var _ RateLimiter = (*ValkeyRateLimiter)(nil)
}
