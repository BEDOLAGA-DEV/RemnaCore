package service

import "errors"

// Service-level sentinel errors for identity operations.
var (
	ErrNotFound                = errors.New("not found")
	ErrAlreadyExists           = errors.New("identity record already exists")
	ErrEmailTaken              = errors.New("email already taken")
	ErrInvalidCredentials      = errors.New("invalid credentials")
	ErrEmailNotVerified        = errors.New("email not verified")
	ErrTokenExpired            = errors.New("verification token expired")
	ErrSessionExpired          = errors.New("session expired")
	ErrPasswordResetExpired    = errors.New("password reset token expired")
	ErrPasswordResetNotFound   = errors.New("password reset token not found")
	ErrSetupAlreadyCompleted   = errors.New("admin setup already completed")
	ErrTelegramAuthUnavailable = errors.New("telegram auth not available: shop has no bot configured")
)
