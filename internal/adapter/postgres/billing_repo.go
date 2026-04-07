package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
)

// ---------------------------------------------------------------------------
// PlanRepository
// ---------------------------------------------------------------------------

// PlanRepository implements billing.PlanRepository backed by PostgreSQL.
type PlanRepository struct {
	pool  *pgxpool.Pool
	clock clock.Clock
}

// NewPlanRepository returns a new PlanRepository using the given pool.
func NewPlanRepository(pool *pgxpool.Pool, clk clock.Clock) *PlanRepository {
	return &PlanRepository{pool: pool, clock: clk}
}

// q returns a *gen.Queries backed by the active transaction (if any) or the
// pool. This ensures all methods transparently participate in RunInTx and
// respect RLS tenant scoping.
func (r *PlanRepository) q(ctx context.Context) *gen.Queries {
	return gen.New(DBFromContext(ctx, r.pool))
}

// planFields holds the common columns returned by all plan queries.
type planFields struct {
	ID                   pgtype.UUID
	Name                 string
	Description          *string
	BasePriceAmount      int64
	BasePriceCurrency    string
	BillingInterval      string
	TrafficLimitBytes    int64
	DeviceLimit          int32
	AllowedCountries     []string
	AllowedProtocols     []string
	Tier                 string
	MaxRemnawaveBindings int32
	FamilyEnabled        bool
	MaxFamilyMembers     int32
	MaxAddons            int32
	IsActive             bool
	CreatedAt            pgtype.Timestamptz
	UpdatedAt            pgtype.Timestamptz
}

// planRow is a constraint matching all sqlc-generated plan row types.
type planRow interface {
	gen.GetPlanByIDRow | gen.GetAllPlansRow | gen.GetActivePlansRow
}

// extractPlanFields extracts the common fields from any plan row type.
func extractPlanFields[T planRow](row T) planFields {
	switch r := any(row).(type) {
	case gen.GetPlanByIDRow:
		return planFields{r.ID, r.Name, r.Description, r.BasePriceAmount, r.BasePriceCurrency, r.BillingInterval, r.TrafficLimitBytes, r.DeviceLimit, r.AllowedCountries, r.AllowedProtocols, r.Tier, r.MaxRemnawaveBindings, r.FamilyEnabled, r.MaxFamilyMembers, r.MaxAddons, r.IsActive, r.CreatedAt, r.UpdatedAt}
	case gen.GetAllPlansRow:
		return planFields{r.ID, r.Name, r.Description, r.BasePriceAmount, r.BasePriceCurrency, r.BillingInterval, r.TrafficLimitBytes, r.DeviceLimit, r.AllowedCountries, r.AllowedProtocols, r.Tier, r.MaxRemnawaveBindings, r.FamilyEnabled, r.MaxFamilyMembers, r.MaxAddons, r.IsActive, r.CreatedAt, r.UpdatedAt}
	case gen.GetActivePlansRow:
		return planFields{r.ID, r.Name, r.Description, r.BasePriceAmount, r.BasePriceCurrency, r.BillingInterval, r.TrafficLimitBytes, r.DeviceLimit, r.AllowedCountries, r.AllowedProtocols, r.Tier, r.MaxRemnawaveBindings, r.FamilyEnabled, r.MaxFamilyMembers, r.MaxAddons, r.IsActive, r.CreatedAt, r.UpdatedAt}
	default:
		panic("unreachable: unhandled planRow type")
	}
}

// planRowToDomain converts any sqlc-generated plan row type to domain.
func planRowToDomain[T planRow](row T) *aggregate.Plan {
	return planFieldsToDomain(extractPlanFields(row))
}

func planFieldsToDomain(f planFields) *aggregate.Plan {
	return &aggregate.Plan{
		ID:                   pgutil.PgtypeToUUID(f.ID),
		Name:                 f.Name,
		Description:          pgutil.DerefStr(f.Description),
		BasePrice:            vo.NewMoney(f.BasePriceAmount, vo.Currency(f.BasePriceCurrency)),
		Interval:             vo.BillingInterval(f.BillingInterval),
		TrafficLimitBytes:    f.TrafficLimitBytes,
		DeviceLimit:          int(f.DeviceLimit),
		AllowedCountries:     f.AllowedCountries,
		AllowedProtocols:     f.AllowedProtocols,
		Tier:                 aggregate.PlanTier(f.Tier),
		MaxRemnawaveBindings: int(f.MaxRemnawaveBindings),
		FamilyEnabled:        f.FamilyEnabled,
		MaxFamilyMembers:     int(f.MaxFamilyMembers),
		MaxAddons:            int(f.MaxAddons),
		IsActive:             f.IsActive,
		CreatedAt:            pgutil.PgtypeToTime(f.CreatedAt),
		UpdatedAt:            pgutil.PgtypeToTime(f.UpdatedAt),
	}
}

func addonRowToDomain(row gen.BillingPlanAddon) aggregate.Addon {
	return aggregate.Addon{
		ID:                pgutil.PgtypeToUUID(row.ID),
		Name:              row.Name,
		Price:             vo.NewMoney(row.PriceAmount, vo.Currency(row.PriceCurrency)),
		Type:              aggregate.AddonType(row.AddonType),
		ExtraTrafficBytes: row.ExtraTrafficBytes,
		ExtraNodes:        row.ExtraNodes,
		ExtraFeatureFlags: row.ExtraFeatureFlags,
	}
}

func (r *PlanRepository) loadAddons(ctx context.Context, plan *aggregate.Plan) error {
	addonRows, err := r.q(ctx).GetAddonsByPlanID(ctx, pgutil.UUIDToPgtype(plan.ID))
	if err != nil {
		return pgutil.MapErr(err, "get addons for plan", billing.ErrPlanNotFound)
	}
	addons := make([]aggregate.Addon, len(addonRows))
	for i, row := range addonRows {
		addons[i] = addonRowToDomain(row)
	}
	plan.AvailableAddons = addons
	return nil
}

