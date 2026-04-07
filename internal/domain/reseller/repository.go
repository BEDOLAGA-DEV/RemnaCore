package reseller

import (
	"context"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/aggregate"
)

// TenantRepository defines the persistence operations for tenants.
type TenantRepository interface {
	CreateTenant(ctx context.Context, tenant *aggregate.Tenant) error
	GetTenantByID(ctx context.Context, id string) (*aggregate.Tenant, error)
	// GetTenantByIDForUpdate retrieves a tenant by ID with a SELECT FOR UPDATE
	// row lock. Must be called within a RunInTx transaction to prevent TOCTOU
	// races during read-modify-write cycles.
	GetTenantByIDForUpdate(ctx context.Context, id string) (*aggregate.Tenant, error)
	GetTenantByDomain(ctx context.Context, domain string) (*aggregate.Tenant, error)
	GetTenantByAPIKeyHash(ctx context.Context, keyHash string) (*aggregate.Tenant, error)
	UpdateTenant(ctx context.Context, tenant *aggregate.Tenant) error
	ListTenants(ctx context.Context, limit, offset int) ([]*aggregate.Tenant, error)
}

// CommissionRepository defines the persistence operations for reseller accounts
// and commissions.
type CommissionRepository interface {
	CreateResellerAccount(ctx context.Context, account *aggregate.ResellerAccount) error
	GetResellerAccountByID(ctx context.Context, id string) (*aggregate.ResellerAccount, error)
	GetResellerAccountByUserAndTenant(ctx context.Context, userID, tenantID string) (*aggregate.ResellerAccount, error)

	CreateCommission(ctx context.Context, commission *aggregate.Commission) error
	GetCommissionByID(ctx context.Context, id string) (*aggregate.Commission, error)
	// GetCommissionByIDForUpdate retrieves a commission by ID with a SELECT FOR
	// UPDATE row lock. Must be called within a RunInTx transaction to prevent
	// TOCTOU races during read-modify-write cycles.
	GetCommissionByIDForUpdate(ctx context.Context, id string) (*aggregate.Commission, error)
	GetPendingCommissions(ctx context.Context, resellerID string) ([]*aggregate.Commission, error)
	UpdateCommission(ctx context.Context, commission *aggregate.Commission) error

	UpdateResellerBalance(ctx context.Context, resellerID string, balance int64) error
}
