package valkey

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// CheckoutRateLimitMax is the maximum number of checkout attempts per user
	// within the checkout rate limit window.
	CheckoutRateLimitMax = 10

	// CheckoutRateLimitWindow is the sliding window duration for checkout rate
	// limiting.
	CheckoutRateLimitWindow = 1 * time.Hour

	// SubscriptionRateLimitMax is the maximum number of subscription creation
	// attempts per user within the subscription rate limit window.
	SubscriptionRateLimitMax = 5

	// SubscriptionRateLimitWindow is the sliding window duration for
	// subscription creation rate limiting.
	SubscriptionRateLimitWindow = 24 * time.Hour

	// CheckoutRateLimitKeyPrefix is the Valkey key prefix for checkout rate
	// limit counters.
	CheckoutRateLimitKeyPrefix = "domain:ratelimit:checkout:"

	// SubscriptionRateLimitKeyPrefix is the Valkey key prefix for subscription
	// creation rate limit counters.
	SubscriptionRateLimitKeyPrefix = "domain:ratelimit:subscription_create:"
)

// DomainRateLimiter implements billing.DomainRateLimiter using Valkey sliding
// window rate limiters. It reuses the same SlidingWindowRateLimiter
// implementation used by the HTTP middleware but with domain-specific limits.
type DomainRateLimiter struct {
	checkoutLimiter     *SlidingWindowRateLimiter
	subscriptionLimiter *SlidingWindowRateLimiter
}

// NewDomainRateLimiter creates a DomainRateLimiter backed by the given Valkey
// client with predefined limits for checkout and subscription creation.
func NewDomainRateLimiter(client *redis.Client) *DomainRateLimiter {
	return &DomainRateLimiter{
		checkoutLimiter:     NewSlidingWindowRateLimiter(client, CheckoutRateLimitMax, CheckoutRateLimitWindow),
		subscriptionLimiter: NewSlidingWindowRateLimiter(client, SubscriptionRateLimitMax, SubscriptionRateLimitWindow),
	}
}

// AllowCheckout checks if the user is within their checkout rate limit.
func (r *DomainRateLimiter) AllowCheckout(ctx context.Context, userID string) (bool, error) {
	return r.checkoutLimiter.Allow(ctx, CheckoutRateLimitKeyPrefix+userID)
}

// AllowSubscriptionCreate checks if the user is within their subscription
// creation rate limit.
func (r *DomainRateLimiter) AllowSubscriptionCreate(ctx context.Context, userID string) (bool, error) {
	return r.subscriptionLimiter.Allow(ctx, SubscriptionRateLimitKeyPrefix+userID)
}
