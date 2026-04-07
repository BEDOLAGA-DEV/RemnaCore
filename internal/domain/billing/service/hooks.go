package service

import "context"

// --- Typed hook function types ---
//
// Each sync hook has its own typed function that accepts a domain payload and
// returns a typed response pointer. Returns (nil, nil) when no hook is
// registered or the implementation decides to swallow the error (best-effort
// semantics). The wiring layer (internal/app/wiring_billing.go) creates
// concrete implementations that handle JSON marshaling, plugin dispatch, and
// response deserialization.

// CancelHookFn dispatches the subscription.cancelling sync hook.
// Returns (nil, nil) when no hook is registered.
type CancelHookFn func(ctx context.Context, payload CancellingPayload) (*CancellingResponse, error)

// RenewHookFn dispatches the subscription.renewing sync hook.
// Returns (nil, nil) when no hook is registered.
type RenewHookFn func(ctx context.Context, payload RenewingPayload) (*RenewingResponse, error)

// UpgradeHookFn dispatches the subscription.upgrading sync hook.
// Returns (nil, nil) when no hook is registered.
type UpgradeHookFn func(ctx context.Context, payload UpgradingPayload) (*UpgradingResponse, error)

// CheckoutValidatingHookFn dispatches the checkout.validating sync hook.
// Returns (nil, nil) when no hook is registered.
type CheckoutValidatingHookFn func(ctx context.Context, payload CheckoutValidatingPayload) (*CheckoutValidatingResponse, error)

// SubCreatingHookFn dispatches the subscription.creating sync hook.
// Returns (nil, nil) when no hook is registered.
type SubCreatingHookFn func(ctx context.Context, payload SubCreatingPayload) (*SubCreatingResponse, error)

// AsyncHookFn dispatches an asynchronous (fire-and-forget) hook. Errors are
// logged by the implementation but never propagated to the caller.
//
// This function type replaces the direct hookdispatch.Dispatcher dependency
// for post-commit notifications.
type AsyncHookFn func(ctx context.Context, hookName string, payload any)

// Hook name constants for subscription lifecycle dispatch points.
// Pre-hooks (sync) are named "{entity}.{verb}ing" and fire before the aggregate
// transition. Post-hooks (async) are named "{entity}.{past_tense}.post" and
// fire after the transaction commits.
const (
	HookSubCancelling     = "subscription.cancelling"
	HookSubCancelledPost  = "subscription.cancelled.post"
	HookSubActivatedPost  = "subscription.activated.post"
	HookSubPausedPost     = "subscription.paused.post"
	HookSubResumedPost    = "subscription.resumed.post"
	HookSubRenewing       = "subscription.renewing"
	HookSubRenewedPost    = "subscription.renewed.post"
	HookSubUpgrading      = "subscription.upgrading"
	HookSubUpgradedPost   = "subscription.upgraded.post"
	HookSubDowngradedPost = "subscription.downgraded.post"
	HookSubExpiredPost    = "subscription.expired.post"
)

// Hook names dispatched by CheckoutService during the checkout flow.
const (
	// HookCheckoutValidating is dispatched before CreateSubscription to allow
	// plugins to perform custom eligibility checks (e.g., geo-blocking,
	// anti-fraud). Sync hook: a plugin can block the checkout.
	HookCheckoutValidating = "checkout.validating"

	// HookSubscriptionCreating is dispatched before CreateSubscription to allow
	// plugins to modify subscription parameters (trial_days, addon_ids).
	// Sync hook: best-effort, errors fall back to defaults.
	HookSubscriptionCreating = "subscription.creating"

	// HookCheckoutCompleted is dispatched after a successful CompleteCheckout
	// transaction. Async hook: fire-and-forget for analytics/notifications.
	HookCheckoutCompleted = "checkout.completed"
)

// --- Domain-local hook payload structs ---
//
// These mirror the wire format of pkg/sdk hook types but live in the domain
// package so that service methods can construct payloads without importing
// the plugin SDK. The wiring layer (internal/app/) marshals these to JSON
// when dispatching through the real hookdispatch.Dispatcher.

// CancellingPayload is the hook payload for subscription.cancelling.
type CancellingPayload struct {
	SubscriptionID string  `json:"subscription_id"`
	UserID         string  `json:"user_id"`
	PlanID         string  `json:"plan_id"`
	Reason         *string `json:"reason,omitempty"`
}

// CancellingResponse is the hook response for subscription.cancelling.
type CancellingResponse struct {
	Block bool `json:"block"`
}

// ActivatedPayload is the async hook payload for subscription.activated.post.
type ActivatedPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	PlanID         string `json:"plan_id"`
}

// RenewingPayload is the hook payload for subscription.renewing.
type RenewingPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	PlanID         string `json:"plan_id"`
	CurrentPeriod  string `json:"current_period"`
}

// RenewingResponse is the hook response for subscription.renewing.
type RenewingResponse struct {
	PriceOverride   *int64 `json:"price_override,omitempty"`
	DiscountPercent *int   `json:"discount_percent,omitempty"`
}

// UpgradingPayload is the hook payload for subscription.upgrading.
type UpgradingPayload struct {
	SubscriptionID  string `json:"subscription_id"`
	UserID          string `json:"user_id"`
	FromPlanID      string `json:"from_plan_id"`
	ToPlanID        string `json:"to_plan_id"`
	ProrationCredit int64  `json:"proration_credit"`
}

// UpgradingResponse is the hook response for subscription.upgrading.
type UpgradingResponse struct {
	CreditOverride *int64 `json:"credit_override,omitempty"`
}

// SubAsyncPayload is the payload for simple post-commit async hooks
// that only need subscription identification fields.
type SubAsyncPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	PlanID         string `json:"plan_id"`
}

// SubUpgradeAsyncPayload is the payload for upgrade/downgrade
// post-commit async hooks that include both old and new plan IDs.
type SubUpgradeAsyncPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	FromPlanID     string `json:"from_plan_id"`
	ToPlanID       string `json:"to_plan_id"`
}

// CheckoutValidatingPayload is the hook payload for checkout.validating.
type CheckoutValidatingPayload struct {
	UserID    string   `json:"user_id"`
	PlanID    string   `json:"plan_id"`
	AddonIDs  []string `json:"addon_ids"`
	UserEmail string   `json:"user_email"`
}

// CheckoutValidatingResponse is the hook response for checkout.validating.
type CheckoutValidatingResponse struct {
	Allowed bool    `json:"allowed"`
	Reason  *string `json:"reason,omitempty"`
}

// SubCreatingPayload is the hook payload for subscription.creating.
type SubCreatingPayload struct {
	UserID   string   `json:"user_id"`
	PlanID   string   `json:"plan_id"`
	AddonIDs []string `json:"addon_ids"`
}

// SubCreatingResponse is the hook response for subscription.creating.
type SubCreatingResponse struct {
	AddonIDs []string `json:"addon_ids,omitempty"`
}

// CheckoutCompletedPayload is the async hook payload for checkout.completed.
type CheckoutCompletedPayload struct {
	SubscriptionID string `json:"subscription_id"`
	InvoiceID      string `json:"invoice_id"`
	UserID         string `json:"user_id"`
	PlanID         string `json:"plan_id"`
	AmountCents    int64  `json:"amount_cents"`
	Currency       string `json:"currency"`
}
