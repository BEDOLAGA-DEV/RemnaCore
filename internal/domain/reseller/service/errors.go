package service

import "errors"

// Service-level sentinel errors for reseller operations.
var (
	ErrNotFound             = errors.New("not found")
	ErrTenantNotFound       = errors.New("tenant not found")
	ErrTenantAlreadyExists  = errors.New("tenant already exists")
	ErrResellerNotFound     = errors.New("reseller account not found")
	ErrResellerAlreadyExists = errors.New("reseller account already exists")
	ErrCommissionNotFound   = errors.New("commission not found")
	ErrCommissionAlreadyExists = errors.New("commission already exists")
	ErrInvalidAPIKey        = errors.New("invalid API key")
	ErrTenantInactive       = errors.New("tenant is inactive")
	ErrDuplicateDomain      = errors.New("domain already in use")
)
