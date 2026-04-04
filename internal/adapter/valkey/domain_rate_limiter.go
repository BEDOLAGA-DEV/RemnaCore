package valkey

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
)

const (
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
// client. The checkout rate limit threshold is read from cfg; if zero, the
// platform default is used.
func NewDomainRateLimiter(client *redis.Client, cfg *config.Config) *DomainRateLimiter {
	checkoutMax := cfg.RateLimit.CheckoutMaxPerHour
	if checkoutMax == 0 {
		checkoutMax = config.DefaultCheckoutMaxPerHour
	}

	return &DomainRateLimiter{
		checkoutLimiter: NewSlidingWindowRateLimiter(client, checkoutMax, CheckoutRateLimitWindow),
	}
}

// AllowCheckout checks if the user is within their checkout rate limit.
func (r *DomainRateLimiter) AllowCheckout(ctx context.Context, userID string) (bool, error) {
	return r.checkoutLimiter.Allow(ctx, CheckoutRateLimitKeyPrefix+userID)
}
