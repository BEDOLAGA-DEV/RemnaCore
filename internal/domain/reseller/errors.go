package reseller

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/service"
)

// Service-level error aliases re-exported for backward compatibility.
var (
	ErrNotFound           = service.ErrNotFound
	ErrTenantNotFound     = service.ErrTenantNotFound
	ErrResellerNotFound   = service.ErrResellerNotFound
	ErrCommissionNotFound = service.ErrCommissionNotFound
	ErrInvalidAPIKey      = service.ErrInvalidAPIKey
	ErrTenantInactive     = service.ErrTenantInactive
	ErrDuplicateDomain    = service.ErrDuplicateDomain
)

// Aggregate-level error aliases re-exported for backward compatibility.
var (
	ErrInvalidCommissionRate = aggregate.ErrInvalidCommissionRate
	ErrCommissionAlreadyPaid = aggregate.ErrCommissionAlreadyPaid
	ErrCommissionOverflow    = aggregate.ErrCommissionOverflow
	ErrTenantAlreadyInactive = aggregate.ErrTenantAlreadyInactive
	ErrTenantAlreadyActive   = aggregate.ErrTenantAlreadyActive
)
