package service

import "errors"

// Service-level sentinel errors for reseller operations.
var (
	ErrNotFound              = errors.New("not found")
	ErrTenantNotFound        = errors.New("tenant not found")
	ErrTenantAlreadyExists   = errors.New("tenant already exists")
	ErrResellerNotFound      = errors.New("reseller account not found")
	ErrResellerAlreadyExists = errors.New("reseller account already exists")
	ErrCommissionNotFound    = errors.New("commission not found")
	ErrCommissionAlreadyExists = errors.New("commission already exists")
	ErrInvalidAPIKey         = errors.New("invalid API key")
	ErrTenantInactive        = errors.New("tenant is inactive")
	ErrDuplicateDomain       = errors.New("domain already in use")

	// ErrShopBotNotFound is returned when no shop_bots row exists for the
	// requested tenant, or when RLS prevents the caller from seeing the row.
	ErrShopBotNotFound = errors.New("shop bot config not found")

	// ErrEncryptionNotConfigured is returned by the ShopBotRepository when the
	// *secretbox.Box is nil (SECURITY_ENCRYPTION_KEY is unset). The application
	// still boots in this state but cannot seal or open bot tokens.
	ErrEncryptionNotConfigured = errors.New("encryption not configured: SECURITY_ENCRYPTION_KEY is required for bot token storage")
)
