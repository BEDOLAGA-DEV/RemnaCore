package checkout

// Checkout domain events.
const (
	EventSessionCreated   = "checkout.session_created"
	EventSessionConfirmed = "checkout.session_confirmed"
	EventSessionCompleted = "checkout.session_completed"
	EventSessionFailed    = "checkout.session_failed"
	EventSessionExpired   = "checkout.session_expired"
	EventSessionAbandoned = "checkout.session_abandoned"
	EventSessionCancelled = "checkout.session_cancelled"
	EventSplitInitiated   = "checkout.split_initiated"
	EventPromoApplied     = "checkout.promo_applied"
	EventMethodSaved      = "checkout.method_saved"
	EventMethodRemoved    = "checkout.method_removed"
)

// Sync hook names.
const (
	HookSessionValidating = "checkout.session_validating"
	HookSourceAvailable   = "checkout.source_available"
	HookPriceFinal        = "checkout.price_final"
)

// Async hook names.
const (
	HookSessionCreated   = "checkout.session_created"
	HookSessionConfirmed = "checkout.session_confirmed"
	HookSessionCompleted = "checkout.session_completed"
	HookSessionFailed    = "checkout.session_failed"
	HookSessionAbandoned = "checkout.session_abandoned"
)
