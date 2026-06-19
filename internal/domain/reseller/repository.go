package reseller

import (
	"context"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/service"
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
	// SetTenantOwnerUserID persists an owner assignment on a pending-owner tenant.
	// Must be called within a RunInTx transaction.
	SetTenantOwnerUserID(ctx context.Context, tenantID, userID string, now time.Time) error
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
	// ListCommissionsByTenant returns all commissions for the active shop,
	// ordered newest-first. RLS on reseller.commissions (043) scopes the rows;
	// the active app.tenant_id GUC must be set (call inside RunInTx).
	ListCommissionsByTenant(ctx context.Context, tenantID string) ([]*aggregate.Commission, error)
	UpdateCommission(ctx context.Context, commission *aggregate.Commission) error

	UpdateResellerBalance(ctx context.Context, resellerID string, balance int64) error
}

// CustomerSummary is re-exported from the reseller/service subpackage where it
// is canonically defined (the service package cannot import this root without a
// cycle). New code may reference reseller.CustomerSummary.
type CustomerSummary = service.CustomerSummary

// DashboardSummary is re-exported from the reseller/service subpackage.
type DashboardSummary = service.DashboardSummary

// CustomerRepository is re-exported from the reseller/service subpackage.
type CustomerRepository = service.CustomerRepository
