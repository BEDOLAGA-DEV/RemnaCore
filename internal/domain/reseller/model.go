package reseller

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/vo"
)

// --- Aggregate type aliases (backward compatibility) ---

type (
	Tenant          = aggregate.Tenant
	ResellerAccount = aggregate.ResellerAccount
	Commission      = aggregate.Commission
)

// --- Value object aliases (backward compatibility) ---

type (
	BrandingConfig   = vo.BrandingConfig
	CommissionStatus = vo.CommissionStatus
)

const (
	CommissionPending = vo.CommissionPending
	CommissionPaid    = vo.CommissionPaid
)

// --- Aggregate constant aliases (backward compatibility) ---

const (
	APIKeyLen         = aggregate.APIKeyLen
	PercentBase       = aggregate.PercentBase
	MinCommissionRate = aggregate.MinCommissionRate
	MaxCommissionRate = aggregate.MaxCommissionRate
)

// --- Constructor aliases (backward compatibility) ---

var (
	NewTenant          = aggregate.NewTenant
	NewResellerAccount = aggregate.NewResellerAccount
	NewCommission      = aggregate.NewCommission
	HashAPIKey         = aggregate.HashAPIKey
)

// --- Service type aliases (backward compatibility) ---

type ResellerService = service.ResellerService

// NewResellerService is a convenience re-export from the service subpackage.
var NewResellerService = service.NewResellerService

// Note: TenantRepository and CommissionRepository are defined canonically in
// repository.go (same package). The service subpackage retains structurally
// identical copies for backward compatibility, but new code should reference
// reseller.TenantRepository and reseller.CommissionRepository directly.