func (r *PlanRepository) GetByID(ctx context.Context, id string) (*aggregate.Plan, error) {
	row, err := r.q(ctx).GetPlanByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get plan by id", billing.ErrPlanNotFound)
	}
	plan := planRowToDomain(row)
	if err := r.loadAddons(ctx, plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func (r *PlanRepository) GetAll(ctx context.Context) ([]*aggregate.Plan, error) {
	rows, err := r.q(ctx).GetAllPlans(ctx)
	if err != nil {
		return nil, pgutil.MapErr(err, "get all plans", billing.ErrPlanNotFound)
	}
	plans := make([]*aggregate.Plan, len(rows))
	for i, row := range rows {
		plans[i] = planRowToDomain(row)
		if err := r.loadAddons(ctx, plans[i]); err != nil {
			return nil, err
		}
	}
	return plans, nil
}

func (r *PlanRepository) GetActive(ctx context.Context) ([]*aggregate.Plan, error) {
	rows, err := r.q(ctx).GetActivePlans(ctx)
	if err != nil {
		return nil, pgutil.MapErr(err, "get active plans", billing.ErrPlanNotFound)
	}
	plans := make([]*aggregate.Plan, len(rows))
	for i, row := range rows {
		plans[i] = planRowToDomain(row)
		if err := r.loadAddons(ctx, plans[i]); err != nil {
			return nil, err
		}
	}
	return plans, nil
}

func (r *PlanRepository) Create(ctx context.Context, plan *aggregate.Plan) error {
	err := r.q(ctx).CreatePlan(ctx, gen.CreatePlanParams{
		ID:                   pgutil.UUIDToPgtype(plan.ID),
		Name:                 plan.Name,
		Description:          pgutil.StrPtrOrNil(plan.Description),
		BasePriceAmount:      plan.BasePrice.Amount,
		BasePriceCurrency:    string(plan.BasePrice.Currency),
		BillingInterval:      string(plan.Interval),
		TrafficLimitBytes:    plan.TrafficLimitBytes,
		DeviceLimit:          int32(plan.DeviceLimit),
		AllowedCountries:     plan.AllowedCountries,
		AllowedProtocols:     plan.AllowedProtocols,
		Tier:                 string(plan.Tier),
		MaxRemnawaveBindings: int32(plan.MaxRemnawaveBindings),
		FamilyEnabled:        plan.FamilyEnabled,
		MaxFamilyMembers:     int32(plan.MaxFamilyMembers),
		MaxAddons:            int32(plan.MaxAddons),
		IsActive:             plan.IsActive,
		CreatedAt:            pgutil.TimeToPgtype(plan.CreatedAt),
		UpdatedAt:            pgutil.TimeToPgtype(plan.UpdatedAt),
	})
	if err != nil {
		return pgutil.MapErr(err, "create plan", billing.ErrPlanNotFound)
	}

	for _, addon := range plan.AvailableAddons {
		if err := r.createAddon(ctx, plan.ID, addon); err != nil {
			return fmt.Errorf("create addon %s for plan: %w", addon.ID, err)
		}
	}
	return nil
}

func (r *PlanRepository) createAddon(ctx context.Context, planID string, addon aggregate.Addon) error {
	err := r.q(ctx).CreatePlanAddon(ctx, gen.CreatePlanAddonParams{
		ID:                pgutil.UUIDToPgtype(addon.ID),
		PlanID:            pgutil.UUIDToPgtype(planID),
		Name:              addon.Name,
		PriceAmount:       addon.Price.Amount,
		PriceCurrency:     string(addon.Price.Currency),
		AddonType:         string(addon.Type),
		ExtraTrafficBytes: addon.ExtraTrafficBytes,
		ExtraNodes:        addon.ExtraNodes,
		ExtraFeatureFlags: addon.ExtraFeatureFlags,
		CreatedAt:         pgutil.TimeToPgtype(r.clock.Now()),
	})
	return pgutil.MapErr(err, "create plan addon", billing.ErrPlanNotFound)
}

func (r *PlanRepository) upsertAddon(ctx context.Context, planID string, addon aggregate.Addon) error {
	err := r.q(ctx).UpsertPlanAddon(ctx, gen.UpsertPlanAddonParams{
		ID:                pgutil.UUIDToPgtype(addon.ID),
		PlanID:            pgutil.UUIDToPgtype(planID),
		Name:              addon.Name,
		PriceAmount:       addon.Price.Amount,
		PriceCurrency:     string(addon.Price.Currency),
		AddonType:         string(addon.Type),
		ExtraTrafficBytes: addon.ExtraTrafficBytes,
		ExtraNodes:        addon.ExtraNodes,
		ExtraFeatureFlags: addon.ExtraFeatureFlags,
		CreatedAt:         pgutil.TimeToPgtype(r.clock.Now()),
	})
	return pgutil.MapErr(err, "upsert plan addon", billing.ErrPlanNotFound)
}

func (r *PlanRepository) Update(ctx context.Context, plan *aggregate.Plan) error {
	err := r.q(ctx).UpdatePlan(ctx, gen.UpdatePlanParams{
		ID:                   pgutil.UUIDToPgtype(plan.ID),
		Name:                 plan.Name,
		Description:          pgutil.StrPtrOrNil(plan.Description),
		BasePriceAmount:      plan.BasePrice.Amount,
		BasePriceCurrency:    string(plan.BasePrice.Currency),
		BillingInterval:      string(plan.Interval),
		TrafficLimitBytes:    plan.TrafficLimitBytes,
		DeviceLimit:          int32(plan.DeviceLimit),
		AllowedCountries:     plan.AllowedCountries,
		AllowedProtocols:     plan.AllowedProtocols,
		Tier:                 string(plan.Tier),
		MaxRemnawaveBindings: int32(plan.MaxRemnawaveBindings),
		FamilyEnabled:        plan.FamilyEnabled,
		MaxFamilyMembers:     int32(plan.MaxFamilyMembers),
		MaxAddons:            int32(plan.MaxAddons),
		IsActive:             plan.IsActive,
	})
	if err != nil {
		return pgutil.MapErr(err, "update plan", billing.ErrPlanNotFound)
	}

	// Upsert each addon, then prune addons removed from the list.
	// This avoids DELETE ALL + RE-INSERT which breaks FK references
	// from other tables pointing to addon IDs.
	retainIDs := make([]pgtype.UUID, 0, len(plan.AvailableAddons))
	for _, addon := range plan.AvailableAddons {
		if err := r.upsertAddon(ctx, plan.ID, addon); err != nil {
			return fmt.Errorf("upsert addon %s for plan: %w", addon.ID, err)
		}
		retainIDs = append(retainIDs, pgutil.UUIDToPgtype(addon.ID))
	}

	if err := r.q(ctx).DeleteRemovedAddons(ctx, gen.DeleteRemovedAddonsParams{
		PlanID: pgutil.UUIDToPgtype(plan.ID),
		Ids:    retainIDs,
	}); err != nil {
		return pgutil.MapErr(err, "delete removed addons for plan", billing.ErrPlanNotFound)
	}
	return nil
}

var _ billing.PlanRepository = (*PlanRepository)(nil)

// ---------------------------------------------------------------------------
// SubscriptionRepository
// ---------------------------------------------------------------------------

// SubscriptionRepository implements billing.SubscriptionRepository backed by PostgreSQL.
type SubscriptionRepository struct {
	pool *pgxpool.Pool
}

// NewSubscriptionRepository returns a new SubscriptionRepository using the given pool.
func NewSubscriptionRepository(pool *pgxpool.Pool) *SubscriptionRepository {
	return &SubscriptionRepository{pool: pool}
}

// q returns a *gen.Queries backed by the active transaction (if any) or the
// pool. This ensures all methods transparently participate in RunInTx and
// respect RLS tenant scoping.
func (r *SubscriptionRepository) q(ctx context.Context) *gen.Queries {
	return gen.New(DBFromContext(ctx, r.pool))
}

// subFields holds the common columns returned by all subscription queries.
// Using an intermediate struct avoids a 15-parameter function and eliminates
// the risk of silently swapping same-typed positional arguments.
type subFields struct {
	ID                    pgtype.UUID
	UserID                pgtype.UUID
	PlanID                pgtype.UUID
	Status                string
	PeriodStart           pgtype.Timestamptz
	PeriodEnd             pgtype.Timestamptz
	PeriodInterval        string
	AddonIds              []pgtype.UUID
	AssignedTo            *string
	PendingPlanID         pgtype.UUID
	PendingOriginalPlanID pgtype.UUID
	PendingRequestedAt    pgtype.Timestamptz
	CancelledAt           pgtype.Timestamptz
	PausedAt              pgtype.Timestamptz
	CreatedAt             pgtype.Timestamptz
	UpdatedAt             pgtype.Timestamptz
}

// subRow is a constraint matching all sqlc-generated subscription row types.
// Each query returns a separate struct because the explicit column list differs
// from the model (which now includes the billing_period generated column).
type subRow interface {
	gen.GetSubscriptionByIDRow | gen.GetSubscriptionByIDForUpdateRow | gen.GetSubscriptionsByUserIDRow | gen.GetActiveSubscriptionsByUserIDRow | gen.GetAllSubscriptionsRow
}

// extractSubFields extracts the common fields from any subscription row type.
func extractSubFields[T subRow](row T) subFields {
	switch r := any(row).(type) {
	case gen.GetSubscriptionByIDRow:
		return subFields{r.ID, r.UserID, r.PlanID, r.Status, r.PeriodStart, r.PeriodEnd, r.PeriodInterval, r.AddonIds, r.AssignedTo, r.PendingPlanID, r.PendingOriginalPlanID, r.PendingRequestedAt, r.CancelledAt, r.PausedAt, r.CreatedAt, r.UpdatedAt}
	case gen.GetSubscriptionByIDForUpdateRow:
		return subFields{r.ID, r.UserID, r.PlanID, r.Status, r.PeriodStart, r.PeriodEnd, r.PeriodInterval, r.AddonIds, r.AssignedTo, r.PendingPlanID, r.PendingOriginalPlanID, r.PendingRequestedAt, r.CancelledAt, r.PausedAt, r.CreatedAt, r.UpdatedAt}
	case gen.GetSubscriptionsByUserIDRow:
		return subFields{r.ID, r.UserID, r.PlanID, r.Status, r.PeriodStart, r.PeriodEnd, r.PeriodInterval, r.AddonIds, r.AssignedTo, r.PendingPlanID, r.PendingOriginalPlanID, r.PendingRequestedAt, r.CancelledAt, r.PausedAt, r.CreatedAt, r.UpdatedAt}
	case gen.GetActiveSubscriptionsByUserIDRow:
		return subFields{r.ID, r.UserID, r.PlanID, r.Status, r.PeriodStart, r.PeriodEnd, r.PeriodInterval, r.AddonIds, r.AssignedTo, r.PendingPlanID, r.PendingOriginalPlanID, r.PendingRequestedAt, r.CancelledAt, r.PausedAt, r.CreatedAt, r.UpdatedAt}
	case gen.GetAllSubscriptionsRow:
		return subFields{r.ID, r.UserID, r.PlanID, r.Status, r.PeriodStart, r.PeriodEnd, r.PeriodInterval, r.AddonIds, r.AssignedTo, r.PendingPlanID, r.PendingOriginalPlanID, r.PendingRequestedAt, r.CancelledAt, r.PausedAt, r.CreatedAt, r.UpdatedAt}
	default:
		panic("unreachable: unhandled subRow type")
	}
}

// subRowToDomain converts any sqlc-generated subscription row type to domain.
func subRowToDomain[T subRow](row T) *aggregate.Subscription {
	return subFieldsToDomain(extractSubFields(row))
}

func (r *SubscriptionRepository) GetByID(ctx context.Context, id string) (*aggregate.Subscription, error) {
	row, err := r.q(ctx).GetSubscriptionByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get subscription by id", billing.ErrSubscriptionNotFound)
	}
	return subRowToDomain(row), nil
}

