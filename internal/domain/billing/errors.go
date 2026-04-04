package billing

import (
	"errors"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
)

var (
	ErrPlanNotFound          = errors.New("plan not found")
	ErrSubscriptionNotFound  = errors.New("subscription not found")
	ErrInvoiceNotFound       = errors.New("invoice not found")
	ErrFamilyGroupNotFound   = errors.New("family group not found")
	ErrInvoiceAlreadyPaid    = errors.New("invoice already paid")
	ErrInsufficientFunds     = errors.New("insufficient payment amount")
	ErrCurrencyMismatch      = errors.New("currency mismatch")
	ErrAddonNotAvailable     = errors.New("addon not available for this plan")
	ErrSubscriptionNotActive = errors.New("subscription is not active")
	ErrNotTrialStatus        = errors.New("subscription is not in trial status")
	ErrCheckoutRateLimited       = errors.New("checkout rate limit exceeded, try again later")
	ErrAddonAlreadyOnSubscription = errors.New("addon already added to subscription")
	ErrAddonNotOnSubscription     = errors.New("addon not found on subscription")

	// ErrPlanNotActive is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrPlanNotActive continue to work.
	ErrPlanNotActive = aggregate.ErrPlanNotActive

	// ErrNoPriceConfigured is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrNoPriceConfigured continue to work.
	ErrNoPriceConfigured = aggregate.ErrNoPriceConfigured

	// ErrFamilyNotEnabled is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrFamilyNotEnabled continue to work.
	ErrFamilyNotEnabled = aggregate.ErrFamilyNotEnabled
)
