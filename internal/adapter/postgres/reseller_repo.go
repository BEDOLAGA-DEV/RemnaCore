package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
)

// ResellerRepository implements both reseller.TenantRepository and
// reseller.CommissionRepository backed by PostgreSQL via sqlc-generated queries.
type ResellerRepository struct {
	pool    *pgxpool.Pool
	queries *gen.Queries
}

// NewResellerRepository returns a new ResellerRepository using the given pool.
func NewResellerRepository(pool *pgxpool.Pool) *ResellerRepository {
	return &ResellerRepository{
		pool:    pool,
		queries: gen.New(pool),
	}
}

// ---------------------------------------------------------------------------
// Row converters
// ---------------------------------------------------------------------------

func tenantRowToDomain(row gen.ResellerTenant) *reseller.Tenant {
	var branding reseller.BrandingConfig
	if len(row.BrandingConfig) > 0 {
		_ = json.Unmarshal(row.BrandingConfig, &branding)
	}

	return &reseller.Tenant{
		ID:             pgutil.PgtypeToUUID(row.ID),
		Name:           row.Name,
		Domain:         pgutil.DerefStr(row.Domain),
		OwnerUserID:    pgutil.PgtypeToUUID(row.OwnerUserID),
		BrandingConfig: branding,
		APIKeyHash:     pgutil.DerefStr(row.ApiKeyHash),
		IsActive:       row.IsActive,
		CreatedAt:      pgutil.PgtypeToTime(row.CreatedAt),
		UpdatedAt:      pgutil.PgtypeToTime(row.UpdatedAt),
	}
}

func resellerAccountRowToDomain(row gen.ResellerResellerAccount) *reseller.ResellerAccount {
	return &reseller.ResellerAccount{
		ID:             pgutil.PgtypeToUUID(row.ID),
		TenantID:       pgutil.PgtypeToUUID(row.TenantID),
		UserID:         pgutil.PgtypeToUUID(row.UserID),
		CommissionRate: int(row.CommissionRate),
		Balance:        row.Balance,
		CreatedAt:      pgutil.PgtypeToTime(row.CreatedAt),
	}
}

func commissionRowToDomain(row gen.ResellerCommission) *reseller.Commission {
	return &reseller.Commission{
		ID:         pgutil.PgtypeToUUID(row.ID),
		ResellerID: pgutil.PgtypeToUUID(row.ResellerID),
		SaleID:     row.SaleID,
		Amount:     row.Amount,
		Currency:   row.Currency,
		Status:     reseller.CommissionStatus(row.Status),
		CreatedAt:  pgutil.PgtypeToTime(row.CreatedAt),
		PaidAt:     pgutil.PgtypeToOptTime(row.PaidAt),
	}
}

// ---------------------------------------------------------------------------
// TenantRepository interface implementation
// ---------------------------------------------------------------------------

func (r *ResellerRepository) CreateTenant(ctx context.Context, tenant *reseller.Tenant) error {
	brandingJSON, err := json.Marshal(tenant.BrandingConfig)
	if err != nil {
		return fmt.Errorf("marshal branding config: %w", err)
	}

	err = r.queries.CreateTenant(ctx, gen.CreateTenantParams{
		ID:             pgutil.UUIDToPgtype(tenant.ID),
		Name:           tenant.Name,
		Domain:         pgutil.StrPtrOrNil(tenant.Domain),
		OwnerUserID:    pgutil.UUIDToPgtype(tenant.OwnerUserID),
		BrandingConfig: brandingJSON,
		ApiKeyHash:     pgutil.StrPtrOrNil(tenant.APIKeyHash),
		IsActive:       tenant.IsActive,
		CreatedAt:      pgutil.TimeToPgtype(tenant.CreatedAt),
		UpdatedAt:      pgutil.TimeToPgtype(tenant.UpdatedAt),
	})
	return pgutil.MapErr(err, "create tenant", reseller.ErrTenantNotFound)
}

func (r *ResellerRepository) GetTenantByID(ctx context.Context, id string) (*reseller.Tenant, error) {
	row, err := r.queries.GetTenantByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get tenant by id", reseller.ErrTenantNotFound)
	}
	return tenantRowToDomain(row), nil
}

func (r *ResellerRepository) GetTenantByDomain(ctx context.Context, domain string) (*reseller.Tenant, error) {
	row, err := r.queries.GetTenantByDomain(ctx, &domain)
	if err != nil {
		return nil, pgutil.MapErr(err, "get tenant by domain", reseller.ErrTenantNotFound)
	}
	return tenantRowToDomain(row), nil
}

func (r *ResellerRepository) GetTenantByAPIKeyHash(ctx context.Context, keyHash string) (*reseller.Tenant, error) {
	row, err := r.queries.GetTenantByAPIKeyHash(ctx, &keyHash)
	if err != nil {
		return nil, pgutil.MapErr(err, "get tenant by api key hash", reseller.ErrTenantNotFound)
	}
	return tenantRowToDomain(row), nil
}

