package billing

import (
	"errors"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// MapToAPIError maps billing service-level domain errors to API error codes.
// Returns nil if the error is not a billing service-level error.
//
// Note: aggregate-level errors (defined in billing/aggregate) are mapped by
// aggregate.MapToAPIError and should be checked separately.
func MapToAPIError(err error) *apierror.Error {
	switch {
	case errors.Is(err, ErrPlanNotFound):
		return apierror.BillingPlanNotFound
	case errors.Is(err, ErrSubscriptionNotFound):
		return apierror.BillingSubscriptionNotFound
	case errors.Is(err, ErrInvoiceNotFound):
		return apierror.BillingInvoiceNotFound
	case errors.Is(err, ErrFamilyGroupNotFound):
		return apierror.BillingFamilyGroupNotFound
	case errors.Is(err, ErrInvoiceAlreadyPaid):
		return apierror.BillingInvoiceAlreadyPaid
	case errors.Is(err, ErrInsufficientFunds):
		return apierror.BillingInsufficientFunds
	case errors.Is(err, ErrCurrencyMismatch):
		return apierror.BillingCurrencyMismatch
	case errors.Is(err, ErrAddonNotAvailable):
		return apierror.BillingAddonNotAvailable
	case errors.Is(err, ErrSubscriptionNotActive):
		return apierror.BillingSubscriptionNotActive
	case errors.Is(err, ErrNotTrialStatus):
		return apierror.BillingNotTrialStatus
	case errors.Is(err, ErrCheckoutRateLimited):
		return apierror.BillingCheckoutRateLimited
	case errors.Is(err, ErrAddonAlreadyOnSubscription):
		return apierror.BillingAddonAlreadyOn
	case errors.Is(err, ErrAddonNotOnSubscription):
		return apierror.BillingAddonNotOn
	case errors.Is(err, ErrPlanNotActive):
		return apierror.BillingPlanNotActive
	case errors.Is(err, ErrNoPriceConfigured):
		return apierror.BillingNoPriceConfigured
	case errors.Is(err, ErrFamilyNotEnabled):
		return apierror.BillingFamilyNotEnabled
	case errors.Is(err, ErrPendingPlanInactive):
		return apierror.BillingPendingPlanInactive
	case errors.Is(err, ErrCancellationBlocked):
		return apierror.BillingCancellationBlocked
	case errors.Is(err, ErrCheckoutBlocked):
		return apierror.BillingCheckoutBlocked
	case errors.Is(err, ErrEmptyInvoiceID):
		return apierror.BillingEmptyInvoiceID
	default:
		return nil
	}
}
