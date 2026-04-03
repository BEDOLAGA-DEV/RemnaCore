package valkey

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	RateLimitKeyPrefix     = "ratelimit:"
	DefaultRateLimit       = 100 // requests per window
	DefaultRateLimitWindow = time.Minute

	// redisNegInf is the Redis representation of negative infinity used in
	// range queries such as ZREMRANGEBYSCORE.
	redisNegInf = "-inf"
)

// Compile-time interface satisfaction checks.
var (
	_ RateLimiter = (*SlidingWindowRateLimiter)(nil)
	_ RateLimiter = (*ValkeyRateLimiter)(nil)
	_ RateLimiter = (*InMemoryRateLimiter)(nil)
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
		return false, fmt.Errorf("fixed window rate limit: %w", err)
	}

	count := incrCmd.Val()
	return count <= int64(r.limit), nil
}

// --- Sliding window implementation (Lua script, atomic) ---

const (
	// SlidingWindowTTLMultiplier controls how long sorted set entries are
	// retained beyond the window duration. Using 2x ensures entries from the
	// previous window are available for overlap calculation.
	SlidingWindowTTLMultiplier = 2

	// slidingWindowScript runs all sliding window operations atomically
	// server-side. It removes expired entries, checks the count, conditionally
	// adds the new member, and refreshes the TTL — all in a single round-trip.
	slidingWindowScript = `
local key = KEYS[1]
local window_start = ARGV[1]
local now_score = ARGV[2]
local member = ARGV[3]
local limit = tonumber(ARGV[4])
local ttl = tonumber(ARGV[5])

redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)
local count = redis.call('ZCARD', key)
if count < limit then
    redis.call('ZADD', key, now_score, member)
end
redis.call('EXPIRE', key, ttl)
return count
`
)

// SlidingWindowRateLimiter uses Redis sorted sets to implement a sliding window
// rate limiter. Each request is stored as a member scored by its timestamp. The
// window slides continuously, preventing the burst-at-boundary problem that
// fixed-window counters suffer from.
//
// All operations are executed atomically via a Lua script to avoid race
// conditions between concurrent callers.
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
// current entries, conditionally adds the new request, and sets a TTL — all in
// a single Lua script execution for true atomicity.
func (r *SlidingWindowRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	now := time.Now()
	windowStart := now.Add(-r.window)
	fullKey := RateLimitKeyPrefix + key
	member := fmt.Sprintf("%d:%x", now.UnixNano(), rand.Uint64())
	ttlSeconds := int(r.window.Seconds()) * SlidingWindowTTLMultiplier

	count, err := r.client.Eval(ctx, slidingWindowScript, []string{fullKey},
		fmt.Sprintf("%d", windowStart.UnixNano()),
		fmt.Sprintf("%f", float64(now.UnixNano())),
		member,
		r.limit,
		ttlSeconds,
	).Int64()

	if err != nil {
		return false, fmt.Errorf("sliding window rate limit: %w", err)
	}

	return count < int64(r.limit), nil
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
