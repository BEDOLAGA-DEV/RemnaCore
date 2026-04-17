package balance

// Balance domain events.
const (
	EventTopUpInitiated    = "balance.topup_initiated"
	EventTopUpCompleted    = "balance.topup_completed"
	EventTopUpFailed       = "balance.topup_failed"
	EventConsumed          = "balance.consumed"
	EventRefunded          = "balance.refunded"
	EventAdjustedByAdmin   = "balance.adjusted_by_admin"
	EventTransferred       = "balance.transferred"
	EventCashbackEarned    = "balance.cashback_earned"
	EventBonusExpired      = "balance.bonus_expired"
	EventHeld              = "balance.held"
	EventReleased          = "balance.released"
	EventKYCRequired       = "balance.kyc_required"
)

// Hook names for balance plugin integration.
const (
	HookPaymentCreateCharge = "payment.create_charge" // balance as payment provider
)
