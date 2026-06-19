package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/money"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
)

// ResellerRepository implements both reseller.TenantRepository and
// reseller.CommissionRepository backed by PostgreSQL via sqlc-generated queries.
// All methods resolve the underlying connection via DBFromContext so that
// queries participate in the active transaction (and therefore respect
// SET LOCAL app.tenant_id for RLS enforcement).
type ResellerRepository struct {
	pool *pgxpool.Pool
}

// NewResellerRepository returns a new ResellerRepository using the given pool.
func NewResellerRepository(pool *pgxpool.Pool) *ResellerRepository {
	return &ResellerRepository{pool: pool}
}

// q returns the sqlc Queries bound to the transaction from ctx (if any),
// otherwise falls back to the pool. This ensures row-level security session
// variables set via SET LOCAL are visible to every query.
func (r *ResellerRepository) q(ctx context.Context) *gen.Queries {
	return gen.New(DBFromContext(ctx, r.pool))
}

// ---------------------------------------------------------------------------
// Row converters
// ---------------------------------------------------------------------------

func tenantRowToDomain(row gen.ResellerTenant) *reseller.Tenant {
	var branding reseller.BrandingConfig
	if len(row.BrandingConfig) > 0 {
		if err := json.Unmarshal(row.BrandingConfig, &branding); err != nil {
			slog.Warn("corrupt branding config in database",
				slog.String("tenant_id", pgutil.PgtypeToUUID(row.ID)),
				slog.String("error", err.Error()),
			)
		}
	}

	return &reseller.Tenant{
		ID:             pgutil.PgtypeToUUID(row.ID),
		Name:           row.Name,
		Domain:         pgutil.DerefStr(row.Domain),
		OwnerUserID:    pgutil.PgtypeUUIDToOptStr(row.OwnerUserID),
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

// commissionFields holds the columns shared by every commission row shape.
type commissionFields struct {
	ID         pgtype.UUID
	ResellerID pgtype.UUID
	SaleID     string
	Amount     int64
	Currency   string
	Status     string
	CreatedAt  pgtype.Timestamptz
	PaidAt     pgtype.Timestamptz
}

// commissionRow constrains the sqlc-generated commission row types and the base
// model. The explicit-column SELECTs each emit a distinct *Row struct (the
// model carries the extra tenant_id column added in migration 043), so the
// converter is generic over all of them plus the model used by the raw FOR
// UPDATE and tenant-scoped scans.
type commissionRow interface {
	gen.ResellerCommission | gen.GetCommissionByIDRow | gen.GetPendingCommissionsRow
}

// extractCommissionFields pulls the shared columns from any commission row type.
func extractCommissionFields[T commissionRow](row T) commissionFields {
	switch r := any(row).(type) {
	case gen.ResellerCommission:
		return commissionFields{r.ID, r.ResellerID, r.SaleID, r.Amount, r.Currency, r.Status, r.CreatedAt, r.PaidAt}
	case gen.GetCommissionByIDRow:
		return commissionFields{r.ID, r.ResellerID, r.SaleID, r.Amount, r.Currency, r.Status, r.CreatedAt, r.PaidAt}
	case gen.GetPendingCommissionsRow:
		return commissionFields{r.ID, r.ResellerID, r.SaleID, r.Amount, r.Currency, r.Status, r.CreatedAt, r.PaidAt}
	default:
		panic("unreachable: unhandled commissionRow type")
	}
}

func commissionRowToDomain[T commissionRow](row T) *reseller.Commission {
	f := extractCommissionFields(row)
	return &reseller.Commission{
		ID:         pgutil.PgtypeToUUID(f.ID),
		ResellerID: pgutil.PgtypeToUUID(f.ResellerID),
		SaleID:     f.SaleID,
		Amount:     money.NewMoney(f.Amount, money.Currency(f.Currency)),
		Status:     reseller.CommissionStatus(f.Status),
		CreatedAt:  pgutil.PgtypeToTime(f.CreatedAt),
		PaidAt:     pgutil.PgtypeToOptTime(f.PaidAt),
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

	err = r.q(ctx).CreateTenant(ctx, gen.CreateTenantParams{
		ID:             pgutil.UUIDToPgtype(tenant.ID),
		Name:           tenant.Name,
		Domain:         pgutil.StrPtrOrNil(tenant.Domain),
		OwnerUserID:    pgutil.OptStrToPgtypeUUID(tenant.OwnerUserID),
		BrandingConfig: brandingJSON,
		ApiKeyHash:     pgutil.StrPtrOrNil(tenant.APIKeyHash),
		IsActive:       tenant.IsActive,
		CreatedAt:      pgutil.TimeToPgtype(tenant.CreatedAt),
		UpdatedAt:      pgutil.TimeToPgtype(tenant.UpdatedAt),
	})
	if err != nil {
		if pgutil.IsUniqueViolation(err) {
			return fmt.Errorf("create tenant: %w", reseller.ErrTenantAlreadyExists)
		}
		return fmt.Errorf("create tenant: %w", err)
	}
	return nil
}

func (r *ResellerRepository) GetTenantByID(ctx context.Context, id string) (*reseller.Tenant, error) {
	row, err := r.q(ctx).GetTenantByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get tenant by id", reseller.ErrTenantNotFound)
	}
	return tenantRowToDomain(row), nil
}

// getTenantByIDForUpdateSQL is identical to GetTenantByID but acquires a
// FOR UPDATE row lock. Must be called within a transaction.
const getTenantByIDForUpdateSQL = `
SELECT id, name, domain, owner_user_id, branding_config, api_key_hash, is_active, created_at, updated_at
FROM reseller.tenants WHERE id = $1 FOR UPDATE
`

func (r *ResellerRepository) GetTenantByIDForUpdate(ctx context.Context, id string) (*reseller.Tenant, error) {
	db := DBFromContext(ctx, r.pool)
	row := db.QueryRow(ctx, getTenantByIDForUpdateSQL, pgutil.UUIDToPgtype(id))

	var rawRow gen.ResellerTenant
	err := row.Scan(
		&rawRow.ID, &rawRow.Name, &rawRow.Domain, &rawRow.OwnerUserID,
		&rawRow.BrandingConfig, &rawRow.ApiKeyHash, &rawRow.IsActive,
		&rawRow.CreatedAt, &rawRow.UpdatedAt,
	)
	if err != nil {
		return nil, pgutil.MapErr(err, "get tenant by id for update", reseller.ErrTenantNotFound)
	}
	return tenantRowToDomain(rawRow), nil
}

func (r *ResellerRepository) GetTenantByDomain(ctx context.Context, domain string) (*reseller.Tenant, error) {
	row, err := r.q(ctx).GetTenantByDomain(ctx, &domain)
	if err != nil {
		return nil, pgutil.MapErr(err, "get tenant by domain", reseller.ErrTenantNotFound)
	}
	return tenantRowToDomain(row), nil
}

func (r *ResellerRepository) GetTenantByAPIKeyHash(ctx context.Context, keyHash string) (*reseller.Tenant, error) {
	row, err := r.q(ctx).GetTenantByAPIKeyHash(ctx, &keyHash)
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

	err = r.q(ctx).UpdateTenant(ctx, gen.UpdateTenantParams{
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
	rows, err := r.q(ctx).ListTenants(ctx, gen.ListTenantsParams{
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
	err := r.q(ctx).CreateResellerAccount(ctx, gen.CreateResellerAccountParams{
		ID:             pgutil.UUIDToPgtype(account.ID),
		TenantID:       pgutil.UUIDToPgtype(account.TenantID),
		UserID:         pgutil.UUIDToPgtype(account.UserID),
		CommissionRate: int32(account.CommissionRate),
		Balance:        account.Balance,
		CreatedAt:      pgutil.TimeToPgtype(account.CreatedAt),
	})
	if err != nil {
		if pgutil.IsUniqueViolation(err) {
			return fmt.Errorf("create reseller account: %w", reseller.ErrResellerAlreadyExists)
		}
		return fmt.Errorf("create reseller account: %w", err)
	}
	return nil
}

func (r *ResellerRepository) GetResellerAccountByID(ctx context.Context, id string) (*reseller.ResellerAccount, error) {
	row, err := r.q(ctx).GetResellerAccountByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get reseller account by id", reseller.ErrResellerNotFound)
	}
	return resellerAccountRowToDomain(row), nil
}

func (r *ResellerRepository) GetResellerAccountByUserAndTenant(ctx context.Context, userID, tenantID string) (*reseller.ResellerAccount, error) {
	row, err := r.q(ctx).GetResellerAccountByUserAndTenant(ctx, gen.GetResellerAccountByUserAndTenantParams{
		UserID:   pgutil.UUIDToPgtype(userID),
		TenantID: pgutil.UUIDToPgtype(tenantID),
	})
	if err != nil {
		return nil, pgutil.MapErr(err, "get reseller account by user and tenant", reseller.ErrResellerNotFound)
	}
	return resellerAccountRowToDomain(row), nil
}

func (r *ResellerRepository) GetCommissionByID(ctx context.Context, id string) (*reseller.Commission, error) {
	row, err := r.q(ctx).GetCommissionByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get commission by id", reseller.ErrCommissionNotFound)
	}
	return commissionRowToDomain(row), nil
}

// getCommissionByIDForUpdateSQL is identical to GetCommissionByID but acquires
// a FOR UPDATE row lock. Must be called within a transaction.
const getCommissionByIDForUpdateSQL = `
SELECT id, reseller_id, sale_id, amount, currency, status, created_at, paid_at
FROM reseller.commissions WHERE id = $1 FOR UPDATE
`

func (r *ResellerRepository) GetCommissionByIDForUpdate(ctx context.Context, id string) (*reseller.Commission, error) {
	db := DBFromContext(ctx, r.pool)
	row := db.QueryRow(ctx, getCommissionByIDForUpdateSQL, pgutil.UUIDToPgtype(id))

	var rawRow gen.ResellerCommission
	err := row.Scan(
		&rawRow.ID, &rawRow.ResellerID, &rawRow.SaleID, &rawRow.Amount,
		&rawRow.Currency, &rawRow.Status, &rawRow.CreatedAt, &rawRow.PaidAt,
	)
	if err != nil {
		return nil, pgutil.MapErr(err, "get commission by id for update", reseller.ErrCommissionNotFound)
	}
	return commissionRowToDomain(rawRow), nil
}

func (r *ResellerRepository) CreateCommission(ctx context.Context, commission *reseller.Commission) error {
	err := r.q(ctx).CreateCommission(ctx, gen.CreateCommissionParams{
		ID:         pgutil.UUIDToPgtype(commission.ID),
		ResellerID: pgutil.UUIDToPgtype(commission.ResellerID),
		SaleID:     commission.SaleID,
		Amount:     commission.Amount.Amount,
		Currency:   string(commission.Amount.Currency),
		Status:     string(commission.Status),
		CreatedAt:  pgutil.TimeToPgtype(commission.CreatedAt),
		PaidAt:     pgutil.OptTimeToPgtype(commission.PaidAt),
	})
	if err != nil {
		if pgutil.IsUniqueViolation(err) {
			return fmt.Errorf("create commission: %w", reseller.ErrCommissionAlreadyExists)
		}
		return fmt.Errorf("create commission: %w", err)
	}
	return nil
}

func (r *ResellerRepository) GetPendingCommissions(ctx context.Context, resellerID string) ([]*reseller.Commission, error) {
	rows, err := r.q(ctx).GetPendingCommissions(ctx, pgutil.UUIDToPgtype(resellerID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get pending commissions", reseller.ErrCommissionNotFound)
	}
	commissions := make([]*reseller.Commission, 0, len(rows))
	for _, row := range rows {
		commissions = append(commissions, commissionRowToDomain(row))
	}
	return commissions, nil
}

// listCommissionsByTenantSQL lists a single reseller's commissions within a
// shop, newest-first. RLS on reseller.commissions (043) scopes rows to the
// shop (tenant_id GUC); the reseller_id predicate additionally prevents a
// horizontal leak between two reseller accounts in the same shop. The explicit
// tenant_id predicate is belt-and-suspenders. Must be called inside RunInTx so
// the GUC is set.
const listCommissionsByTenantSQL = `
SELECT id, reseller_id, sale_id, amount, currency, status, created_at, paid_at
FROM reseller.commissions
WHERE tenant_id = $1 AND reseller_id = $2
ORDER BY created_at DESC
`

func (r *ResellerRepository) ListCommissionsByTenant(ctx context.Context, tenantID, resellerID string) ([]*reseller.Commission, error) {
	db := DBFromContext(ctx, r.pool)
	rows, err := db.Query(ctx, listCommissionsByTenantSQL, pgutil.UUIDToPgtype(tenantID), pgutil.UUIDToPgtype(resellerID))
	if err != nil {
		return nil, fmt.Errorf("list commissions by tenant: %w", err)
	}
	defer rows.Close()

	commissions := make([]*reseller.Commission, 0)
	for rows.Next() {
		var row gen.ResellerCommission
		if err := rows.Scan(
			&row.ID, &row.ResellerID, &row.SaleID, &row.Amount,
			&row.Currency, &row.Status, &row.CreatedAt, &row.PaidAt,
		); err != nil {
			return nil, fmt.Errorf("scan commission row: %w", err)
		}
		commissions = append(commissions, commissionRowToDomain(row))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate commission rows: %w", err)
	}
	return commissions, nil
}

func (r *ResellerRepository) UpdateCommission(ctx context.Context, commission *reseller.Commission) error {
	err := r.q(ctx).UpdateCommission(ctx, gen.UpdateCommissionParams{
		ID:     pgutil.UUIDToPgtype(commission.ID),
		Status: string(commission.Status),
		PaidAt: pgutil.OptTimeToPgtype(commission.PaidAt),
	})
	return pgutil.MapErr(err, "update commission", reseller.ErrCommissionNotFound)
}

func (r *ResellerRepository) UpdateResellerBalance(ctx context.Context, resellerID string, balance int64) error {
	err := r.q(ctx).UpdateResellerBalance(ctx, gen.UpdateResellerBalanceParams{
		ID:      pgutil.UUIDToPgtype(resellerID),
		Balance: balance,
	})
	return pgutil.MapErr(err, "update reseller balance", reseller.ErrResellerNotFound)
}

// setTenantOwnerSQL updates only the owner_user_id and updated_at columns.
// Used by SetTenantOwner to atomically assign an owner to a pending-owner tenant.
const setTenantOwnerSQL = `
UPDATE reseller.tenants
SET owner_user_id = $2, updated_at = $3
WHERE id = $1
`

// SetTenantOwnerUserID persists an owner assignment on a tenant row.
// Called from ResellerService.SetTenantOwner inside a transaction.
func (r *ResellerRepository) SetTenantOwnerUserID(ctx context.Context, tenantID, userID string, now time.Time) error {
	db := DBFromContext(ctx, r.pool)
	_, err := db.Exec(ctx, setTenantOwnerSQL,
		pgutil.UUIDToPgtype(tenantID),
		pgutil.UUIDToPgtype(userID),
		pgutil.TimeToPgtype(now),
	)
	return pgutil.MapErr(err, "set tenant owner", reseller.ErrTenantNotFound)
}

// listCustomersByTenantSQL lists a shop's customers (identity.platform_users
// with tenant_id = the active shop) plus per-customer active-subscription count.
// RLS on platform_users (040) scopes the rows; the explicit tenant_id predicate
// is belt-and-suspenders (spec §7). Must be called inside RunInTx.
const listCustomersByTenantSQL = `
SELECT
    u.id,
    u.email,
    u.display_name,
    u.created_at,
    COALESCE(s.active_subs, 0) AS active_subs
FROM identity.platform_users u
LEFT JOIN (
    SELECT user_id, count(*) AS active_subs
    FROM billing.subscriptions
    WHERE status IN ('trial', 'active')
    GROUP BY user_id
) s ON s.user_id = u.id
WHERE u.tenant_id = $1
ORDER BY u.created_at DESC
LIMIT $2 OFFSET $3
`

// ListCustomersByTenant MUST be called inside RunInTx with an active
// app.tenant_id GUC (a shop UUID or the platform sentinel). Scoping is RLS-only
// plus the explicit tenant_id predicate; if the GUC is unset/empty this returns
// ZERO rows with NO error (fail-closed, silent) rather than every tenant's rows.
func (r *ResellerRepository) ListCustomersByTenant(ctx context.Context, tenantID string, limit, offset int) ([]*reseller.CustomerSummary, error) {
	db := DBFromContext(ctx, r.pool)
	rows, err := db.Query(ctx, listCustomersByTenantSQL, pgutil.UUIDToPgtype(tenantID), int32(limit), int32(offset))
	if err != nil {
		return nil, fmt.Errorf("list customers by tenant: %w", err)
	}
	defer rows.Close()

	customers := make([]*reseller.CustomerSummary, 0)
	for rows.Next() {
		var (
			email, displayName pgtype.Text
			userID             pgtype.UUID
			createdAt          pgtype.Timestamptz
			activeSubs         int64
		)
		if err := rows.Scan(&userID, &email, &displayName, &createdAt, &activeSubs); err != nil {
			return nil, fmt.Errorf("scan customer row: %w", err)
		}
		customers = append(customers, &reseller.CustomerSummary{
			UserID:          pgutil.PgtypeToUUID(userID),
			Email:           email.String,
			DisplayName:     displayName.String,
			ActiveSubsCount: int(activeSubs),
			CreatedAt:       pgutil.PgtypeToTime(createdAt),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate customer rows: %w", err)
	}
	return customers, nil
}

const countActiveCustomersByTenantSQL = `
SELECT count(*) FROM identity.platform_users WHERE tenant_id = $1
`

func (r *ResellerRepository) CountActiveCustomersByTenant(ctx context.Context, tenantID string) (int, error) {
	db := DBFromContext(ctx, r.pool)
	var n int64
	if err := db.QueryRow(ctx, countActiveCustomersByTenantSQL, pgutil.UUIDToPgtype(tenantID)).Scan(&n); err != nil {
		return 0, fmt.Errorf("count active customers by tenant: %w", err)
	}
	return int(n), nil
}

const countActiveSubscriptionsByTenantSQL = `
SELECT count(*) FROM billing.subscriptions
WHERE tenant_id = $1 AND status IN ('trial', 'active')
`

func (r *ResellerRepository) CountActiveSubscriptionsByTenant(ctx context.Context, tenantID string) (int, error) {
	db := DBFromContext(ctx, r.pool)
	var n int64
	if err := db.QueryRow(ctx, countActiveSubscriptionsByTenantSQL, pgutil.UUIDToPgtype(tenantID)).Scan(&n); err != nil {
		return 0, fmt.Errorf("count active subscriptions by tenant: %w", err)
	}
	return int(n), nil
}

const sumPendingCommissionByTenantSQL = `
SELECT COALESCE(sum(amount), 0) AS amount,
       COALESCE(max(currency), '') AS currency
FROM reseller.commissions
WHERE tenant_id = $1 AND status = 'pending'
`

func (r *ResellerRepository) SumPendingCommissionByTenant(ctx context.Context, tenantID string) (int64, string, error) {
	db := DBFromContext(ctx, r.pool)
	var (
		amount   int64
		currency string
	)
	if err := db.QueryRow(ctx, sumPendingCommissionByTenantSQL, pgutil.UUIDToPgtype(tenantID)).Scan(&amount, &currency); err != nil {
		return 0, "", fmt.Errorf("sum pending commission by tenant: %w", err)
	}
	return amount, currency, nil
}

// compile-time interface checks
var (
	_ reseller.TenantRepository     = (*ResellerRepository)(nil)
	_ reseller.CommissionRepository = (*ResellerRepository)(nil)
	_ reseller.CustomerRepository   = (*ResellerRepository)(nil)
)
