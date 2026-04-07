package identity

import (
	"errors"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// MapToAPIError maps identity domain errors to API error codes.
// Returns nil if the error is not an identity domain error.
func MapToAPIError(err error) *apierror.Error {
	switch {
	case errors.Is(err, ErrEmailTaken):
		return apierror.IdentityEmailTaken
	case errors.Is(err, ErrInvalidCredentials):
		return apierror.IdentityInvalidCreds
	case errors.Is(err, ErrTokenExpired):
		return apierror.IdentityTokenExpired
	case errors.Is(err, ErrSessionExpired):
		return apierror.IdentitySessionExpired
	case errors.Is(err, ErrNotFound):
		return apierror.IdentityNotFound
	case errors.Is(err, ErrEmailNotVerified):
		return apierror.IdentityEmailNotVerified
	case errors.Is(err, ErrPasswordTooShort):
		return apierror.IdentityPasswordTooShort
	case errors.Is(err, ErrPasswordTooWeak):
		return apierror.IdentityPasswordTooWeak
	case errors.Is(err, ErrPasswordResetExpired):
		return apierror.IdentityResetExpired
	case errors.Is(err, ErrPasswordResetNotFound):
		return apierror.IdentityResetNotFound
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
