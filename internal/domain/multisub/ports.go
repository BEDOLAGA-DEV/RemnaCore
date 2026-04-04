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

// SubscriptionInfo holds the minimal subscription data the multisub domain
// needs to orchestrate Remnawave provisioning. This is an ACL type — upstream
// subscription aggregates are projected into this struct at the adapter boundary.
type SubscriptionInfo struct {
	ID       string
	UserID   string
	PlanID   string
	AddonIDs []string
}
