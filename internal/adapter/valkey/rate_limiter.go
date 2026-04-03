package valkey

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	RateLimitKeyPrefix     = "ratelimit:"
	DefaultRateLimit       = 100 // requests per window
	DefaultRateLimitWindow = time.Minute
)

// RateLimiter determines whether a request identified by key should be allowed.
type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// --- Valkey (Redis-compatible) implementation ---

// ValkeyRateLimiter uses Redis INCR + EXPIRE pipeline for distributed rate
// limiting with a fixed-window counter approach.
type ValkeyRateLimiter struct {
	client *redis.Client
	limit  int
	window time.Duration
}

// NewValkeyRateLimiter returns a RateLimiter backed by a Valkey/Redis client.
func NewValkeyRateLimiter(client *redis.Client, limit int, window time.Duration) *ValkeyRateLimiter {
	return &ValkeyRateLimiter{
		client: client,
		limit:  limit,
		window: window,
	}
}

// Allow checks whether the given key is within its rate limit. It atomically
// increments the counter and sets an expiry on first access within a window.
func (r *ValkeyRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	fullKey := RateLimitKeyPrefix + key

	pipe := r.client.Pipeline()
	incrCmd := pipe.Incr(ctx, fullKey)
	pipe.Expire(ctx, fullKey, r.window)

	if _, err := pipe.Exec(ctx); err != nil {
		return false, err
	}

	count := incrCmd.Val()
	return count <= int64(r.limit), nil
}

// --- In-memory implementation (for testing) ---

// InMemoryRateLimiter is a simple in-process rate limiter backed by a map with
// a mutex. It is intended for unit tests where a real Valkey instance is not
// available.
type InMemoryRateLimiter struct {
	mu     sync.Mutex
	counts map[string]int
	limit  int
}

// NewInMemoryRateLimiter returns a RateLimiter that tracks counts in memory.
func NewInMemoryRateLimiter(limit int) *InMemoryRateLimiter {
	return &InMemoryRateLimiter{
		counts: make(map[string]int),
		limit:  limit,
	}
}

// Allow increments the counter for key and returns whether it is within the
// configured limit.
func (r *InMemoryRateLimiter) Allow(_ context.Context, key string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	fullKey := RateLimitKeyPrefix + key
	r.counts[fullKey]++

	return r.counts[fullKey] <= r.limit, nil
}
