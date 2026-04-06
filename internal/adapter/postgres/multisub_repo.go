package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
)

// BindingRepository implements multisub.BindingRepository backed by PostgreSQL.
type BindingRepository struct {
	pool *pgxpool.Pool
}

// NewBindingRepository returns a new BindingRepository using the given pool.
func NewBindingRepository(pool *pgxpool.Pool) *BindingRepository {
	return &BindingRepository{pool: pool}
}

// q returns a *gen.Queries backed by the active transaction (if any) or the
// pool. This ensures all methods transparently participate in RunInTx and
// respect RLS tenant scoping.
func (r *BindingRepository) q(ctx context.Context) *gen.Queries {
	return gen.New(DBFromContext(ctx, r.pool))
}

func bindingRowToDomain(row gen.MultisubRemnawaveBinding) *aggregate.RemnawaveBinding {
	return &aggregate.RemnawaveBinding{
		ID:                 pgutil.PgtypeToUUID(row.ID),
		SubscriptionID:     pgutil.PgtypeToUUID(row.SubscriptionID),
		PlatformUserID:     pgutil.PgtypeToUUID(row.PlatformUserID),
		RemnawaveUUID:      pgutil.DerefStr(row.RemnawaveUuid),
		RemnawaveShortUUID: pgutil.DerefStr(row.RemnawaveShortUuid),
		RemnawaveUsername:  row.RemnawaveUsername,
		Purpose:            aggregate.BindingPurpose(row.Purpose),
		Status:             aggregate.BindingStatus(row.Status),
		TrafficLimitBytes:  row.TrafficLimitBytes,
		AllowedNodes:       row.AllowedNodes,
		InboundTags:        row.InboundTags,
		FailReason:         row.FailReason,
		SyncedAt:           pgutil.PgtypeToOptTime(row.SyncedAt),
		CreatedAt:          pgutil.PgtypeToTime(row.CreatedAt),
		UpdatedAt:          pgutil.PgtypeToTime(row.UpdatedAt),
	}
}

func (r *BindingRepository) GetByID(ctx context.Context, id string) (*aggregate.RemnawaveBinding, error) {
	row, err := r.q(ctx).GetBindingByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get binding by id", multisub.ErrBindingNotFound)
	}
	return bindingRowToDomain(row), nil
}

