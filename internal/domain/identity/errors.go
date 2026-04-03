package identity

import "errors"

var (
	ErrNotFound              = errors.New("not found")
	ErrEmailTaken            = errors.New("email already taken")
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrEmailNotVerified      = errors.New("email not verified")
	ErrTokenExpired          = errors.New("verification token expired")
	ErrSessionExpired        = errors.New("session expired")
	ErrPasswordResetExpired  = errors.New("password reset token expired")
	ErrPasswordResetNotFound = errors.New("password reset token not found")
	ErrPasswordTooShort      = errors.New("password must be at least 8 characters")
	ErrPasswordTooWeak       = errors.New("password must contain uppercase, lowercase, and digit characters")
)
