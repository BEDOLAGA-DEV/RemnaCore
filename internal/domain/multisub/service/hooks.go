package service

import "context"

// SyncHookFn dispatches a synchronous hook and returns the typed response.
// hookName identifies the dispatch point; payload is marshaled to JSON by
// the implementation. Returns (nil, nil) when no hook is registered or the
// implementation decides to swallow the error (best-effort semantics).
//
// This function type replaces the direct hookdispatch.Dispatcher dependency,
// keeping the multisub domain free of plugin-infrastructure imports. The
// concrete implementation is wired in internal/app/wiring_multisub.go.
type SyncHookFn func(ctx context.Context, hookName string, payload any) ([]byte, error)

// AsyncHookFn dispatches an asynchronous (fire-and-forget) hook. Errors are
// logged by the implementation but never propagated to the caller.
//
// This function type replaces the direct hookdispatch.Dispatcher dependency
// for post-commit notifications.
type AsyncHookFn func(ctx context.Context, hookName string, payload any)

// LimitingHookFn dispatches the subscription.limiting sync hook and returns
// a typed response. The implementation handles JSON marshaling, plugin dispatch,
// and response deserialization. Returns (nil, nil) when no hook is registered.
type LimitingHookFn func(ctx context.Context, payload SubLimitingPayload) (*SubLimitingResponse, error)

// Hook name constants for traffic lifecycle dispatch points.
// Pre-hooks (sync) are named "{entity}.{verb}ing" and fire before the state
// transition. Post-hooks (async) are named "{entity}.{past_tense}.post" and
// fire after the transition completes.
const (
	HookSubLimiting       = "subscription.limiting"
	HookSubLimitedPost    = "subscription.limited.post"
	HookSubUnlimiting     = "subscription.unlimiting"
	HookSubUnlimitedPost  = "subscription.unlimited.post"
	HookSubTrafficWarning = "subscription.traffic_warning.post"
)

// --- Domain-local hook payload structs ---
//
// These mirror the wire format of pkg/sdk hook types but live in the domain
// package so that service methods can construct payloads without importing
// the plugin SDK. The wiring layer (internal/app/) marshals these to JSON
// when dispatching through the real hookdispatch.Dispatcher.

// SubLimitingPayload is the hook payload for subscription.limiting.
type SubLimitingPayload struct {
	SubscriptionID string `json:"subscription_id"`
	BindingID      string `json:"binding_id"`
	UsedBytes      int64  `json:"used_bytes"`
	LimitBytes     int64  `json:"limit_bytes"`
}

// SubLimitingResponse is the hook response for subscription.limiting.
type SubLimitingResponse struct {
	Block bool `json:"block"`
}

// SubUnlimitingPayload is the hook payload for subscription.unlimiting.
type SubUnlimitingPayload struct {
	SubscriptionID string `json:"subscription_id"`
	BindingID      string `json:"binding_id"`
}

// SubUnlimitingResponse is the hook response for subscription.unlimiting.
type SubUnlimitingResponse struct {
	Notify bool `json:"notify"`
}

// SubLimitedNotification is the async payload for subscription.limited.post.
type SubLimitedNotification struct {
	SubscriptionID string `json:"subscription_id"`
	BindingID      string `json:"binding_id"`
	UsedBytes      int64  `json:"used_bytes"`
	LimitBytes     int64  `json:"limit_bytes"`
}

// SubUnlimitedNotification is the async payload for subscription.unlimited.post.
type SubUnlimitedNotification struct {
	SubscriptionID string `json:"subscription_id"`
	BindingID      string `json:"binding_id"`
}

// SubTrafficWarningNotification is the async payload for subscription.traffic_warning.post.
type SubTrafficWarningNotification struct {
	SubscriptionID string `json:"subscription_id"`
	BindingID      string `json:"binding_id"`
	UsedBytes      int64  `json:"used_bytes"`
	LimitBytes     int64  `json:"limit_bytes"`
	ThresholdPct   int    `json:"threshold_pct"`
}
