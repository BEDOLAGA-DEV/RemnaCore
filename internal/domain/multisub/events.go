package multisub

import "github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"

// Multi-subscription event types.
const (
	EventBindingProvisioned     domainevent.EventType = "binding.provisioned"
	EventBindingDeprovisioned   domainevent.EventType = "binding.deprovisioned"
	EventBindingSyncFailed      domainevent.EventType = "binding.sync_failed"
	EventBindingSyncCompleted   domainevent.EventType = "binding.sync_completed"
	EventBindingTrafficExceeded domainevent.EventType = "binding.traffic_exceeded"
)

// Event is an alias for the shared domainevent.Event so that callers within the
// multisub context can reference multisub.Event without importing pkg/domainevent.
type Event = domainevent.Event

// EventType is an alias for the shared domainevent.EventType.
type EventType = domainevent.EventType

// NewBindingProvisionedEvent creates an event when a binding is provisioned in Remnawave.
func NewBindingProvisionedEvent(bindingID, subscriptionID, remnawaveUUID, purpose string) Event {
	return domainevent.NewWithEntity(EventBindingProvisioned, BindingProvisionedPayload{
		BindingID:      bindingID,
		SubscriptionID: subscriptionID,
		RemnawaveUUID:  remnawaveUUID,
		Purpose:        purpose,
	}, bindingID)
}

// NewBindingDeprovisionedEvent creates an event when a binding is removed from Remnawave.
func NewBindingDeprovisionedEvent(bindingID, subscriptionID, remnawaveUUID string) Event {
	return domainevent.NewWithEntity(EventBindingDeprovisioned, BindingDeprovisionedPayload{
		BindingID:      bindingID,
		SubscriptionID: subscriptionID,
		RemnawaveUUID:  remnawaveUUID,
	}, bindingID)
}

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

// NewBindingTrafficExceededEvent creates an event when a binding exceeds its traffic limit.
func NewBindingTrafficExceededEvent(bindingID, subscriptionID, remnawaveUUID string) Event {
	return domainevent.NewWithEntity(EventBindingTrafficExceeded, BindingTrafficExceededPayload{
		BindingID:      bindingID,
		SubscriptionID: subscriptionID,
		RemnawaveUUID:  remnawaveUUID,
	}, bindingID)
}
