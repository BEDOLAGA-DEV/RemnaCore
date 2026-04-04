package billing

import "context"

// DomainRateLimiter checks if an operation is allowed for a given user.
// Used to prevent abuse of expensive operations like checkout. Implementations
// live in the adapter layer (e.g. Valkey).
type DomainRateLimiter interface {
	// AllowCheckout returns true if the user is within their checkout rate limit.
	AllowCheckout(ctx context.Context, userID string) (bool, error)
}

// AlwaysAllowRateLimiter is a no-op rate limiter that always allows operations.
// Used as default when no rate limiting infrastructure is configured.
type AlwaysAllowRateLimiter struct{}

// AllowCheckout always returns true.
func (AlwaysAllowRateLimiter) AllowCheckout(_ context.Context, _ string) (bool, error) {
	return true, nil
}
