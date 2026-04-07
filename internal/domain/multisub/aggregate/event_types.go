package aggregate

import "github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"

// Event type constants for multisub aggregates. Defined in the aggregate
// package so that aggregate methods can record events without circular imports.
const (
	EventBindingCreated         domainevent.EventType = "binding.created"
	EventBindingProvisioned     domainevent.EventType = "binding.provisioned"
	EventBindingDeprovisioned   domainevent.EventType = "binding.deprovisioned"
	EventBindingSyncFailed      domainevent.EventType = "binding.sync_failed"
	EventBindingSyncCompleted   domainevent.EventType = "binding.sync_completed"
	EventBindingTrafficExceeded domainevent.EventType = "binding.traffic_exceeded"
	EventBindingLimited         domainevent.EventType = "binding.limited"
	EventBindingUnlimited       domainevent.EventType = "binding.unlimited"
	EventBindingDisabled        domainevent.EventType = "binding.disabled"
	EventBindingEnabled         domainevent.EventType = "binding.enabled"
	EventBindingFailed          domainevent.EventType = "binding.failed"
)

// --- Binding event payloads ---

// BindingCreatedPayload is the typed payload for EventBindingCreated.
// Recorded when a new RemnawaveBinding is constructed via NewBinding.
type BindingCreatedPayload struct {
	BindingID      string `json:"binding_id"`
	SubscriptionID string `json:"subscription_id"`
	Purpose        string `json:"purpose"`
}

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

// BindingLimitedPayload is the typed payload for EventBindingLimited.
// Recorded when the VPN provider reports that traffic has been exceeded.
type BindingLimitedPayload struct {
	BindingID      string `json:"binding_id"`
	SubscriptionID string `json:"subscription_id"`
	RemnawaveUUID  string `json:"remnawave_uuid"`
	Reason         string `json:"reason"`
}

// BindingUnlimitedPayload is the typed payload for EventBindingUnlimited.
// Recorded when the VPN provider lifts the traffic restriction.
type BindingUnlimitedPayload struct {
	BindingID      string `json:"binding_id"`
	SubscriptionID string `json:"subscription_id"`
	RemnawaveUUID  string `json:"remnawave_uuid"`
}

// --- EventPayload interface implementations ---

func (BindingCreatedPayload) EventType() domainevent.EventType     { return EventBindingCreated }
func (BindingProvisionedPayload) EventType() domainevent.EventType { return EventBindingProvisioned }
func (BindingDeprovisionedPayload) EventType() domainevent.EventType {
	return EventBindingDeprovisioned
}
func (BindingFailedPayload) EventType() domainevent.EventType    { return EventBindingFailed }
func (BindingDisabledPayload) EventType() domainevent.EventType  { return EventBindingDisabled }
func (BindingEnabledPayload) EventType() domainevent.EventType   { return EventBindingEnabled }
func (BindingLimitedPayload) EventType() domainevent.EventType   { return EventBindingLimited }
func (BindingUnlimitedPayload) EventType() domainevent.EventType { return EventBindingUnlimited }

// Compile-time interface checks.
var (
	_ domainevent.EventPayload = BindingCreatedPayload{}
	_ domainevent.EventPayload = BindingProvisionedPayload{}
	_ domainevent.EventPayload = BindingDeprovisionedPayload{}
	_ domainevent.EventPayload = BindingFailedPayload{}
	_ domainevent.EventPayload = BindingDisabledPayload{}
	_ domainevent.EventPayload = BindingEnabledPayload{}
	_ domainevent.EventPayload = BindingLimitedPayload{}
	_ domainevent.EventPayload = BindingUnlimitedPayload{}
)
