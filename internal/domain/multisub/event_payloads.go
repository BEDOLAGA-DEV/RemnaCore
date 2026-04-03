package multisub

// BindingProvisionedPayload is the typed payload for EventBindingProvisioned.
type BindingProvisionedPayload struct {
	BindingID      string `json:"binding_id"`
	SubscriptionID string `json:"subscription_id"`
	RemnawaveUUID  string `json:"remnawave_uuid"`
	Purpose        string `json:"purpose"`
}

// BindingDeprovisionedPayload is the typed payload for EventBindingDeprovisioned.
type BindingDeprovisionedPayload struct {
	BindingID      string `json:"binding_id"`
	SubscriptionID string `json:"subscription_id"`
	RemnawaveUUID  string `json:"remnawave_uuid"`
}

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
