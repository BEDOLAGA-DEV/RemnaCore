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
	ErrCheckoutRateLimited = errors.New("checkout rate limit exceeded, try again later")

	// ErrAddonAlreadyOnSubscription is an alias to the aggregate-level sentinel
	// so that callers using billing.ErrAddonAlreadyOnSubscription continue to work.
	ErrAddonAlreadyOnSubscription = aggregate.ErrAddonAlreadyOnSubscription

	// ErrAddonNotOnSubscription is an alias to the aggregate-level sentinel
	// so that callers using billing.ErrAddonNotOnSubscription continue to work.
	ErrAddonNotOnSubscription = aggregate.ErrAddonNotOnSubscription

	// ErrPlanNotActive is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrPlanNotActive continue to work.
	ErrPlanNotActive = aggregate.ErrPlanNotActive

	// ErrNoPriceConfigured is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrNoPriceConfigured continue to work.
	ErrNoPriceConfigured = aggregate.ErrNoPriceConfigured

	// ErrFamilyNotEnabled is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrFamilyNotEnabled continue to work.
	ErrFamilyNotEnabled = aggregate.ErrFamilyNotEnabled

	// ErrPendingPlanInactive is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrPendingPlanInactive continue to work.
	ErrPendingPlanInactive = aggregate.ErrPendingPlanInactive

	// ErrSubscriptionNotActiveForInvoicing is an alias to the aggregate-level
	// sentinel so that callers using billing.ErrSubscriptionNotActiveForInvoicing
	// continue to work.
	ErrSubscriptionNotActiveForInvoicing = aggregate.ErrSubscriptionNotActiveForInvoicing

	// ErrPlanAlreadyActive is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrPlanAlreadyActive continue to work.
	ErrPlanAlreadyActive = aggregate.ErrPlanAlreadyActive

	// ErrPlanAlreadyInactive is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrPlanAlreadyInactive continue to work.
	ErrPlanAlreadyInactive = aggregate.ErrPlanAlreadyInactive

	// ErrAddonNotOnPlan is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrAddonNotOnPlan continue to work.
	ErrAddonNotOnPlan = aggregate.ErrAddonNotOnPlan

	// ErrEmptyOwnerID is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrEmptyOwnerID continue to work.
	ErrEmptyOwnerID = aggregate.ErrEmptyOwnerID

	// ErrMaxMembersTooLow is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrMaxMembersTooLow continue to work.
	ErrMaxMembersTooLow = aggregate.ErrMaxMembersTooLow

	// ErrMaxMembersTooHigh is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrMaxMembersTooHigh continue to work.
	ErrMaxMembersTooHigh = aggregate.ErrMaxMembersTooHigh

	// ErrEmptyUserID is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrEmptyUserID continue to work.
	ErrEmptyUserID = aggregate.ErrEmptyUserID

	// ErrEmptyPlanID is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrEmptyPlanID continue to work.
	ErrEmptyPlanID = aggregate.ErrEmptyPlanID

	// ErrEmptyInvoiceID indicates that a required invoice ID was not provided.
	ErrEmptyInvoiceID = errors.New("invoice ID is required")

	// ErrCancellationBlocked is returned when a plugin blocks subscription cancellation.
	ErrCancellationBlocked = errors.New("cancellation blocked by plugin")

	// ErrCheckoutBlocked is returned when a plugin blocks checkout via the
	// checkout.validating hook.
	ErrCheckoutBlocked = errors.New("checkout blocked by plugin")

	// ErrEmptyAddonID is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrEmptyAddonID continue to work.
	ErrEmptyAddonID = aggregate.ErrEmptyAddonID

	// ErrAddonNotAvailableOnPlan is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrAddonNotAvailableOnPlan continue to work.
	ErrAddonNotAvailableOnPlan = aggregate.ErrAddonNotAvailableOnPlan

	// ErrMaxAddonsExceeded is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrMaxAddonsExceeded continue to work.
	ErrMaxAddonsExceeded = aggregate.ErrMaxAddonsExceeded

	// ErrSubscriptionNotActiveForAddon is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrSubscriptionNotActiveForAddon continue to work.
	ErrSubscriptionNotActiveForAddon = aggregate.ErrSubscriptionNotActiveForAddon

	// ErrAlreadySubscribed is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrAlreadySubscribed continue to work.
	ErrAlreadySubscribed = aggregate.ErrAlreadySubscribed

	// ErrCountryNotAllowed is an alias to the aggregate-level sentinel so
	// that callers using billing.ErrCountryNotAllowed continue to work.
	ErrCountryNotAllowed = aggregate.ErrCountryNotAllowed
)
