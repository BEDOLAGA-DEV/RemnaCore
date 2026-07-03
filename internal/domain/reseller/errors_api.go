package reseller

import (
	"errors"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// MapToAPIError maps reseller domain errors to API error codes.
// Returns nil if the error is not a reseller domain error.
func MapToAPIError(err error) *apierror.Error {
	switch {
	case errors.Is(err, ErrTenantAlreadyExists):
		return apierror.ResellerTenantAlreadyExists
	case errors.Is(err, ErrResellerAlreadyExists):
		return apierror.ResellerAccountAlreadyExists
	case errors.Is(err, ErrCommissionAlreadyExists):
		return apierror.ResellerCommissionAlreadyExists
	case errors.Is(err, ErrTenantNotFound):
		return apierror.ResellerTenantNotFound
	case errors.Is(err, ErrResellerNotFound):
		return apierror.ResellerAccountNotFound
	case errors.Is(err, ErrCommissionNotFound):
		return apierror.ResellerCommissionNotFound
	case errors.Is(err, ErrInvalidCommissionRate):
		return apierror.ResellerInvalidCommission
	case errors.Is(err, ErrInvalidAPIKey):
		return apierror.ResellerInvalidAPIKey
	case errors.Is(err, ErrTenantInactive):
		return apierror.ResellerTenantInactive
	case errors.Is(err, ErrDuplicateDomain):
		return apierror.ResellerDuplicateDomain
	case errors.Is(err, ErrNotFound):
		return apierror.ResellerNotFound
	case errors.Is(err, ErrShopBotInvalidToken):
		return apierror.ValidationFailed
	case errors.Is(err, ErrShopBotInvalidCabinetURL):
		return apierror.ValidationFailed
	case errors.Is(err, ErrShopBotInvalidPlugin):
		return apierror.ValidationFailed
	case errors.Is(err, ErrShopBotTokenInUse):
		return apierror.ResellerShopBotTokenInUse
	default:
		return nil
	}
}
