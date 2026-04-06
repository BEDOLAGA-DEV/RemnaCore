package sdk

// Checkout hook payload types.

// --- checkout.validating ---

// CheckoutValidatingRequest is sent to the checkout.validating hook
// before CreateSubscription is called.
type CheckoutValidatingRequest struct {
	UserID    string   `json:"user_id"`
	PlanID    string   `json:"plan_id"`
	AddonIDs  []string `json:"addon_ids"`
	UserEmail string   `json:"user_email"`
	IP        *string  `json:"ip,omitempty"`
	Country   *string  `json:"country,omitempty"`
}

// CheckoutValidatingResponse allows plugins to block or modify checkout.
type CheckoutValidatingResponse struct {
	Allowed        bool     `json:"allowed"`
	Reason         *string  `json:"reason,omitempty"`
	ModifiedAddons []string `json:"modified_addon_ids,omitempty"`
}

// --- checkout.completed ---

// CheckoutCompletedNotification is the async payload sent after successful checkout.
type CheckoutCompletedNotification struct {
	SubscriptionID string `json:"subscription_id"`
	InvoiceID      string `json:"invoice_id"`
	UserID         string `json:"user_id"`
	PlanID         string `json:"plan_id"`
	AmountCents    int64  `json:"amount_cents"`
	Currency       string `json:"currency"`
}

// --- Post-lifecycle async hooks ---

// SubActivatedNotification is the async payload for subscription.activated.post.
type SubActivatedNotification struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	PlanID         string `json:"plan_id"`
}

// SubCancelledNotification is the async payload for subscription.cancelled.post.
type SubCancelledNotification struct {
	SubscriptionID string  `json:"subscription_id"`
	UserID         string  `json:"user_id"`
	CancelReason   *string `json:"cancel_reason,omitempty"`
}

// SubRenewedNotification is the async payload for subscription.renewed.post.
type SubRenewedNotification struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	NewPeriodEnd   string `json:"new_period_end"` // RFC3339
}

// SubLimitedNotification is the async payload for subscription.limited.post.
type SubLimitedNotification struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	BindingID      string `json:"binding_id"`
	UsedBytes      int64  `json:"used_bytes"`
	LimitBytes     int64  `json:"limit_bytes"`
}

// SubUnlimitedNotification is the async payload for subscription.unlimited.post.
type SubUnlimitedNotification struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	BindingID      string `json:"binding_id"`
}

// SubTrafficWarningNotification is the async payload for subscription.traffic_warning.post.
type SubTrafficWarningNotification struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	BindingID      string `json:"binding_id"`
	UsedBytes      int64  `json:"used_bytes"`
	LimitBytes     int64  `json:"limit_bytes"`
	ThresholdPct   int    `json:"threshold_pct"` // e.g. 80, 90
}
