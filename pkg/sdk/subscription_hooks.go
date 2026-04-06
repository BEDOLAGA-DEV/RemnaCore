package sdk

// Subscription lifecycle hook payload types.
// These types define the JSON schema for sync hook payloads
// dispatched at key subscription lifecycle points.

// --- subscription.creating ---

// SubCreatingRequest is sent to the subscription.creating hook
// before a subscription aggregate is created.
type SubCreatingRequest struct {
	UserID   string   `json:"user_id"`
	PlanID   string   `json:"plan_id"`
	AddonIDs []string `json:"addon_ids"`
	Interval string   `json:"interval"` // e.g. "month", "year"
}

// SubCreatingResponse allows plugins to modify subscription creation parameters.
type SubCreatingResponse struct {
	TrialDays *int              `json:"trial_days,omitempty"`
	AddonIDs  []string          `json:"addon_ids,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// --- subscription.renewing ---

// SubRenewingRequest is sent to the subscription.renewing hook
// before Renew() is called on the aggregate.
type SubRenewingRequest struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	PlanID         string `json:"plan_id"`
	CurrentPeriod  string `json:"current_period"` // ISO 8601 end date
	RenewalCount   int    `json:"renewal_count"`
}

// SubRenewingResponse allows plugins to modify renewal parameters.
type SubRenewingResponse struct {
	PriceOverride    *int64  `json:"price_override,omitempty"`    // cents
	IntervalOverride *string `json:"interval_override,omitempty"` // e.g. "year"
	DiscountPercent  *int    `json:"discount_percent,omitempty"`  // 0-100
}

// --- subscription.upgrading ---

// SubUpgradingRequest is sent to the subscription.upgrading hook
// before Upgrade() is called on the aggregate.
type SubUpgradingRequest struct {
	SubscriptionID  string `json:"subscription_id"`
	UserID          string `json:"user_id"`
	FromPlanID      string `json:"from_plan_id"`
	ToPlanID        string `json:"to_plan_id"`
	ProrationCredit int64  `json:"proration_credit"` // cents
}

// SubUpgradingResponse allows plugins to modify upgrade parameters.
type SubUpgradingResponse struct {
	CreditOverride  *int64 `json:"credit_override,omitempty"`  // cents
	DiscountPercent *int   `json:"discount_percent,omitempty"` // 0-100
}

// --- subscription.cancelling ---

// SubCancellingRequest is sent to the subscription.cancelling hook
// before Cancel() is called on the aggregate.
type SubCancellingRequest struct {
	SubscriptionID string  `json:"subscription_id"`
	UserID         string  `json:"user_id"`
	PlanID         string  `json:"plan_id"`
	Reason         *string `json:"reason,omitempty"`
}

// SubCancellingResponse allows plugins to block or modify cancellation.
type SubCancellingResponse struct {
	Block           bool    `json:"block"`
	BlockReason     *string `json:"block_reason,omitempty"`
	GracePeriodDays *int    `json:"grace_period_days,omitempty"`
	RetentionOffer  *string `json:"retention_offer,omitempty"`
}

// --- subscription.limiting ---

// SubLimitingRequest is sent to the subscription.limiting hook
// when a binding's traffic is exceeded.
type SubLimitingRequest struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	PlanID         string `json:"plan_id"`
	BindingID      string `json:"binding_id"`
	UsedBytes      int64  `json:"used_bytes"`
	LimitBytes     int64  `json:"limit_bytes"`
}

// SubLimitingResponse allows plugins to control limiting behavior.
type SubLimitingResponse struct {
	Block              bool    `json:"block"`
	Notify             bool    `json:"notify"`
	UpgradeOfferPlanID *string `json:"upgrade_offer_plan_id,omitempty"`
}

// --- subscription.unlimiting ---

// SubUnlimitingRequest is sent to the subscription.unlimiting hook
// when traffic resets.
type SubUnlimitingRequest struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	BindingID      string `json:"binding_id"`
}

// SubUnlimitingResponse allows plugins to control unlimiting behavior.
type SubUnlimitingResponse struct {
	Notify bool `json:"notify"`
}
