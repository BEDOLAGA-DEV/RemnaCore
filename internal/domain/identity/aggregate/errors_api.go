package aggregate

import (
	"errors"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// MapToAPIError maps identity aggregate-level errors to API error codes.
// Returns nil if the error is not an identity aggregate error.
func MapToAPIError(err error) *apierror.Error {
	switch {
	case errors.Is(err, ErrPasswordTooShort):
		return apierror.IdentityPasswordTooShort
	case errors.Is(err, ErrPasswordTooWeak):
		return apierror.IdentityPasswordTooWeak
	case errors.Is(err, ErrDisplayNameTooLong):
		return apierror.IdentityDisplayNameTooLong
	case errors.Is(err, ErrTelegramAlreadyLinked):
		return apierror.IdentityTelegramAlreadyLinked
	case errors.Is(err, ErrTelegramNotLinked):
		return apierror.IdentityTelegramNotLinked
	default:
		return nil
	}
}
