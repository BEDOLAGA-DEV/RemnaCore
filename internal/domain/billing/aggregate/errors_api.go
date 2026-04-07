package aggregate

import (
	"errors"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// MapToAPIError maps billing aggregate-level errors to API error codes.
// Returns nil if the error is not a billing aggregate error.
func MapToAPIError(err error) *apierror.Error {
	switch {
	case errors.Is(err, ErrInvalidTransition):
		return apierror.BillingInvalidTransition
	case errors.Is(err, ErrMaxFamilyExceeded):
		return apierror.BillingMaxFamilyExceeded
	case errors.Is(err, ErrAlreadyMember):
		return apierror.BillingAlreadyMember
	case errors.Is(err, ErrCannotRemoveOwner):
		return apierror.BillingCannotRemoveOwner
	case errors.Is(err, ErrMemberNotFound):
		return apierror.BillingMemberNotFound
	case errors.Is(err, ErrEmptyPlanName):
		return apierror.BillingEmptyPlanName
	case errors.Is(err, ErrBasePriceNotPositive):
		return apierror.BillingBasePriceNotPositive
	case errors.Is(err, ErrNoCountriesAllowed):
		return apierror.BillingNoCountriesAllowed
	case errors.Is(err, ErrAddonAlreadyExists):
		return apierror.BillingAddonAlreadyExists
	case errors.Is(err, ErrAddonNotFound):
		return apierror.BillingAddonNotFound
	case errors.Is(err, ErrInvoiceRequiresLineItems):
		return apierror.BillingInvoiceRequiresItems
	case errors.Is(err, ErrInvoiceMustBeDraftForPending):
		return apierror.BillingInvoiceMustBeDraft
	case errors.Is(err, ErrInvoiceMustBePendingForPaid):
		return apierror.BillingInvoiceMustBePending
	case errors.Is(err, ErrInvoiceMustBePendingForFailed):
		return apierror.BillingInvoicePendingForFailed
	case errors.Is(err, ErrInvoiceMustBePaidForRefund):
		return apierror.BillingInvoiceMustBePaid
	case errors.Is(err, ErrSubscriptionNotActiveForRenewal):
		return apierror.BillingSubscriptionNotActive
	case errors.Is(err, ErrSubscriptionNotActiveForInvoicing):
		return apierror.BillingSubNotActiveForInvoice
	case errors.Is(err, ErrPlanAlreadyActive):
		return apierror.BillingPlanAlreadyActive
	case errors.Is(err, ErrPlanAlreadyInactive):
		return apierror.BillingPlanAlreadyInactive
	case errors.Is(err, ErrAddonNotOnPlan):
		return apierror.BillingAddonNotOnPlan
	case errors.Is(err, ErrEmptyOwnerID):
		return apierror.BillingEmptyOwnerID
	case errors.Is(err, ErrMaxMembersTooLow):
		return apierror.BillingMaxMembersTooLow
	case errors.Is(err, ErrMaxMembersTooHigh):
		return apierror.BillingMaxMembersTooHigh
	case errors.Is(err, ErrEmptyUserID):
		return apierror.BillingEmptyUserID
	case errors.Is(err, ErrEmptyPlanID):
		return apierror.BillingEmptyPlanID
	case errors.Is(err, ErrEmptyAddonID):
		return apierror.BillingEmptyAddonID
	case errors.Is(err, ErrAddonNotAvailableOnPlan):
		return apierror.BillingAddonNotAvailableOnPlan
	case errors.Is(err, ErrMaxAddonsExceeded):
		return apierror.BillingMaxAddonsExceeded
	case errors.Is(err, ErrSubscriptionNotActiveForAddon):
		return apierror.BillingSubNotActiveForAddon
	case errors.Is(err, ErrAlreadySubscribed):
		return apierror.BillingAlreadySubscribed
	case errors.Is(err, ErrCountryNotAllowed):
		return apierror.BillingCountryNotAllowed
	default:
		return nil
	}
}
