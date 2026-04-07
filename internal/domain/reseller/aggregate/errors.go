package aggregate

import "errors"

// Aggregate-level sentinel errors for reseller aggregates (Tenant,
// ResellerAccount, Commission). Defined here so aggregate methods can
// reference them without circular imports with the parent reseller package.

var (
	ErrInvalidCommissionRate = errors.New("commission rate must be between 0 and 100")
	ErrCommissionAlreadyPaid = errors.New("commission already paid")
	ErrCommissionOverflow    = errors.New("commission amount overflow")
	ErrTenantAlreadyInactive = errors.New("tenant already inactive")
	ErrTenantAlreadyActive   = errors.New("tenant already active")
)
