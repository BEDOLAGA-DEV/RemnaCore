package multisub

import "context"

// PlanProvider is multisub's port for retrieving plan information from an
// external bounded context. The adapter layer (e.g. billing_lookup.go)
// implements this interface and translates upstream types into the multisub
// Anti-Corruption Layer type PlanSnapshot.
type PlanProvider interface {
	GetPlanSnapshot(ctx context.Context, planID string) (PlanSnapshot, error)
}

// SubscriptionProvider is multisub's port for retrieving subscription
// information needed by the orchestrator to enrich sparse domain events.
// The adapter layer implements this interface, translating upstream data
// into multisub-local types.
type SubscriptionProvider interface {
	GetSubscriptionInfo(ctx context.Context, subscriptionID string) (SubscriptionInfo, error)
	GetFamilyMemberIDs(ctx context.Context, ownerID string) ([]string, error)
}

// SubscriptionStatus is the multisub domain's Anti-Corruption Layer type for
// subscription status. The adapter boundary translates billing-specific status
// values into these constants so the multisub domain never imports billing types.
type SubscriptionStatus string

const (
	SubStatusActive    SubscriptionStatus = "active"
	SubStatusCancelled SubscriptionStatus = "cancelled"
	SubStatusExpired   SubscriptionStatus = "expired"
	SubStatusPaused    SubscriptionStatus = "paused"
	SubStatusTrial     SubscriptionStatus = "trial"
	SubStatusPastDue   SubscriptionStatus = "past_due"
)

// IsTerminal returns true if the subscription status is a terminal state
// (cancelled or expired) where no further provisioning should occur.
func (s SubscriptionStatus) IsTerminal() bool {
	return s == SubStatusCancelled || s == SubStatusExpired
}

// SubscriptionInfo holds the minimal subscription data the multisub domain
// needs to orchestrate Remnawave provisioning. This is an ACL type — upstream
// subscription aggregates are projected into this struct at the adapter boundary.
type SubscriptionInfo struct {
	ID       string
	UserID   string
	PlanID   string
	Status   SubscriptionStatus
	AddonIDs []string
}
