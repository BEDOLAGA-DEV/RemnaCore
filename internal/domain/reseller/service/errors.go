package service

import "errors"

// Service-level sentinel errors for reseller operations.
var (
	ErrNotFound         = errors.New("not found")
	ErrTenantNotFound   = errors.New("tenant not found")
	ErrResellerNotFound = errors.New("reseller account not found")
	ErrCommissionNotFound = errors.New("commission not found")
	ErrInvalidAPIKey    = errors.New("invalid API key")
	ErrTenantInactive   = errors.New("tenant is inactive")
	ErrDuplicateDomain  = errors.New("domain already in use")
)
