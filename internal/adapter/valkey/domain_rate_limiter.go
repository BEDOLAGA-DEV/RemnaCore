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

	// CheckoutRateLimitKeyPrefix is the Valkey key prefix for checkout rate
	// limit counters.
	CheckoutRateLimitKeyPrefix = "domain:ratelimit:checkout:"
)

// DomainRateLimiter implements billing.DomainRateLimiter using Valkey sliding
// window rate limiters. It reuses the same SlidingWindowRateLimiter
// implementation used by the HTTP middleware but with domain-specific limits.
type DomainRateLimiter struct {
	checkoutLimiter *SlidingWindowRateLimiter
}

// NewDomainRateLimiter creates a DomainRateLimiter backed by the given Valkey
// client with predefined limits for checkout.
func NewDomainRateLimiter(client *redis.Client) *DomainRateLimiter {
	return &DomainRateLimiter{
		checkoutLimiter: NewSlidingWindowRateLimiter(client, CheckoutRateLimitMax, CheckoutRateLimitWindow),
	}
}

// AllowCheckout checks if the user is within their checkout rate limit.
func (r *DomainRateLimiter) AllowCheckout(ctx context.Context, userID string) (bool, error) {
	return r.checkoutLimiter.Allow(ctx, CheckoutRateLimitKeyPrefix+userID)
}
