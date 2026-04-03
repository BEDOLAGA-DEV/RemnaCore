package billing

import "errors"

var (
	ErrPlanNotFound          = errors.New("plan not found")
	ErrSubscriptionNotFound  = errors.New("subscription not found")
	ErrInvoiceNotFound       = errors.New("invoice not found")
	ErrFamilyGroupNotFound   = errors.New("family group not found")
	ErrMaxBindingsExceeded   = errors.New("maximum remnawave bindings exceeded")
	ErrFamilyNotEnabled      = errors.New("family not enabled for this plan")
	ErrInvoiceAlreadyPaid    = errors.New("invoice already paid")
	ErrInsufficientFunds     = errors.New("insufficient payment amount")
	ErrCurrencyMismatch      = errors.New("currency mismatch")
	ErrAddonNotAvailable     = errors.New("addon not available for this plan")
	ErrSubscriptionNotActive = errors.New("subscription is not active")
	ErrNotTrialStatus        = errors.New("subscription is not in trial status")
)
