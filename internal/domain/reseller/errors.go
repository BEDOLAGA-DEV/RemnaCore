package reseller

import "errors"

var (
	ErrNotFound              = errors.New("not found")
	ErrTenantNotFound        = errors.New("tenant not found")
	ErrResellerNotFound      = errors.New("reseller account not found")
	ErrCommissionNotFound    = errors.New("commission not found")
	ErrInvalidCommissionRate = errors.New("commission rate must be between 0 and 100")
	ErrInvalidAPIKey         = errors.New("invalid API key")
	ErrTenantInactive        = errors.New("tenant is inactive")
	ErrDuplicateDomain       = errors.New("domain already in use")
)
