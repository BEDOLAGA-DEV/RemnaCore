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

	// ErrShopBotInvalidToken is returned when the provided bot token does not
	// match the Telegram format (<id>:<suffix> with suffix ≥35 chars from [A-Za-z0-9_-]).
	ErrShopBotInvalidToken = errors.New("invalid bot token format: must match <id>:<suffix> with suffix ≥35 chars from [A-Za-z0-9_-]")

	// ErrShopBotInvalidCabinetURL is returned when the cabinet URL does not use
	// the HTTPS scheme.
	ErrShopBotInvalidCabinetURL = errors.New("cabinet URL must use HTTPS scheme")

	// ErrShopBotInvalidPlugin is returned when the provided BotPlugin slug does
	// not name an installed, enabled, bot-capable plugin.
	ErrShopBotInvalidPlugin = errors.New("shop bot plugin is not a valid enabled bot plugin")

	// ErrShopBotTokenInUse is returned when another shop already uses the same
	// Telegram bot token. Two shops sharing one token is a cross-tenant hijack
	// (Telegram delivers a token's updates to whichever webhook registered last).
	ErrShopBotTokenInUse = errors.New("this Telegram bot token is already used by another shop")
)