func (r *BindingRepository) GetBySubscriptionID(ctx context.Context, subID string) ([]*aggregate.RemnawaveBinding, error) {
	rows, err := r.q(ctx).GetBindingsBySubscriptionID(ctx, pgutil.UUIDToPgtype(subID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get bindings by subscription id", multisub.ErrBindingNotFound)
	}
	bindings := make([]*aggregate.RemnawaveBinding, len(rows))
	for i, row := range rows {
		bindings[i] = bindingRowToDomain(row)
	}
	return bindings, nil
}

func (r *BindingRepository) GetByPlatformUserID(ctx context.Context, userID string) ([]*aggregate.RemnawaveBinding, error) {
	rows, err := r.q(ctx).GetBindingsByPlatformUserID(ctx, pgutil.UUIDToPgtype(userID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get bindings by platform user id", multisub.ErrBindingNotFound)
	}
	bindings := make([]*aggregate.RemnawaveBinding, len(rows))
	for i, row := range rows {
		bindings[i] = bindingRowToDomain(row)
	}
	return bindings, nil
}

func (r *BindingRepository) GetByRemnawaveUUID(ctx context.Context, rwUUID string) (*aggregate.RemnawaveBinding, error) {
	row, err := r.q(ctx).GetBindingByRemnawaveUUID(ctx, &rwUUID)
	if err != nil {
		return nil, pgutil.MapErr(err, "get binding by remnawave uuid", multisub.ErrBindingNotFound)
	}
	return bindingRowToDomain(row), nil
}

func (r *BindingRepository) GetActiveBySubscriptionID(ctx context.Context, subID string) ([]*aggregate.RemnawaveBinding, error) {
	rows, err := r.q(ctx).GetActiveBindingsBySubscriptionID(ctx, pgutil.UUIDToPgtype(subID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get active bindings by subscription id", multisub.ErrBindingNotFound)
	}
	bindings := make([]*aggregate.RemnawaveBinding, len(rows))
	for i, row := range rows {
		bindings[i] = bindingRowToDomain(row)
	}
	return bindings, nil
}

func (r *BindingRepository) GetAllActive(ctx context.Context) ([]*aggregate.RemnawaveBinding, error) {
	rows, err := r.q(ctx).GetAllActiveBindings(ctx)
	if err != nil {
		return nil, pgutil.MapErr(err, "get all active bindings", multisub.ErrBindingNotFound)
	}
	bindings := make([]*aggregate.RemnawaveBinding, len(rows))
	for i, row := range rows {
		bindings[i] = bindingRowToDomain(row)
	}
	return bindings, nil
}

func (r *BindingRepository) GetFailedWithRemnawaveUUID(ctx context.Context) ([]*aggregate.RemnawaveBinding, error) {
	rows, err := r.q(ctx).GetFailedBindingsWithRemnawaveUUID(ctx)
	if err != nil {
		return nil, pgutil.MapErr(err, "get failed bindings with remnawave uuid", multisub.ErrBindingNotFound)
	}
	bindings := make([]*aggregate.RemnawaveBinding, len(rows))
	for i, row := range rows {
		bindings[i] = bindingRowToDomain(row)
	}
	return bindings, nil
}

// getFailedForReconciliationQuery selects failed bindings with a non-null
// remnawave_uuid, ordered by updated_at, and locks them with SKIP LOCKED to
// prevent concurrent reconciler instances from processing the same rows.
const getFailedForReconciliationQuery = `
SELECT id, subscription_id, platform_user_id, remnawave_uuid, remnawave_short_uuid,
       remnawave_username, purpose, status, traffic_limit_bytes,
       allowed_nodes, inbound_tags, fail_reason, synced_at, created_at, updated_at
FROM multisub.remnawave_bindings
WHERE status = 'failed' AND remnawave_uuid IS NOT NULL
ORDER BY updated_at
LIMIT $1
FOR UPDATE SKIP LOCKED
`

func (r *BindingRepository) GetFailedForReconciliation(ctx context.Context, limit int) ([]*aggregate.RemnawaveBinding, error) {
	db := DBFromContext(ctx, r.pool)
	rows, err := db.Query(ctx, getFailedForReconciliationQuery, limit)
	if err != nil {
		return nil, pgutil.MapErr(err, "get failed bindings for reconciliation", multisub.ErrBindingNotFound)
	}
	defer rows.Close()

	var bindings []*aggregate.RemnawaveBinding
	for rows.Next() {
		var row gen.MultisubRemnawaveBinding
		if err := rows.Scan(
			&row.ID,
			&row.SubscriptionID,
			&row.PlatformUserID,
			&row.RemnawaveUuid,
			&row.RemnawaveShortUuid,
			&row.RemnawaveUsername,
			&row.Purpose,
			&row.Status,
			&row.TrafficLimitBytes,
			&row.AllowedNodes,
			&row.InboundTags,
			&row.FailReason,
			&row.SyncedAt,
			&row.CreatedAt,
			&row.UpdatedAt,
		); err != nil {
			return nil, pgutil.MapErr(err, "scan failed binding for reconciliation", multisub.ErrBindingNotFound)
		}
		bindings = append(bindings, bindingRowToDomain(row))
	}
	if err := rows.Err(); err != nil {
		return nil, pgutil.MapErr(err, "iterate failed bindings for reconciliation", multisub.ErrBindingNotFound)
	}
	return bindings, nil
}

// resetAbandonedReconcilingQuery sets bindings stuck in 'reconciling' for
// longer than the given interval back to 'failed' so they can be retried.
const resetAbandonedReconcilingQuery = `
UPDATE multisub.remnawave_bindings
SET status = 'failed'
WHERE status = 'reconciling' AND updated_at < NOW() - $1::interval
`

func (r *BindingRepository) ResetAbandonedReconciling(ctx context.Context, olderThan time.Duration) (int64, error) {
	db := DBFromContext(ctx, r.pool)
	tag, err := db.Exec(ctx, resetAbandonedReconcilingQuery, olderThan)
	if err != nil {
		return 0, pgutil.MapErr(err, "reset abandoned reconciling bindings", multisub.ErrBindingNotFound)
	}
	return tag.RowsAffected(), nil
}

func (r *BindingRepository) Create(ctx context.Context, b *aggregate.RemnawaveBinding) error {
	err := r.q(ctx).CreateBinding(ctx, gen.CreateBindingParams{
		ID:                 pgutil.UUIDToPgtype(b.ID),
		SubscriptionID:     pgutil.UUIDToPgtype(b.SubscriptionID),
		PlatformUserID:     pgutil.UUIDToPgtype(b.PlatformUserID),
		RemnawaveUuid:      pgutil.StrPtrOrNil(b.RemnawaveUUID),
		RemnawaveShortUuid: pgutil.StrPtrOrNil(b.RemnawaveShortUUID),
		RemnawaveUsername:  b.RemnawaveUsername,
		Purpose:            string(b.Purpose),
		Status:             string(b.Status),
		TrafficLimitBytes:  b.TrafficLimitBytes,
		AllowedNodes:       b.AllowedNodes,
		InboundTags:        b.InboundTags,
		FailReason:         b.FailReason,
		SyncedAt:           pgutil.OptTimeToPgtype(b.SyncedAt),
		CreatedAt:          pgutil.TimeToPgtype(b.CreatedAt),
		UpdatedAt:          pgutil.TimeToPgtype(b.UpdatedAt),
	})
	return pgutil.MapErr(err, "create binding", multisub.ErrBindingNotFound)
}

func (r *BindingRepository) Update(ctx context.Context, b *aggregate.RemnawaveBinding) error {
	err := r.q(ctx).UpdateBinding(ctx, gen.UpdateBindingParams{
		ID:                 pgutil.UUIDToPgtype(b.ID),
		RemnawaveUuid:      pgutil.StrPtrOrNil(b.RemnawaveUUID),
		RemnawaveShortUuid: pgutil.StrPtrOrNil(b.RemnawaveShortUUID),
		Status:             string(b.Status),
		TrafficLimitBytes:  b.TrafficLimitBytes,
		AllowedNodes:       b.AllowedNodes,
		InboundTags:        b.InboundTags,
		FailReason:         b.FailReason,
		SyncedAt:           pgutil.OptTimeToPgtype(b.SyncedAt),
	})
	return pgutil.MapErr(err, "update binding", multisub.ErrBindingNotFound)
}

func (r *BindingRepository) Delete(ctx context.Context, id string) error {
	err := r.q(ctx).DeleteBinding(ctx, pgutil.UUIDToPgtype(id))
	return pgutil.MapErr(err, "delete binding", multisub.ErrBindingNotFound)
}

var _ multisub.BindingRepository = (*BindingRepository)(nil)
