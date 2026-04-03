package reseller

import "context"

// TenantRepository defines the persistence operations for tenants.
type TenantRepository interface {
	CreateTenant(ctx context.Context, tenant *Tenant) error
	GetTenantByID(ctx context.Context, id string) (*Tenant, error)
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
	GetPendingCommissions(ctx context.Context, resellerID string) ([]*Commission, error)
	UpdateCommission(ctx context.Context, commission *Commission) error

	UpdateResellerBalance(ctx context.Context, resellerID string, balance int64) error
}
