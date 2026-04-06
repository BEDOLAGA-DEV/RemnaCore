package reseller

import "context"

// TenantRepository defines the persistence operations for tenants.
type TenantRepository interface {
	CreateTenant(ctx context.Context, tenant *Tenant) error
	GetTenantByID(ctx context.Context, id string) (*Tenant, error)
	// GetTenantByIDForUpdate retrieves a tenant by ID with a SELECT FOR UPDATE
	// row lock. Must be called within a RunInTx transaction to prevent TOCTOU
	// races during read-modify-write cycles.
	GetTenantByIDForUpdate(ctx context.Context, id string) (*Tenant, error)
	GetTenantByDomain(ctx context.Context, domain string) (*Tenant, error)
	GetTenantByAPIKeyHash(ctx context.Context, keyHash string) (*Tenant, error)
	UpdateTenant(ctx context.Context, tenant *Tenant) error
	ListTenants(ctx context.Context, limit, offset int) ([]*Tenant, error)
}

// CommissionRepository defines the persistence operations for reseller accounts
// and commissions.
type CommissionRepository interface {
	CreateResellerAccount(ctx context.Context, account *ResellerAccount) error
	GetResellerAccountByID(ctx context.Context, id string) (*ResellerAccount, error)
	GetResellerAccountByUserAndTenant(ctx context.Context, userID, tenantID string) (*ResellerAccount, error)

	CreateCommission(ctx context.Context, commission *Commission) error
	GetCommissionByID(ctx context.Context, id string) (*Commission, error)
	// GetCommissionByIDForUpdate retrieves a commission by ID with a SELECT FOR
	// UPDATE row lock. Must be called within a RunInTx transaction to prevent
	// TOCTOU races during read-modify-write cycles.
	GetCommissionByIDForUpdate(ctx context.Context, id string) (*Commission, error)
	GetPendingCommissions(ctx context.Context, resellerID string) ([]*Commission, error)
	UpdateCommission(ctx context.Context, commission *Commission) error

	UpdateResellerBalance(ctx context.Context, resellerID string, balance int64) error
}
