package multisub

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// Multi-subscription event types re-exported from aggregate for backward
// compatibility. New code should prefer the aggregate constants directly.
const (
	EventBindingProvisioned     = aggregate.EventBindingProvisioned
	EventBindingDeprovisioned   = aggregate.EventBindingDeprovisioned
	EventBindingSyncFailed      = aggregate.EventBindingSyncFailed
	EventBindingSyncCompleted   = aggregate.EventBindingSyncCompleted
	EventBindingTrafficExceeded = aggregate.EventBindingTrafficExceeded
	EventBindingDisabled        = aggregate.EventBindingDisabled
	EventBindingEnabled         = aggregate.EventBindingEnabled
	EventBindingFailed          = aggregate.EventBindingFailed
)

// Event is an alias for the shared domainevent.Event so that callers within the
// multisub context can reference multisub.Event without importing pkg/domainevent.
type Event = domainevent.Event

// EventType is an alias for the shared domainevent.EventType.
type EventType = domainevent.EventType

// --- Sync event factories (saga-level, not self-recorded by binding) ---

// NewBindingSyncFailedEvent creates an event when binding synchronisation fails.
func NewBindingSyncFailedEvent(bindingID, subscriptionID, reason string) Event {
	return domainevent.NewWithEntity(EventBindingSyncFailed, BindingSyncFailedPayload{
		BindingID:      bindingID,
		SubscriptionID: subscriptionID,
		Reason:         reason,
	}, bindingID)
}

// NewBindingSyncCompletedEvent creates an event when a binding sync succeeds.
func NewBindingSyncCompletedEvent(bindingID, subscriptionID string) Event {
	return domainevent.NewWithEntity(EventBindingSyncCompleted, BindingSyncCompletedPayload{
		BindingID:      bindingID,
		SubscriptionID: subscriptionID,
	}, bindingID)
}
