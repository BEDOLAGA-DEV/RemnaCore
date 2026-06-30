package reseller

import (
	"context"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/vo"
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
	// ListCommissionsByTenant returns a single reseller's commissions within the
	// active shop, ordered newest-first. RLS on reseller.commissions (043) scopes
	// rows to the shop; the resellerID argument additionally scopes them to the
	// resolved account so co-tenant resellers cannot see each other's rows. The
	// active app.tenant_id GUC must be set (call inside RunInTx).
	ListCommissionsByTenant(ctx context.Context, tenantID, resellerID string) ([]*aggregate.Commission, error)
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

// ShopBotRepository defines persistence operations for per-shop Telegram bot
// configuration. Token values are encrypted at rest by the adapter; callers
// always receive and supply plaintext (wrapped in config.SecretString).
type ShopBotRepository interface {
	// Upsert inserts or updates the bot config for tenantID. The adapter seals
	// cfg.Token with AES-256-GCM before writing it to the database.
	Upsert(ctx context.Context, tenantID string, cfg vo.ShopBotConfig) error

	// GetByTenant returns the bot config for tenantID, decrypting the stored
	// token. Returns ErrShopBotNotFound when no row exists or RLS blocks access.
	GetByTenant(ctx context.Context, tenantID string) (*vo.ShopBotConfig, error)

	// ListEnabled returns all enabled bot configs (platform-sentinel scope).
	// Intended for the bot manager that needs to poll all active bots on startup.
	ListEnabled(ctx context.Context) ([]vo.ShopBotWithTenant, error)
}
