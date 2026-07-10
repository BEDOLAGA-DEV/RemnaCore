package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
)

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
	db := DBFromContext(ctx, r.pool)
	row := db.QueryRow(ctx, getSubscriptionByIDForUpdateGuardedSQL, pgutil.UUIDToPgtype(id))

	var raw gen.GetSubscriptionByIDForUpdateRow
	err := row.Scan(
		&raw.ID, &raw.UserID, &raw.PlanID, &raw.Status, &raw.PeriodStart, &raw.PeriodEnd, &raw.PeriodInterval,
		&raw.AddonIds, &raw.AssignedTo, &raw.PendingPlanID, &raw.PendingOriginalPlanID, &raw.PendingRequestedAt,
		&raw.CancelledAt, &raw.PausedAt, &raw.CreatedAt, &raw.UpdatedAt,
	)
	if err != nil {
		return nil, pgutil.MapErr(err, "get subscription by id for update", billing.ErrSubscriptionNotFound)
	}
	return subRowToDomain(raw), nil
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
	if err != nil {
		if pgutil.IsUniqueViolation(err) {
			return fmt.Errorf("create subscription: %w", billing.ErrSubscriptionAlreadyExists)
		}
		return fmt.Errorf("create subscription: %w", err)
	}
	return nil
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

// getSubscriptionByIDForUpdateGuardedSQL is the by-id lock path with an explicit
// tenant predicate (spec §5.3 belt-and-suspenders behind the RLS GUC). Must be
// called inside RunInTx so the GUC is set; the predicate also blocks a by-id
// lookup that would otherwise bypass the user_id chain. The predicate matches the
// RLS USING clause exactly (sentinel '*' OR tenant match; no IS NULL fail-open
// branch), so a NULL-tenant row is visible ONLY under the platform sentinel.
// Column order matches
// gen.GetSubscriptionByIDForUpdateRow so subRowToDomain can convert the scan.
const getSubscriptionByIDForUpdateGuardedSQL = `
SELECT id, user_id, plan_id, status, period_start, period_end, period_interval,
       addon_ids, assigned_to, pending_plan_id, pending_original_plan_id, pending_requested_at,
       cancelled_at, paused_at, created_at, updated_at
FROM billing.subscriptions
WHERE id = $1
  AND ` + rlsTenantGuard + `
FOR UPDATE
`

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