func (r *ResellerRepository) UpdateTenant(ctx context.Context, tenant *reseller.Tenant) error {
	brandingJSON, err := json.Marshal(tenant.BrandingConfig)
	if err != nil {
		return fmt.Errorf("marshal branding config for update: %w", err)
	}

	err = r.queries.UpdateTenant(ctx, gen.UpdateTenantParams{
		ID:             pgutil.UUIDToPgtype(tenant.ID),
		Name:           tenant.Name,
		Domain:         pgutil.StrPtrOrNil(tenant.Domain),
		BrandingConfig: brandingJSON,
		ApiKeyHash:     pgutil.StrPtrOrNil(tenant.APIKeyHash),
		IsActive:       tenant.IsActive,
	})
	return pgutil.MapErr(err, "update tenant", reseller.ErrTenantNotFound)
}

func (r *ResellerRepository) ListTenants(ctx context.Context, limit, offset int) ([]*reseller.Tenant, error) {
	rows, err := r.queries.ListTenants(ctx, gen.ListTenantsParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, pgutil.MapErr(err, "list tenants", reseller.ErrTenantNotFound)
	}
	tenants := make([]*reseller.Tenant, 0, len(rows))
	for _, row := range rows {
		tenants = append(tenants, tenantRowToDomain(row))
	}
	return tenants, nil
}

// ---------------------------------------------------------------------------
// CommissionRepository interface implementation
// ---------------------------------------------------------------------------

func (r *ResellerRepository) CreateResellerAccount(ctx context.Context, account *reseller.ResellerAccount) error {
	err := r.queries.CreateResellerAccount(ctx, gen.CreateResellerAccountParams{
		ID:             pgutil.UUIDToPgtype(account.ID),
		TenantID:       pgutil.UUIDToPgtype(account.TenantID),
		UserID:         pgutil.UUIDToPgtype(account.UserID),
		CommissionRate: int32(account.CommissionRate),
		Balance:        account.Balance,
		CreatedAt:      pgutil.TimeToPgtype(account.CreatedAt),
	})
	return pgutil.MapErr(err, "create reseller account", reseller.ErrResellerNotFound)
}

func (r *ResellerRepository) GetResellerAccountByID(ctx context.Context, id string) (*reseller.ResellerAccount, error) {
	row, err := r.queries.GetResellerAccountByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get reseller account by id", reseller.ErrResellerNotFound)
	}
	return resellerAccountRowToDomain(row), nil
}

func (r *ResellerRepository) GetResellerAccountByUserAndTenant(ctx context.Context, userID, tenantID string) (*reseller.ResellerAccount, error) {
	row, err := r.queries.GetResellerAccountByUserAndTenant(ctx, gen.GetResellerAccountByUserAndTenantParams{
		UserID:   pgutil.UUIDToPgtype(userID),
		TenantID: pgutil.UUIDToPgtype(tenantID),
	})
	if err != nil {
		return nil, pgutil.MapErr(err, "get reseller account by user and tenant", reseller.ErrResellerNotFound)
	}
	return resellerAccountRowToDomain(row), nil
}

func (r *ResellerRepository) CreateCommission(ctx context.Context, commission *reseller.Commission) error {
	err := r.queries.CreateCommission(ctx, gen.CreateCommissionParams{
		ID:         pgutil.UUIDToPgtype(commission.ID),
		ResellerID: pgutil.UUIDToPgtype(commission.ResellerID),
		SaleID:     commission.SaleID,
		Amount:     commission.Amount,
		Currency:   commission.Currency,
		Status:     string(commission.Status),
		CreatedAt:  pgutil.TimeToPgtype(commission.CreatedAt),
		PaidAt:     pgutil.OptTimeToPgtype(commission.PaidAt),
	})
	return pgutil.MapErr(err, "create commission", reseller.ErrCommissionNotFound)
}

func (r *ResellerRepository) GetPendingCommissions(ctx context.Context, resellerID string) ([]*reseller.Commission, error) {
	rows, err := r.queries.GetPendingCommissions(ctx, pgutil.UUIDToPgtype(resellerID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get pending commissions", reseller.ErrCommissionNotFound)
	}
	commissions := make([]*reseller.Commission, 0, len(rows))
	for _, row := range rows {
		commissions = append(commissions, commissionRowToDomain(row))
	}
	return commissions, nil
}

func (r *ResellerRepository) UpdateCommission(ctx context.Context, commission *reseller.Commission) error {
	err := r.queries.UpdateCommission(ctx, gen.UpdateCommissionParams{
		ID:     pgutil.UUIDToPgtype(commission.ID),
		Status: string(commission.Status),
		PaidAt: pgutil.OptTimeToPgtype(commission.PaidAt),
	})
	return pgutil.MapErr(err, "update commission", reseller.ErrCommissionNotFound)
}

func (r *ResellerRepository) UpdateResellerBalance(ctx context.Context, resellerID string, balance int64) error {
	err := r.queries.UpdateResellerBalance(ctx, gen.UpdateResellerBalanceParams{
		ID:      pgutil.UUIDToPgtype(resellerID),
		Balance: balance,
	})
	return pgutil.MapErr(err, "update reseller balance", reseller.ErrResellerNotFound)
}

// compile-time interface checks
var (
	_ reseller.TenantRepository     = (*ResellerRepository)(nil)
	_ reseller.CommissionRepository = (*ResellerRepository)(nil)
)