func (r *SubscriptionRepository) GetByIDForUpdate(ctx context.Context, id string) (*aggregate.Subscription, error) {
	row, err := r.q(ctx).GetSubscriptionByIDForUpdate(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get subscription by id for update", billing.ErrSubscriptionNotFound)
	}
	return subRowToDomain(row), nil
}

func (r *SubscriptionRepository) GetByUserID(ctx context.Context, userID string) ([]*aggregate.Subscription, error) {
	rows, err := r.q(ctx).GetSubscriptionsByUserID(ctx, pgutil.UUIDToPgtype(userID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get subscriptions by user id", billing.ErrSubscriptionNotFound)
	}
	subs := make([]*aggregate.Subscription, len(rows))
	for i, row := range rows {
		subs[i] = subRowToDomain(row)
	}
	return subs, nil
}

func (r *SubscriptionRepository) GetActiveByUserID(ctx context.Context, userID string) ([]*aggregate.Subscription, error) {
	rows, err := r.q(ctx).GetActiveSubscriptionsByUserID(ctx, pgutil.UUIDToPgtype(userID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get active subscriptions by user id", billing.ErrSubscriptionNotFound)
	}
	subs := make([]*aggregate.Subscription, len(rows))
	for i, row := range rows {
		subs[i] = subRowToDomain(row)
	}
	return subs, nil
}

func (r *SubscriptionRepository) GetAll(ctx context.Context, limit, offset int) ([]*aggregate.Subscription, error) {
	rows, err := r.q(ctx).GetAllSubscriptions(ctx, gen.GetAllSubscriptionsParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, pgutil.MapErr(err, "get all subscriptions", billing.ErrSubscriptionNotFound)
	}
	subs := make([]*aggregate.Subscription, len(rows))
	for i, row := range rows {
		subs[i] = subRowToDomain(row)
	}
	return subs, nil
}

func (r *SubscriptionRepository) Create(ctx context.Context, sub *aggregate.Subscription) error {
	err := r.q(ctx).CreateSubscription(ctx, gen.CreateSubscriptionParams{
		ID:                    pgutil.UUIDToPgtype(sub.ID),
		UserID:                pgutil.UUIDToPgtype(sub.UserID),
		PlanID:                pgutil.UUIDToPgtype(sub.PlanID),
		Status:                string(sub.Status),
		PeriodStart:           pgutil.TimeToPgtype(sub.Period.Start),
		PeriodEnd:             pgutil.TimeToPgtype(sub.Period.End),
		PeriodInterval:        string(sub.Period.Interval),
		AddonIds:              pgutil.StringsToPgtypeUUIDs(sub.AddonIDs),
		AssignedTo:            pgutil.StrPtrOrNil(sub.AssignedTo),
		PendingPlanID:         pendingChangeToPlanID(sub.PendingChange),
		PendingOriginalPlanID: pendingChangeToOriginalPlanID(sub.PendingChange),
		PendingRequestedAt:    pendingChangeToRequestedAt(sub.PendingChange),
		CancelledAt:           pgutil.OptTimeToPgtype(sub.CancelledAt),
		PausedAt:              pgutil.OptTimeToPgtype(sub.PausedAt),
		CreatedAt:             pgutil.TimeToPgtype(sub.CreatedAt),
		UpdatedAt:             pgutil.TimeToPgtype(sub.UpdatedAt),
	})
	return pgutil.MapErr(err, "create subscription", billing.ErrSubscriptionNotFound)
}

func (r *SubscriptionRepository) Update(ctx context.Context, sub *aggregate.Subscription) error {
	err := r.q(ctx).UpdateSubscription(ctx, gen.UpdateSubscriptionParams{
		ID:                    pgutil.UUIDToPgtype(sub.ID),
		Status:                string(sub.Status),
		PeriodStart:           pgutil.TimeToPgtype(sub.Period.Start),
		PeriodEnd:             pgutil.TimeToPgtype(sub.Period.End),
		PeriodInterval:        string(sub.Period.Interval),
		AddonIds:              pgutil.StringsToPgtypeUUIDs(sub.AddonIDs),
		AssignedTo:            pgutil.StrPtrOrNil(sub.AssignedTo),
		PendingPlanID:         pendingChangeToPlanID(sub.PendingChange),
		PendingOriginalPlanID: pendingChangeToOriginalPlanID(sub.PendingChange),
		PendingRequestedAt:    pendingChangeToRequestedAt(sub.PendingChange),
		CancelledAt:           pgutil.OptTimeToPgtype(sub.CancelledAt),
		PausedAt:              pgutil.OptTimeToPgtype(sub.PausedAt),
	})
	return pgutil.MapErr(err, "update subscription", billing.ErrSubscriptionNotFound)
}

// updateSubscriptionStatusSQL uses PG18 native OLD/NEW in RETURNING to
// atomically capture both the previous and new status in a single round-trip.
// This bypasses sqlc (which does not yet support OLD/NEW syntax) and uses
// pgx directly. The query is race-free unlike the CTE-based alternative.
const updateSubscriptionStatusSQL = `UPDATE billing.subscriptions SET status = $2 WHERE id = $1 RETURNING old.status AS previous_status, new.status AS current_status`

// UpdateStatus atomically transitions a subscription's status and returns both
// the old and new values for audit trail and event payloads.
func (r *SubscriptionRepository) UpdateStatus(ctx context.Context, id string, newStatus aggregate.SubscriptionStatus) (*billing.StatusTransition, error) {
	var prev, curr string
	db := DBFromContext(ctx, r.pool)
	err := db.QueryRow(ctx, updateSubscriptionStatusSQL, pgutil.UUIDToPgtype(id), string(newStatus)).Scan(&prev, &curr)
	if err != nil {
		return nil, pgutil.MapErr(err, "update subscription status", billing.ErrSubscriptionNotFound)
	}
	return &billing.StatusTransition{
		PreviousStatus: aggregate.SubscriptionStatus(prev),
		CurrentStatus:  aggregate.SubscriptionStatus(curr),
	}, nil
}

// getActiveSubscriptionByUserAtTimeSQL uses the GiST index on billing_period
// via the @> containment operator to find the subscription whose billing period
// contains the given point in time. Includes 'past_due' because a past-due
// subscription still has a valid billing period (unlike the stricter
// GetActiveSubscriptionsByUserID which only returns 'trial'/'active').
// This bypasses sqlc, which does not support the @> operator on tstzrange.
const getActiveSubscriptionByUserAtTimeSQL = `
SELECT id, user_id, plan_id, status, period_start, period_end, period_interval,
       addon_ids, assigned_to, pending_plan_id, pending_original_plan_id, pending_requested_at,
       cancelled_at, paused_at, created_at, updated_at
FROM billing.subscriptions
WHERE user_id = $1
  AND billing_period @> $2::timestamptz
  AND status IN ('trial', 'active', 'past_due')
LIMIT 1`

// GetActiveByUserAtTime returns the single active subscription whose billing
// period contains the given point in time.
func (r *SubscriptionRepository) GetActiveByUserAtTime(ctx context.Context, userID string, at time.Time) (*aggregate.Subscription, error) {
	db := DBFromContext(ctx, r.pool)
	row := db.QueryRow(ctx, getActiveSubscriptionByUserAtTimeSQL, pgutil.UUIDToPgtype(userID), pgutil.TimeToPgtype(at))

	var f subFields
	err := row.Scan(
		&f.ID, &f.UserID, &f.PlanID, &f.Status,
		&f.PeriodStart, &f.PeriodEnd, &f.PeriodInterval,
		&f.AddonIds, &f.AssignedTo, &f.PendingPlanID, &f.PendingOriginalPlanID, &f.PendingRequestedAt,
		&f.CancelledAt, &f.PausedAt,
		&f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		return nil, pgutil.MapErr(err, "get active subscription by user at time", billing.ErrSubscriptionNotFound)
	}
	return subFieldsToDomain(f), nil
}

// getOverlappingSubscriptionsSQL uses the GiST index on billing_period via the
// && overlap operator to find subscriptions whose billing period overlaps the
// given [start, end) range. Includes 'paused' because paused subscriptions
// still occupy their billing period and must block overlapping subscriptions
// (matches the EXCLUDE constraint filter in migration 011).
// This bypasses sqlc, which does not support the && operator on tstzrange.
const getOverlappingSubscriptionsSQL = `
SELECT id, user_id, plan_id, status, period_start, period_end, period_interval,
       addon_ids, assigned_to, pending_plan_id, pending_original_plan_id, pending_requested_at,
       cancelled_at, paused_at, created_at, updated_at
FROM billing.subscriptions
WHERE user_id = $1
  AND plan_id = $2
  AND billing_period && tstzrange($3, $4, '[)')
  AND status IN ('trial', 'active', 'past_due', 'paused')`

// GetOverlapping returns subscriptions whose billing period overlaps the given
// [start, end) range for a specific user and plan.
func (r *SubscriptionRepository) GetOverlapping(ctx context.Context, userID, planID string, start, end time.Time) ([]*aggregate.Subscription, error) {
	db := DBFromContext(ctx, r.pool)
	rows, err := db.Query(ctx, getOverlappingSubscriptionsSQL,
		pgutil.UUIDToPgtype(userID),
		pgutil.UUIDToPgtype(planID),
		pgutil.TimeToPgtype(start),
		pgutil.TimeToPgtype(end),
	)
	if err != nil {
		return nil, pgutil.MapErr(err, "get overlapping subscriptions", billing.ErrSubscriptionNotFound)
	}
	defer rows.Close()

	var subs []*aggregate.Subscription
	for rows.Next() {
		var f subFields
		if err := rows.Scan(
			&f.ID, &f.UserID, &f.PlanID, &f.Status,
			&f.PeriodStart, &f.PeriodEnd, &f.PeriodInterval,
			&f.AddonIds, &f.AssignedTo, &f.PendingPlanID, &f.PendingOriginalPlanID, &f.PendingRequestedAt,
			&f.CancelledAt, &f.PausedAt,
			&f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, pgutil.MapErr(err, "scan overlapping subscription", billing.ErrSubscriptionNotFound)
		}
		subs = append(subs, subFieldsToDomain(f))
	}
	if err := rows.Err(); err != nil {
		return nil, pgutil.MapErr(err, "iterate overlapping subscriptions", billing.ErrSubscriptionNotFound)
	}
	return subs, nil
}

// subFieldsToDomain converts a raw subFields struct to a domain Subscription.
// Used by raw pgx queries that bypass sqlc-generated row types.
func subFieldsToDomain(f subFields) *aggregate.Subscription {
	return &aggregate.Subscription{
		ID:     pgutil.PgtypeToUUID(f.ID),
		UserID: pgutil.PgtypeToUUID(f.UserID),
		PlanID: pgutil.PgtypeToUUID(f.PlanID),
		Status: aggregate.SubscriptionStatus(f.Status),
		Period: vo.BillingPeriod{
			Start:    pgutil.PgtypeToTime(f.PeriodStart),
			End:      pgutil.PgtypeToTime(f.PeriodEnd),
			Interval: vo.BillingInterval(f.PeriodInterval),
		},
		AddonIDs:      pgutil.PgtypeUUIDsToStrings(f.AddonIds),
		AssignedTo:    pgutil.DerefStr(f.AssignedTo),
		PendingChange: pgtypeToPendingChange(f.PendingPlanID, f.PendingOriginalPlanID, f.PendingRequestedAt),
		CancelledAt:   pgutil.PgtypeToOptTime(f.CancelledAt),
		PausedAt:      pgutil.PgtypeToOptTime(f.PausedAt),
		CreatedAt:     pgutil.PgtypeToTime(f.CreatedAt),
		UpdatedAt:     pgutil.PgtypeToTime(f.UpdatedAt),
	}
}

// pgtypeToPendingChange maps the three pending-change DB columns to the
// PendingPlanChange value object. Returns a zero VO when no pending change
// exists (pending_plan_id is NULL).
func pgtypeToPendingChange(planID, originalPlanID pgtype.UUID, requestedAt pgtype.Timestamptz) vo.PendingPlanChange {
	if !planID.Valid {
		return vo.PendingPlanChange{}
	}
	return vo.PendingPlanChange{
		PlanID:         pgutil.PgtypeToUUID(planID),
		OriginalPlanID: pgutil.PgtypeToUUID(originalPlanID),
		RequestedAt:    pgutil.PgtypeToTime(requestedAt),
	}
}

// pendingChangeToPlanID maps PendingPlanChange.PlanID to a nullable DB UUID.
func pendingChangeToPlanID(pc vo.PendingPlanChange) pgtype.UUID {
	if pc.IsZero() {
		return pgtype.UUID{}
	}
	return pgutil.UUIDToPgtype(pc.PlanID)
}

// pendingChangeToOriginalPlanID maps PendingPlanChange.OriginalPlanID to a nullable DB UUID.
func pendingChangeToOriginalPlanID(pc vo.PendingPlanChange) pgtype.UUID {
	if pc.IsZero() {
		return pgtype.UUID{}
	}
	return pgutil.UUIDToPgtype(pc.OriginalPlanID)
}

// pendingChangeToRequestedAt maps PendingPlanChange.RequestedAt to a nullable DB Timestamptz.
func pendingChangeToRequestedAt(pc vo.PendingPlanChange) pgtype.Timestamptz {
	if pc.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgutil.TimeToPgtype(pc.RequestedAt)
}

// getExpiredActiveSubscriptionsSQL fetches active subscriptions whose billing
// period ended before the given timestamp. These are candidates for renewal or
// expiration by the SubscriptionScheduler. The query orders by period_end ASC
// so that the oldest elapsed subscriptions are processed first.
const getExpiredActiveSubscriptionsSQL = `
SELECT id, user_id, plan_id, status, period_start, period_end, period_interval,
       addon_ids, assigned_to, pending_plan_id, pending_original_plan_id, pending_requested_at,
       cancelled_at, paused_at, created_at, updated_at
FROM billing.subscriptions
WHERE status = 'active'
  AND period_end < $1
ORDER BY period_end ASC
LIMIT $2`

// GetExpiredActive returns active subscriptions whose billing period ended
// before the given time, up to the specified limit.
func (r *SubscriptionRepository) GetExpiredActive(ctx context.Context, before time.Time, limit int) ([]*aggregate.Subscription, error) {
	db := DBFromContext(ctx, r.pool)
	rows, err := db.Query(ctx, getExpiredActiveSubscriptionsSQL,
		pgutil.TimeToPgtype(before),
		int32(limit),
	)
	if err != nil {
		return nil, pgutil.MapErr(err, "get expired active subscriptions", billing.ErrSubscriptionNotFound)
	}
	defer rows.Close()

	var subs []*aggregate.Subscription
	for rows.Next() {
		var f subFields
		if err := rows.Scan(
			&f.ID, &f.UserID, &f.PlanID, &f.Status,
			&f.PeriodStart, &f.PeriodEnd, &f.PeriodInterval,
			&f.AddonIds, &f.AssignedTo, &f.PendingPlanID, &f.PendingOriginalPlanID, &f.PendingRequestedAt,
			&f.CancelledAt, &f.PausedAt,
			&f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, pgutil.MapErr(err, "scan expired active subscription", billing.ErrSubscriptionNotFound)
		}
		subs = append(subs, subFieldsToDomain(f))
	}
	if err := rows.Err(); err != nil {
		return nil, pgutil.MapErr(err, "iterate expired active subscriptions", billing.ErrSubscriptionNotFound)
	}
	return subs, nil
}

// getAllSubscriptionsCursorFirstPageSQL returns the first page of subscriptions
// ordered by (created_at DESC, id DESC). Used when no cursor is provided.
const getAllSubscriptionsCursorFirstPageSQL = `
SELECT id, user_id, plan_id, status, period_start, period_end, period_interval,
       addon_ids, assigned_to, pending_plan_id, pending_original_plan_id, pending_requested_at,
       cancelled_at, paused_at, created_at, updated_at
FROM billing.subscriptions
ORDER BY created_at DESC, id DESC
LIMIT $1`

// getAllSubscriptionsCursorNextPageSQL returns subsequent pages using cursor-based
// pagination keyed on (created_at DESC, id DESC). Bypasses sqlc because the
// conditional WHERE clause for optional cursor parameters is not supported.
const getAllSubscriptionsCursorNextPageSQL = `
SELECT id, user_id, plan_id, status, period_start, period_end, period_interval,
       addon_ids, assigned_to, pending_plan_id, pending_original_plan_id, pending_requested_at,
       cancelled_at, paused_at, created_at, updated_at
FROM billing.subscriptions
WHERE (created_at, id) < ($2, $3)
ORDER BY created_at DESC, id DESC
LIMIT $1`

// GetAllCursor returns subscriptions using cursor-based pagination keyed on
// (created_at DESC, id DESC). Nil cursor returns the first page.
func (r *SubscriptionRepository) GetAllCursor(ctx context.Context, params billing.CursorParams) ([]*aggregate.Subscription, error) {
	db := DBFromContext(ctx, r.pool)

	if params.CreatedAt == nil || params.ID == nil {
		rows, err := db.Query(ctx, getAllSubscriptionsCursorFirstPageSQL, int32(params.Limit))
		if err != nil {
			return nil, pgutil.MapErr(err, "get all subscriptions cursor first page", billing.ErrSubscriptionNotFound)
		}
		defer rows.Close()
		return scanSubscriptionRows(rows)
	}

	rows, err := db.Query(ctx, getAllSubscriptionsCursorNextPageSQL,
		int32(params.Limit),
		pgutil.TimeToPgtype(*params.CreatedAt),
		pgutil.UUIDToPgtype(*params.ID),
	)
	if err != nil {
		return nil, pgutil.MapErr(err, "get all subscriptions cursor next page", billing.ErrSubscriptionNotFound)
	}
	defer rows.Close()
	return scanSubscriptionRows(rows)
}

// scanSubscriptionRows scans pgx rows into a slice of domain subscriptions.
// Shared by GetAllCursor to avoid duplicating the scan loop.
func scanSubscriptionRows(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
},
) ([]*aggregate.Subscription, error) {
	var subs []*aggregate.Subscription
	for rows.Next() {
		var f subFields
		if err := rows.Scan(
			&f.ID, &f.UserID, &f.PlanID, &f.Status,
			&f.PeriodStart, &f.PeriodEnd, &f.PeriodInterval,
			&f.AddonIds, &f.AssignedTo, &f.PendingPlanID, &f.PendingOriginalPlanID, &f.PendingRequestedAt,
			&f.CancelledAt, &f.PausedAt,
			&f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, pgutil.MapErr(err, "scan subscription cursor row", billing.ErrSubscriptionNotFound)
		}
		subs = append(subs, subFieldsToDomain(f))
	}
	if err := rows.Err(); err != nil {
		return nil, pgutil.MapErr(err, "iterate subscription cursor rows", billing.ErrSubscriptionNotFound)
	}
	return subs, nil
}

var _ billing.SubscriptionRepository = (*SubscriptionRepository)(nil)

// ---------------------------------------------------------------------------
// InvoiceRepository
// ---------------------------------------------------------------------------

// InvoiceRepository implements billing.InvoiceRepository backed by PostgreSQL.
type InvoiceRepository struct {
	pool *pgxpool.Pool
}

// NewInvoiceRepository returns a new InvoiceRepository using the given pool.
func NewInvoiceRepository(pool *pgxpool.Pool) *InvoiceRepository {
	return &InvoiceRepository{pool: pool}
}

// q returns a *gen.Queries backed by the active transaction (if any) or the
// pool. This ensures all methods transparently participate in RunInTx and
// respect RLS tenant scoping.
func (r *InvoiceRepository) q(ctx context.Context) *gen.Queries {
	return gen.New(DBFromContext(ctx, r.pool))
}

// invFields holds the common columns returned by all invoice queries.
type invFields struct {
	ID                  pgtype.UUID
	SubscriptionID      pgtype.UUID
	UserID              pgtype.UUID
	SubtotalAmount      int64
	TotalDiscountAmount int64
	TotalAmount         int64
	Currency            string
	PricingReason       string
	Discounts           []byte
	Status              string
	PaidAt              pgtype.Timestamptz
	CreatedAt           pgtype.Timestamptz
	UpdatedAt           pgtype.Timestamptz
}

// invRow is a constraint matching all sqlc-generated invoice row types.
type invRow interface {
	gen.GetInvoiceByIDRow | gen.GetInvoiceByIDForUpdateRow | gen.GetInvoicesBySubscriptionIDRow | gen.GetPendingInvoicesByUserIDRow | gen.GetAllInvoicesRow
}

func extractInvFields[T invRow](row T) invFields {
	switch r := any(row).(type) {
	case gen.GetInvoiceByIDRow:
		return invFields{r.ID, r.SubscriptionID, r.UserID, r.SubtotalAmount, r.TotalDiscountAmount, r.TotalAmount, r.Currency, r.PricingReason, r.Discounts, r.Status, r.PaidAt, r.CreatedAt, r.UpdatedAt}
	case gen.GetInvoiceByIDForUpdateRow:
		return invFields{r.ID, r.SubscriptionID, r.UserID, r.SubtotalAmount, r.TotalDiscountAmount, r.TotalAmount, r.Currency, r.PricingReason, r.Discounts, r.Status, r.PaidAt, r.CreatedAt, r.UpdatedAt}
	case gen.GetInvoicesBySubscriptionIDRow:
		return invFields{r.ID, r.SubscriptionID, r.UserID, r.SubtotalAmount, r.TotalDiscountAmount, r.TotalAmount, r.Currency, r.PricingReason, r.Discounts, r.Status, r.PaidAt, r.CreatedAt, r.UpdatedAt}
	case gen.GetPendingInvoicesByUserIDRow:
		return invFields{r.ID, r.SubscriptionID, r.UserID, r.SubtotalAmount, r.TotalDiscountAmount, r.TotalAmount, r.Currency, r.PricingReason, r.Discounts, r.Status, r.PaidAt, r.CreatedAt, r.UpdatedAt}
	case gen.GetAllInvoicesRow:
		return invFields{r.ID, r.SubscriptionID, r.UserID, r.SubtotalAmount, r.TotalDiscountAmount, r.TotalAmount, r.Currency, r.PricingReason, r.Discounts, r.Status, r.PaidAt, r.CreatedAt, r.UpdatedAt}
	default:
		panic("unreachable: unhandled invRow type")
	}
}

func invoiceRowToDomain[T invRow](row T) (*aggregate.Invoice, error) {
	return invFieldsToDomain(extractInvFields(row))
}

// invFieldsToDomain converts a raw invFields struct to a domain Invoice. Used
// by both the generic invoiceRowToDomain and raw pgx FOR UPDATE queries.
func invFieldsToDomain(f invFields) (*aggregate.Invoice, error) {
	var discounts []vo.Discount
	if len(f.Discounts) > 0 {
		if err := json.Unmarshal(f.Discounts, &discounts); err != nil {
			return nil, fmt.Errorf("unmarshal discounts: %w", err)
		}
	}
	return &aggregate.Invoice{
		ID:             pgutil.PgtypeToUUID(f.ID),
		SubscriptionID: pgutil.PgtypeToUUID(f.SubscriptionID),
		UserID:         pgutil.PgtypeToUUID(f.UserID),
		Discounts:      discounts,
		Subtotal:       vo.NewMoney(f.SubtotalAmount, vo.Currency(f.Currency)),
		TotalDiscount:  vo.NewMoney(f.TotalDiscountAmount, vo.Currency(f.Currency)),
		Total:          vo.NewMoney(f.TotalAmount, vo.Currency(f.Currency)),
		PricingReason:  f.PricingReason,
		Status:         aggregate.InvoiceStatus(f.Status),
		PaidAt:         pgutil.PgtypeToOptTime(f.PaidAt),
		CreatedAt:      pgutil.PgtypeToTime(f.CreatedAt),
		UpdatedAt:      pgutil.PgtypeToTime(f.UpdatedAt),
	}, nil
}

func lineItemRowToDomain(row gen.BillingInvoiceLineItem) vo.LineItem {
	return vo.LineItem{
		Description: row.Description,
		Type:        vo.LineItemType(row.ItemType),
		Amount:      vo.NewMoney(row.Amount, vo.Currency(row.Currency)),
		Quantity:    int(row.Quantity),
	}
}

func (r *InvoiceRepository) loadLineItems(ctx context.Context, inv *aggregate.Invoice) error {
	rows, err := r.q(ctx).GetLineItemsByInvoiceID(ctx, pgutil.UUIDToPgtype(inv.ID))
	if err != nil {
		return pgutil.MapErr(err, "get line items for invoice", billing.ErrInvoiceNotFound)
	}
	items := make([]vo.LineItem, len(rows))
	for i, row := range rows {
		items[i] = lineItemRowToDomain(row)
	}
	inv.LineItems = items
	return nil
}

func (r *InvoiceRepository) GetByID(ctx context.Context, id string) (*aggregate.Invoice, error) {
	row, err := r.q(ctx).GetInvoiceByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get invoice by id", billing.ErrInvoiceNotFound)
	}
	inv, err := invoiceRowToDomain(row)
	if err != nil {
		return nil, fmt.Errorf("get invoice by id: %w", err)
	}
	if err := r.loadLineItems(ctx, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

func (r *InvoiceRepository) GetByIDForUpdate(ctx context.Context, id string) (*aggregate.Invoice, error) {
	row, err := r.q(ctx).GetInvoiceByIDForUpdate(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get invoice by id for update", billing.ErrInvoiceNotFound)
	}
	inv, err := invoiceRowToDomain(row)
	if err != nil {
		return nil, fmt.Errorf("get invoice by id for update: %w", err)
	}
	if err := r.loadLineItems(ctx, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

func (r *InvoiceRepository) GetBySubscriptionID(ctx context.Context, subID string) ([]*aggregate.Invoice, error) {
	rows, err := r.q(ctx).GetInvoicesBySubscriptionID(ctx, pgutil.UUIDToPgtype(subID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get invoices by subscription id", billing.ErrInvoiceNotFound)
	}
	invoices := make([]*aggregate.Invoice, len(rows))
	for i, row := range rows {
		inv, err := invoiceRowToDomain(row)
		if err != nil {
			return nil, fmt.Errorf("get invoices by subscription id: %w", err)
		}
		invoices[i] = inv
		if err := r.loadLineItems(ctx, invoices[i]); err != nil {
			return nil, err
		}
	}
	return invoices, nil
}

func (r *InvoiceRepository) GetPendingByUserID(ctx context.Context, userID string) ([]*aggregate.Invoice, error) {
	rows, err := r.q(ctx).GetPendingInvoicesByUserID(ctx, pgutil.UUIDToPgtype(userID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get pending invoices by user id", billing.ErrInvoiceNotFound)
	}
	invoices := make([]*aggregate.Invoice, len(rows))
	for i, row := range rows {
		inv, err := invoiceRowToDomain(row)
		if err != nil {
			return nil, fmt.Errorf("get pending invoices by user id: %w", err)
		}
		invoices[i] = inv
		if err := r.loadLineItems(ctx, invoices[i]); err != nil {
			return nil, err
		}
	}
	return invoices, nil
}

func (r *InvoiceRepository) GetAll(ctx context.Context, limit, offset int) ([]*aggregate.Invoice, error) {
	rows, err := r.q(ctx).GetAllInvoices(ctx, gen.GetAllInvoicesParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, pgutil.MapErr(err, "get all invoices", billing.ErrInvoiceNotFound)
	}
	invoices := make([]*aggregate.Invoice, len(rows))
	for i, row := range rows {
		inv, err := invoiceRowToDomain(row)
		if err != nil {
			return nil, fmt.Errorf("get all invoices: %w", err)
		}
		invoices[i] = inv
	}
	return invoices, nil
}

func (r *InvoiceRepository) Create(ctx context.Context, inv *aggregate.Invoice) error {
	discountsJSON, err := json.Marshal(inv.Discounts)
	if err != nil {
		return fmt.Errorf("marshal invoice discounts: %w", err)
	}
	err = r.q(ctx).CreateInvoice(ctx, gen.CreateInvoiceParams{
		ID:                  pgutil.UUIDToPgtype(inv.ID),
		SubscriptionID:      pgutil.UUIDToPgtype(inv.SubscriptionID),
		UserID:              pgutil.UUIDToPgtype(inv.UserID),
		SubtotalAmount:      inv.Subtotal.Amount,
		TotalDiscountAmount: inv.TotalDiscount.Amount,
		TotalAmount:         inv.Total.Amount,
		Currency:            string(inv.Total.Currency),
		PricingReason:       inv.PricingReason,
		Discounts:           discountsJSON,
		Status:              string(inv.Status),
		PaidAt:              pgutil.OptTimeToPgtype(inv.PaidAt),
		CreatedAt:           pgutil.TimeToPgtype(inv.CreatedAt),
		UpdatedAt:           pgutil.TimeToPgtype(inv.UpdatedAt),
	})
	if err != nil {
		return pgutil.MapErr(err, "create invoice", billing.ErrInvoiceNotFound)
	}

	for _, item := range inv.LineItems {
		if err := r.createLineItem(ctx, inv.ID, item); err != nil {
			return fmt.Errorf("create line item %q for invoice: %w", item.Description, err)
		}
	}
	return nil
}

func (r *InvoiceRepository) createLineItem(ctx context.Context, invoiceID string, item vo.LineItem) error {
	err := r.q(ctx).CreateInvoiceLineItem(ctx, gen.CreateInvoiceLineItemParams{
		InvoiceID:   pgutil.UUIDToPgtype(invoiceID),
		Description: item.Description,
		ItemType:    string(item.Type),
		Amount:      item.Amount.Amount,
		Currency:    string(item.Amount.Currency),
		Quantity:    int32(item.Quantity),
	})
	return pgutil.MapErr(err, "create invoice line item", billing.ErrInvoiceNotFound)
}

func (r *InvoiceRepository) Update(ctx context.Context, inv *aggregate.Invoice) error {
	discountsJSON, err := json.Marshal(inv.Discounts)
	if err != nil {
		return fmt.Errorf("marshal invoice discounts: %w", err)
	}
	err = r.q(ctx).UpdateInvoice(ctx, gen.UpdateInvoiceParams{
		ID:                  pgutil.UUIDToPgtype(inv.ID),
		Status:              string(inv.Status),
		PaidAt:              pgutil.OptTimeToPgtype(inv.PaidAt),
		SubtotalAmount:      inv.Subtotal.Amount,
		TotalDiscountAmount: inv.TotalDiscount.Amount,
		TotalAmount:         inv.Total.Amount,
		PricingReason:       inv.PricingReason,
		Discounts:           discountsJSON,
	})
	return pgutil.MapErr(err, "update invoice", billing.ErrInvoiceNotFound)
}

// getAllInvoicesCursorFirstPageSQL returns the first page of invoices
// ordered by (created_at DESC, id DESC). Used when no cursor is provided.
const getAllInvoicesCursorFirstPageSQL = `
SELECT id, subscription_id, user_id, subtotal_amount, total_discount_amount,
       total_amount, currency, pricing_reason, discounts, status, paid_at, created_at, updated_at
FROM billing.invoices
ORDER BY created_at DESC, id DESC
LIMIT $1`

// getAllInvoicesCursorNextPageSQL returns subsequent pages using cursor-based
// pagination keyed on (created_at DESC, id DESC). Bypasses sqlc because the
// conditional WHERE clause for optional cursor parameters is not supported.
const getAllInvoicesCursorNextPageSQL = `
SELECT id, subscription_id, user_id, subtotal_amount, total_discount_amount,
       total_amount, currency, pricing_reason, discounts, status, paid_at, created_at, updated_at
FROM billing.invoices
WHERE (created_at, id) < ($2, $3)
ORDER BY created_at DESC, id DESC
LIMIT $1`

// GetAllCursor returns invoices using cursor-based pagination keyed on
// (created_at DESC, id DESC). Nil cursor returns the first page.
func (r *InvoiceRepository) GetAllCursor(ctx context.Context, params billing.CursorParams) ([]*aggregate.Invoice, error) {
	db := DBFromContext(ctx, r.pool)

	if params.CreatedAt == nil || params.ID == nil {
		rows, err := db.Query(ctx, getAllInvoicesCursorFirstPageSQL, int32(params.Limit))
		if err != nil {
			return nil, pgutil.MapErr(err, "get all invoices cursor first page", billing.ErrInvoiceNotFound)
		}
		defer rows.Close()
		return scanInvoiceRows(rows)
	}

	rows, err := db.Query(ctx, getAllInvoicesCursorNextPageSQL,
		int32(params.Limit),
		pgutil.TimeToPgtype(*params.CreatedAt),
		pgutil.UUIDToPgtype(*params.ID),
	)
	if err != nil {
		return nil, pgutil.MapErr(err, "get all invoices cursor next page", billing.ErrInvoiceNotFound)
	}
	defer rows.Close()
	return scanInvoiceRows(rows)
}

// scanInvoiceRows scans pgx rows into a slice of domain invoices.
// Shared by GetAllCursor to avoid duplicating the scan loop.
func scanInvoiceRows(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
},
) ([]*aggregate.Invoice, error) {
	var invoices []*aggregate.Invoice
	for rows.Next() {
		var f invFields
		if err := rows.Scan(
			&f.ID, &f.SubscriptionID, &f.UserID, &f.SubtotalAmount, &f.TotalDiscountAmount,
			&f.TotalAmount, &f.Currency, &f.PricingReason, &f.Discounts, &f.Status,
			&f.PaidAt, &f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, pgutil.MapErr(err, "scan invoice cursor row", billing.ErrInvoiceNotFound)
		}
		inv, err := invFieldsToDomain(f)
		if err != nil {
			return nil, fmt.Errorf("scan invoice cursor row: %w", err)
		}
		invoices = append(invoices, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, pgutil.MapErr(err, "iterate invoice cursor rows", billing.ErrInvoiceNotFound)
	}
	return invoices, nil
}

var _ billing.InvoiceRepository = (*InvoiceRepository)(nil)

// ---------------------------------------------------------------------------
// FamilyRepository
// ---------------------------------------------------------------------------

// FamilyRepository implements billing.FamilyRepository backed by PostgreSQL.
type FamilyRepository struct {
	pool *pgxpool.Pool
}

// NewFamilyRepository returns a new FamilyRepository using the given pool.
func NewFamilyRepository(pool *pgxpool.Pool) *FamilyRepository {
	return &FamilyRepository{pool: pool}
}

// q returns a *gen.Queries backed by the active transaction (if any) or the
// pool. This ensures all methods transparently participate in RunInTx and
// respect RLS tenant scoping.
func (r *FamilyRepository) q(ctx context.Context) *gen.Queries {
	return gen.New(DBFromContext(ctx, r.pool))
}

func familyGroupRowToDomain(row gen.BillingFamilyGroup) *aggregate.FamilyGroup {
	return &aggregate.FamilyGroup{
		ID:         pgutil.PgtypeToUUID(row.ID),
		OwnerID:    pgutil.PgtypeToUUID(row.OwnerID),
		MaxMembers: int(row.MaxMembers),
		CreatedAt:  pgutil.PgtypeToTime(row.CreatedAt),
		UpdatedAt:  pgutil.PgtypeToTime(row.UpdatedAt),
	}
}

func familyMemberRowToDomain(row gen.BillingFamilyMember) aggregate.FamilyMember {
	return aggregate.FamilyMember{
		UserID:   pgutil.PgtypeToUUID(row.UserID),
		Role:     aggregate.MemberRole(row.Role),
		Nickname: pgutil.DerefStr(row.Nickname),
		JoinedAt: pgutil.PgtypeToTime(row.JoinedAt),
	}
}

func (r *FamilyRepository) loadMembers(ctx context.Context, fg *aggregate.FamilyGroup) error {
	rows, err := r.q(ctx).GetFamilyMembersByGroupID(ctx, pgutil.UUIDToPgtype(fg.ID))
	if err != nil {
		return pgutil.MapErr(err, "get family members", billing.ErrFamilyGroupNotFound)
	}
	members := make([]aggregate.FamilyMember, len(rows))
	for i, row := range rows {
		members[i] = familyMemberRowToDomain(row)
	}
	fg.Members = members
	return nil
}

func (r *FamilyRepository) GetByID(ctx context.Context, id string) (*aggregate.FamilyGroup, error) {
	row, err := r.q(ctx).GetFamilyGroupByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get family group by id", billing.ErrFamilyGroupNotFound)
	}
	fg := familyGroupRowToDomain(row)
	if err := r.loadMembers(ctx, fg); err != nil {
		return nil, err
	}
	return fg, nil
}

func (r *FamilyRepository) GetByOwnerID(ctx context.Context, ownerID string) (*aggregate.FamilyGroup, error) {
	row, err := r.q(ctx).GetFamilyGroupByOwnerID(ctx, pgutil.UUIDToPgtype(ownerID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get family group by owner id", billing.ErrFamilyGroupNotFound)
	}
	fg := familyGroupRowToDomain(row)
	if err := r.loadMembers(ctx, fg); err != nil {
		return nil, err
	}
	return fg, nil
}

func (r *FamilyRepository) GetByOwnerIDForUpdate(ctx context.Context, ownerID string) (*aggregate.FamilyGroup, error) {
	row, err := r.q(ctx).GetFamilyGroupByOwnerIDForUpdate(ctx, pgutil.UUIDToPgtype(ownerID))
	if err != nil {
		return nil, pgutil.MapErr(err, "get family group by owner id for update", billing.ErrFamilyGroupNotFound)
	}
	fg := familyGroupRowToDomain(row)
	if err := r.loadMembers(ctx, fg); err != nil {
		return nil, err
	}
	return fg, nil
}

func (r *FamilyRepository) Create(ctx context.Context, fg *aggregate.FamilyGroup) error {
	err := r.q(ctx).CreateFamilyGroup(ctx, gen.CreateFamilyGroupParams{
		ID:         pgutil.UUIDToPgtype(fg.ID),
		OwnerID:    pgutil.UUIDToPgtype(fg.OwnerID),
		MaxMembers: int32(fg.MaxMembers),
		CreatedAt:  pgutil.TimeToPgtype(fg.CreatedAt),
		UpdatedAt:  pgutil.TimeToPgtype(fg.UpdatedAt),
	})
	if err != nil {
		return pgutil.MapErr(err, "create family group", billing.ErrFamilyGroupNotFound)
	}

	for _, member := range fg.Members {
		if err := r.createMember(ctx, fg.ID, member); err != nil {
			return fmt.Errorf("create member %s for family group: %w", member.UserID, err)
		}
	}
	return nil
}

func (r *FamilyRepository) createMember(ctx context.Context, groupID string, member aggregate.FamilyMember) error {
	err := r.q(ctx).CreateFamilyMember(ctx, gen.CreateFamilyMemberParams{
		FamilyGroupID: pgutil.UUIDToPgtype(groupID),
		UserID:        pgutil.UUIDToPgtype(member.UserID),
		Role:          string(member.Role),
		Nickname:      pgutil.StrPtrOrNil(member.Nickname),
		JoinedAt:      pgutil.TimeToPgtype(member.JoinedAt),
	})
	return pgutil.MapErr(err, "create family member", billing.ErrFamilyGroupNotFound)
}

func (r *FamilyRepository) upsertMember(ctx context.Context, groupID string, member aggregate.FamilyMember) error {
	err := r.q(ctx).UpsertFamilyMember(ctx, gen.UpsertFamilyMemberParams{
		FamilyGroupID: pgutil.UUIDToPgtype(groupID),
		UserID:        pgutil.UUIDToPgtype(member.UserID),
		Role:          string(member.Role),
		Nickname:      pgutil.StrPtrOrNil(member.Nickname),
		JoinedAt:      pgutil.TimeToPgtype(member.JoinedAt),
	})
	return pgutil.MapErr(err, "upsert family member", billing.ErrFamilyGroupNotFound)
}

func (r *FamilyRepository) Update(ctx context.Context, fg *aggregate.FamilyGroup) error {
	err := r.q(ctx).UpdateFamilyGroup(ctx, gen.UpdateFamilyGroupParams{
		ID:         pgutil.UUIDToPgtype(fg.ID),
		MaxMembers: int32(fg.MaxMembers),
	})
	if err != nil {
		return pgutil.MapErr(err, "update family group", billing.ErrFamilyGroupNotFound)
	}

	// Upsert each member, then prune members removed from the list.
	// This avoids DELETE ALL + RE-INSERT which breaks FK references
	// and loses joined_at timestamps for unchanged members.
	retainUserIDs := make([]pgtype.UUID, 0, len(fg.Members))
	for _, member := range fg.Members {
		if err := r.upsertMember(ctx, fg.ID, member); err != nil {
			return fmt.Errorf("upsert member %s for family group: %w", member.UserID, err)
		}
		retainUserIDs = append(retainUserIDs, pgutil.UUIDToPgtype(member.UserID))
	}

	if err := r.q(ctx).DeleteRemovedFamilyMembers(ctx, gen.DeleteRemovedFamilyMembersParams{
		FamilyGroupID: pgutil.UUIDToPgtype(fg.ID),
		UserIds:       retainUserIDs,
	}); err != nil {
		return pgutil.MapErr(err, "delete removed family members", billing.ErrFamilyGroupNotFound)
	}
	return nil
}

func (r *FamilyRepository) Delete(ctx context.Context, id string) error {
	err := r.q(ctx).DeleteFamilyGroup(ctx, pgutil.UUIDToPgtype(id))
	return pgutil.MapErr(err, "delete family group", billing.ErrFamilyGroupNotFound)
}

var _ billing.FamilyRepository = (*FamilyRepository)(nil)
