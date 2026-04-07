package aggregate

import "errors"

// Aggregate-level sentinel errors for identity aggregates (PlatformUser,
// EmailVerification, PasswordReset). Defined here so aggregate methods can
// reference them without circular imports with the parent identity package.

var (
	ErrPasswordTooShort      = errors.New("password must be at least 8 characters")
	ErrPasswordTooWeak       = errors.New("password must contain uppercase, lowercase, and digit characters")
	ErrDisplayNameTooLong    = errors.New("display name exceeds maximum length")
	ErrTelegramAlreadyLinked = errors.New("telegram already linked")
	ErrTelegramNotLinked     = errors.New("telegram not linked")
)
