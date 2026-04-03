package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
)

// BindingRepository implements multisub.BindingRepository backed by PostgreSQL.
type BindingRepository struct {
	pool    *pgxpool.Pool
	queries *gen.Queries
}

// NewBindingRepository returns a new BindingRepository using the given pool.
func NewBindingRepository(pool *pgxpool.Pool) *BindingRepository {
	return &BindingRepository{
		pool:    pool,
		queries: gen.New(pool),
	}
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
		SyncedAt:           pgutil.PgtypeToOptTime(row.SyncedAt),
		CreatedAt:          pgutil.PgtypeToTime(row.CreatedAt),
		UpdatedAt:          pgutil.PgtypeToTime(row.UpdatedAt),
	}
}

func (r *BindingRepository) GetByID(ctx context.Context, id string) (*aggregate.RemnawaveBinding, error) {
	row, err := r.queries.GetBindingByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get binding by id", multisub.ErrBindingNotFound)
	}
	return bindingRowToDomain(row), nil
}

func (r *BindingRepository) GetBySubscriptionID(ctx context.Context, subID string) ([]*aggregate.RemnawaveBinding, error) {
	rows, err := r.queries.GetBindingsBySubscriptionID(ctx, pgutil.UUIDToPgtype(subID))
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
	rows, err := r.queries.GetBindingsByPlatformUserID(ctx, pgutil.UUIDToPgtype(userID))
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
	row, err := r.queries.GetBindingByRemnawaveUUID(ctx, &rwUUID)
	if err != nil {
		return nil, pgutil.MapErr(err, "get binding by remnawave uuid", multisub.ErrBindingNotFound)
	}
	return bindingRowToDomain(row), nil
}

func (r *BindingRepository) GetActiveBySubscriptionID(ctx context.Context, subID string) ([]*aggregate.RemnawaveBinding, error) {
	rows, err := r.queries.GetActiveBindingsBySubscriptionID(ctx, pgutil.UUIDToPgtype(subID))
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
	rows, err := r.queries.GetAllActiveBindings(ctx)
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
	rows, err := r.queries.GetFailedBindingsWithRemnawaveUUID(ctx)
	if err != nil {
		return nil, pgutil.MapErr(err, "get failed bindings with remnawave uuid", multisub.ErrBindingNotFound)
	}
	bindings := make([]*aggregate.RemnawaveBinding, len(rows))
	for i, row := range rows {
		bindings[i] = bindingRowToDomain(row)
	}
	return bindings, nil
}

func (r *BindingRepository) Create(ctx context.Context, b *aggregate.RemnawaveBinding) error {
	err := r.queries.CreateBinding(ctx, gen.CreateBindingParams{
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
		SyncedAt:           pgutil.OptTimeToPgtype(b.SyncedAt),
		CreatedAt:          pgutil.TimeToPgtype(b.CreatedAt),
		UpdatedAt:          pgutil.TimeToPgtype(b.UpdatedAt),
	})
	return pgutil.MapErr(err, "create binding", multisub.ErrBindingNotFound)
}

func (r *BindingRepository) Update(ctx context.Context, b *aggregate.RemnawaveBinding) error {
	err := r.queries.UpdateBinding(ctx, gen.UpdateBindingParams{
		ID:                 pgutil.UUIDToPgtype(b.ID),
		RemnawaveUuid:      pgutil.StrPtrOrNil(b.RemnawaveUUID),
		RemnawaveShortUuid: pgutil.StrPtrOrNil(b.RemnawaveShortUUID),
		Status:             string(b.Status),
		TrafficLimitBytes:  b.TrafficLimitBytes,
		AllowedNodes:       b.AllowedNodes,
		InboundTags:        b.InboundTags,
		SyncedAt:           pgutil.OptTimeToPgtype(b.SyncedAt),
	})
	return pgutil.MapErr(err, "update binding", multisub.ErrBindingNotFound)
}

func (r *BindingRepository) Delete(ctx context.Context, id string) error {
	err := r.queries.DeleteBinding(ctx, pgutil.UUIDToPgtype(id))
	return pgutil.MapErr(err, "delete binding", multisub.ErrBindingNotFound)
}

var _ multisub.BindingRepository = (*BindingRepository)(nil)
