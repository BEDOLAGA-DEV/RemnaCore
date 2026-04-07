package multisub

import (
	"errors"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// MapToAPIError maps multisub domain errors to API error codes.
// Returns nil if the error is not a multisub domain error.
func MapToAPIError(err error) *apierror.Error {
	switch {
	case errors.Is(err, ErrBindingNotFound):
		return apierror.MultiSubBindingNotFound
	case errors.Is(err, ErrProvisioningFailed):
		return apierror.MultiSubProvisioningFailed
	case errors.Is(err, ErrDeprovisioningFailed):
		return apierror.MultiSubDeprovisioningFailed
	case errors.Is(err, ErrSyncFailed):
		return apierror.MultiSubSyncFailed
	case errors.Is(err, ErrBindingAlreadyActive):
		return apierror.MultiSubBindingAlreadyActive
	case errors.Is(err, ErrRemnawaveUnavailable):
		return apierror.MultiSubRemnawaveUnavailable
	case errors.Is(err, ErrMaxBindingsExceeded):
		return apierror.MultiSubMaxBindingsExceeded
	case errors.Is(err, ErrSagaNotFound):
		return apierror.MultiSubSagaNotFound
	case errors.Is(err, ErrSagaAlreadyExists):
		return apierror.MultiSubSagaAlreadyExists
	case errors.Is(err, ErrBindingAlreadyLimited):
		return apierror.MultiSubBindingAlreadyLimited
	case errors.Is(err, ErrBindingNotLimited):
		return apierror.MultiSubBindingNotLimited
	case errors.Is(err, ErrNoVPNPlugin):
		return apierror.MultiSubNoVPNPlugin
	default:
		return nil
	}
}
