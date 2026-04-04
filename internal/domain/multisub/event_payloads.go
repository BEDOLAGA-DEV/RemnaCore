package multisub

import "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"

// Payload type aliases re-exported from aggregate for backward compatibility.
type (
	BindingProvisionedPayload   = aggregate.BindingProvisionedPayload
	BindingDeprovisionedPayload = aggregate.BindingDeprovisionedPayload
	BindingFailedPayload        = aggregate.BindingFailedPayload
	BindingDisabledPayload      = aggregate.BindingDisabledPayload
	BindingEnabledPayload       = aggregate.BindingEnabledPayload
)

// BindingSyncFailedPayload is the typed payload for EventBindingSyncFailed.
type BindingSyncFailedPayload struct {
	BindingID      string `json:"binding_id"`
	SubscriptionID string `json:"subscription_id"`
	Reason         string `json:"reason"`
}

// BindingSyncCompletedPayload is the typed payload for EventBindingSyncCompleted.
type BindingSyncCompletedPayload struct {
	BindingID      string `json:"binding_id"`
	SubscriptionID string `json:"subscription_id"`
}

// BindingTrafficExceededPayload is the typed payload for EventBindingTrafficExceeded.
type BindingTrafficExceededPayload struct {
	BindingID      string `json:"binding_id"`
	SubscriptionID string `json:"subscription_id"`
	RemnawaveUUID  string `json:"remnawave_uuid"`
}

// BindingWebhookPayload is a generic typed payload used for dynamically-typed
// webhook events that carry binding identifiers. Used by SyncSaga.HandleWebhookEvent
// when the exact event type is determined at runtime.
type BindingWebhookPayload struct {
	BindingID      string `json:"binding_id"`
	SubscriptionID string `json:"subscription_id"`
	RemnawaveUUID  string `json:"remnawave_uuid"`
}
