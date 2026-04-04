package billing

import "context"

// DomainRateLimiter checks if an operation is allowed for a given user.
// Used to prevent abuse of expensive operations like checkout and subscription
// creation. Implementations live in the adapter layer (e.g. Valkey).
type DomainRateLimiter interface {
	// AllowCheckout returns true if the user is within their checkout rate limit.
	AllowCheckout(ctx context.Context, userID string) (bool, error)
	// AllowSubscriptionCreate returns true if the user is within their
	// subscription creation rate limit.
	AllowSubscriptionCreate(ctx context.Context, userID string) (bool, error)
}
