package aggregate

import "github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"

// Event type constants for multisub aggregates. Defined in the aggregate
// package so that aggregate methods can record events without circular imports.
const (
	EventBindingProvisioned     domainevent.EventType = "binding.provisioned"
	EventBindingDeprovisioned   domainevent.EventType = "binding.deprovisioned"
	EventBindingSyncFailed      domainevent.EventType = "binding.sync_failed"
	EventBindingSyncCompleted   domainevent.EventType = "binding.sync_completed"
	EventBindingTrafficExceeded domainevent.EventType = "binding.traffic_exceeded"
	EventBindingDisabled        domainevent.EventType = "binding.disabled"
	EventBindingEnabled         domainevent.EventType = "binding.enabled"
	EventBindingFailed          domainevent.EventType = "binding.failed"
)

// --- Binding event payloads ---

// BindingProvisionedPayload is the typed payload for EventBindingProvisioned.
type BindingProvisionedPayload struct {
	BindingID      string `json:"binding_id"`
	SubscriptionID string `json:"subscription_id"`
	RemnawaveUUID  string `json:"remnawave_uuid"`
	ShortUUID      string `json:"short_uuid"`
}

// BindingDeprovisionedPayload is the typed payload for EventBindingDeprovisioned.
type BindingDeprovisionedPayload struct {
	BindingID      string `json:"binding_id"`
	SubscriptionID string `json:"subscription_id"`
	RemnawaveUUID  string `json:"remnawave_uuid"`
}

// BindingFailedPayload is the typed payload for EventBindingFailed.
type BindingFailedPayload struct {
	BindingID      string `json:"binding_id"`
	SubscriptionID string `json:"subscription_id"`
	Reason         string `json:"reason"`
}

// BindingDisabledPayload is the typed payload for EventBindingDisabled.
type BindingDisabledPayload struct {
	BindingID      string `json:"binding_id"`
	SubscriptionID string `json:"subscription_id"`
}

// BindingEnabledPayload is the typed payload for EventBindingEnabled.
type BindingEnabledPayload struct {
	BindingID      string `json:"binding_id"`
	SubscriptionID string `json:"subscription_id"`
}

// --- EventPayload interface implementations ---

func (BindingProvisionedPayload) EventType() domainevent.EventType   { return EventBindingProvisioned }
func (BindingDeprovisionedPayload) EventType() domainevent.EventType { return EventBindingDeprovisioned }
func (BindingFailedPayload) EventType() domainevent.EventType        { return EventBindingFailed }
func (BindingDisabledPayload) EventType() domainevent.EventType      { return EventBindingDisabled }
func (BindingEnabledPayload) EventType() domainevent.EventType       { return EventBindingEnabled }

// Compile-time interface checks.
var (
	_ domainevent.EventPayload = BindingProvisionedPayload{}
	_ domainevent.EventPayload = BindingDeprovisionedPayload{}
	_ domainevent.EventPayload = BindingFailedPayload{}
	_ domainevent.EventPayload = BindingDisabledPayload{}
	_ domainevent.EventPayload = BindingEnabledPayload{}
)
