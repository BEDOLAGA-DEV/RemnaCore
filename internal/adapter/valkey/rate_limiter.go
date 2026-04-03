package valkey

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
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

// --- Sliding window implementation (sorted set) ---

const (
	// SlidingWindowTTLMultiplier controls how long sorted set entries are
	// retained beyond the window duration. Using 2x ensures entries from the
	// previous window are available for overlap calculation.
	SlidingWindowTTLMultiplier = 2

	// slidingWindowMemberIDLen is the number of bytes used from a UUID to make
	// sorted set members unique within the same nanosecond.
	slidingWindowMemberIDLen = 8
)

// SlidingWindowRateLimiter uses Redis sorted sets to implement a sliding window
// rate limiter. Each request is stored as a member scored by its timestamp. The
// window slides continuously, preventing the burst-at-boundary problem that
// fixed-window counters suffer from.
type SlidingWindowRateLimiter struct {
	client *redis.Client
	limit  int
	window time.Duration
}

// NewSlidingWindowRateLimiter returns a RateLimiter that uses sorted sets for
// sliding window rate limiting.
func NewSlidingWindowRateLimiter(client *redis.Client, limit int, window time.Duration) *SlidingWindowRateLimiter {
	return &SlidingWindowRateLimiter{
		client: client,
		limit:  limit,
		window: window,
	}
}

// Allow checks whether the given key is within its rate limit using a sliding
// window over a Redis sorted set. It atomically removes expired entries, counts
// current entries, adds the new request, and sets a TTL — all in a single
// pipeline round-trip.
func (r *SlidingWindowRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	now := time.Now()
	windowStart := now.Add(-r.window)
	fullKey := RateLimitKeyPrefix + key

	pipe := r.client.Pipeline()

	// Remove entries that have fallen outside the sliding window.
	pipe.ZRemRangeByScore(ctx, fullKey, "-inf", fmt.Sprintf("%d", windowStart.UnixNano()))

	// Count how many entries remain in the current window.
	countCmd := pipe.ZCard(ctx, fullKey)

	// Add the current request as a unique member scored by its timestamp.
	member := fmt.Sprintf("%d:%s", now.UnixNano(), uuid.NewString()[:slidingWindowMemberIDLen])
	pipe.ZAdd(ctx, fullKey, redis.Z{
		Score:  float64(now.UnixNano()),
		Member: member,
	})

	// Set a TTL so keys are eventually garbage-collected even if no further
	// requests arrive for this key.
	pipe.Expire(ctx, fullKey, r.window*SlidingWindowTTLMultiplier)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("sliding window rate limit: %w", err)
	}

	// The count was taken BEFORE the current request was added, so if
	// count < limit the current (just-added) request fits within the limit.
	return countCmd.Val() < int64(r.limit), nil
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
