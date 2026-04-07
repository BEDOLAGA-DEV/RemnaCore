package postgres

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
)

// ErrOutboxBackpressure is returned by CheckBackpressure when the outbox
// unpublished event count exceeds the critical threshold. This is an
// informational signal — callers should NOT reject events (that would violate
// transactional guarantees). Instead, use it for circuit-breaking observability
// at the publisher side.
var ErrOutboxBackpressure = errors.New("outbox: critical backpressure threshold exceeded")

// MaxRelayAttempts is the maximum number of times the relay will attempt to
// publish a single outbox event before marking it as permanently failed
// (relay_failed=true). After this threshold, the event is excluded from relay
// polling and must be inspected manually.
const MaxRelayAttempts = 10

// OutboxEvent represents a single event stored in the transactional outbox
// table, awaiting relay to the message broker.
type OutboxEvent struct {
	ID             string
	EventType      string
	Payload        []byte
	CreatedAt      time.Time
	SequenceNumber int64
	RelayAttempts  int
}

// OutboxRepository provides access to the public.outbox table. It is used by
// the OutboxPublisher to store events within the same DB transaction as
// business logic, and by the OutboxRelay to poll for unpublished events.
//
// When called within a TxManager.RunInTx context, all methods participate in
// the active transaction via DBFromContext. This is critical for the relay:
// GetUnpublished uses FOR UPDATE SKIP LOCKED, which requires a transaction to
// hold the row locks until publish + mark-published complete.
type OutboxRepository struct {
	pool  *pgxpool.Pool
	clock clock.Clock
}

// NewOutboxRepository returns a new OutboxRepository using the given pool.
func NewOutboxRepository(pool *pgxpool.Pool, clk clock.Clock) *OutboxRepository {
	return &OutboxRepository{pool: pool, clock: clk}
}

// queries returns a *gen.Queries backed by the active transaction (if any) or
// the pool. This ensures all methods transparently participate in RunInTx.
func (r *OutboxRepository) queries(ctx context.Context) *gen.Queries {
	return gen.New(DBFromContext(ctx, r.pool))
}

// Store saves an event to the outbox table. This should be called within
// the same database transaction as the business logic that produced the event.
func (r *OutboxRepository) Store(ctx context.Context, eventType string, payload []byte, entityID string) error {
	err := r.queries(ctx).InsertOutboxEvent(ctx, gen.InsertOutboxEventParams{
		EventType: eventType,
		Payload:   payload,
		EntityID:  entityID,
	})
	if err != nil {
		return fmt.Errorf("store outbox event: %w", err)
	}
	return nil
}

// GetUnpublished returns up to limit unpublished events ordered by created_at
// (oldest first). The underlying query uses FOR UPDATE SKIP LOCKED, so this
// MUST be called within a transaction (via TxManager.RunInTx) to hold the row
// locks. Multiple relay instances safely skip rows locked by each other.
func (r *OutboxRepository) GetUnpublished(ctx context.Context, limit int) ([]OutboxEvent, error) {
	rows, err := r.queries(ctx).GetUnpublishedOutboxEvents(ctx, int32(limit))
	if err != nil {
		return nil, fmt.Errorf("get unpublished outbox events: %w", err)
	}

	events := make([]OutboxEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, rowToOutboxEvent(row))
	}
	return events, nil
}

// GetUnpublishedForWorker returns up to limit unpublished events that are
// assigned to the given workerID via entity_id hash affinity. The query uses
// abs(hashtext(entity_id)) % workerCount = workerID, ensuring events for the
// same entity always go to the same worker and preserving per-entity sequence
// ordering. Like GetUnpublished, this MUST be called within a transaction.
func (r *OutboxRepository) GetUnpublishedForWorker(ctx context.Context, limit int, workerID, workerCount int) ([]OutboxEvent, error) {
	rows, err := r.queries(ctx).GetUnpublishedForWorker(ctx, gen.GetUnpublishedForWorkerParams{
		Limit:       int32(limit),
		WorkerCount: int32(workerCount),
		WorkerID:    int32(workerID),
	})
	if err != nil {
		return nil, fmt.Errorf("get unpublished outbox events for worker %d/%d: %w", workerID, workerCount, err)
	}

	events := make([]OutboxEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, workerRowToOutboxEvent(row))
	}
	return events, nil
}

// Deprecated: Use MarkPublishedBatch instead — it uses a single PG18 MERGE
// statement instead of N individual UPDATEs.
//
// MarkPublished sets the published flag and timestamp for a single event.
// Both id and createdAt are required for partition pruning on the
// range-partitioned outbox table.
func (r *OutboxRepository) MarkPublished(ctx context.Context, id string, createdAt time.Time) error {
	err := r.queries(ctx).MarkOutboxEventPublished(ctx, gen.MarkOutboxEventPublishedParams{
		ID:        pgutil.UUIDToPgtype(id),
		CreatedAt: pgutil.TimeToPgtype(createdAt),
	})
	if err != nil {
		return fmt.Errorf("mark outbox event published: %w", err)
	}
	return nil
}

// IncrementRelayAttempts atomically increments the relay_attempts counter for
// a single event. When the new count reaches MaxRelayAttempts, the SQL
// statement sets relay_failed=true, permanently excluding the event from
// relay polling. Both id and createdAt are required for partition pruning.
func (r *OutboxRepository) IncrementRelayAttempts(ctx context.Context, id string, createdAt time.Time) error {
	err := r.queries(ctx).IncrementRelayAttempts(ctx, gen.IncrementRelayAttemptsParams{
		ID:            pgutil.UUIDToPgtype(id),
		RelayAttempts: int32(MaxRelayAttempts),
		CreatedAt:     pgutil.TimeToPgtype(createdAt),
	})
	if err != nil {
		return fmt.Errorf("increment relay attempts: %w", err)
	}
	return nil
}

// markPublishedBatchSQL uses PG18 MERGE ... RETURNING to atomically mark
// multiple outbox events as published in a single statement. The unnest
// approach passes parallel arrays of IDs and created_at timestamps.
//
// $3 (minCreatedAt) and $4 (maxCreatedAtExcl) provide a date range hint
// so the PG planner can prune partitions at plan time. Without these
// bounds, unnest produces runtime values that prevent partition pruning,
// causing all partitions to be scanned. The WHEN MATCHED clause restricts
// the update to rows within [minCreatedAt, maxCreatedAtExcl), which the
// planner uses for static partition elimination.
const markPublishedBatchSQL = `
MERGE INTO public.outbox AS o
USING (
    SELECT unnest($1::uuid[]) AS id, unnest($2::timestamptz[]) AS created_at
) AS input ON o.id = input.id AND o.created_at = input.created_at
WHEN MATCHED AND o.published = false
    AND o.created_at >= $3 AND o.created_at < $4
THEN
    UPDATE SET published = true, published_at = now()
RETURNING o.id`

// partitionPruningPadding is added to the max created_at timestamp to form an
// exclusive upper bound. This ensures the max value itself is included in the
// half-open range [min, max + padding) used for partition pruning.
const partitionPruningPadding = time.Second

// MarkPublishedBatch atomically marks multiple events as published using a
// single PG18 MERGE statement. Returns the number of rows actually updated.
// Both ids and createdAts must have the same length; each pair (ids[i],
// createdAts[i]) identifies one event. Uses MERGE ... RETURNING for
// partition-pruned batch updates in a single round-trip.
//
// The method computes the min/max of createdAts and passes them as additional
// parameters ($3, $4) so the PG planner can eliminate partitions that do not
// overlap the batch's time range. Without these hints, unnest produces runtime
// values that prevent static partition pruning.
func (r *OutboxRepository) MarkPublishedBatch(ctx context.Context, ids []string, createdAts []time.Time) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	if len(ids) != len(createdAts) {
		return 0, fmt.Errorf("mark published batch: ids and createdAts length mismatch: %d vs %d", len(ids), len(createdAts))
	}

	db := DBFromContext(ctx, r.pool)

	// pgx v5 natively maps []pgtype.UUID to uuid[] and []time.Time to
	// timestamptz[], so we convert IDs to pgtype and pass times directly.
	pgIDs := pgutil.StringsToPgtypeUUIDs(ids)

	// Compute date range for partition pruning.
	minTime, maxTime := timeRange(createdAts)
	maxTimeExcl := maxTime.Add(partitionPruningPadding)

	rows, err := db.Query(ctx, markPublishedBatchSQL,
		pgIDs, createdAts,
		pgutil.TimeToPgtype(minTime), pgutil.TimeToPgtype(maxTimeExcl),
	)
	if err != nil {
		return 0, fmt.Errorf("merge mark published batch: %w", err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var id pgtype.UUID
		if err := rows.Scan(&id); err != nil {
			return count, fmt.Errorf("scan merge result: %w", err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return count, fmt.Errorf("merge mark published batch rows: %w", err)
	}

	return count, nil
}

// timeRange returns the minimum and maximum values from a time slice.
// Returns zero-value times for an empty slice (defense-in-depth; the caller
// already guards with len(ids)==0).
func timeRange(ts []time.Time) (min, max time.Time) {
	if len(ts) == 0 {
		return time.Time{}, time.Time{}
	}
	min, max = ts[0], ts[0]
	for _, t := range ts[1:] {
		if t.Before(min) {
			min = t
		}
		if t.After(max) {
			max = t
		}
	}
	return min, max
}

// DeleteOld removes published events whose published_at is older than the
// given duration AND whose created_at is at or after minCreatedAt. This
// restricts cleanup to the current quarter's partition — older partitions
// are dropped entirely by PartitionManager.DetachAndDrop (O(1), no vacuum).
func (r *OutboxRepository) DeleteOld(ctx context.Context, olderThan time.Duration, minCreatedAt time.Time) error {
	cutoff := pgutil.TimeToPgtype(r.clock.Now().Add(-olderThan))
	err := r.queries(ctx).DeleteOldPublishedOutboxEvents(ctx, gen.DeleteOldPublishedOutboxEventsParams{
		PublishedAt: cutoff,
		CreatedAt:   pgutil.TimeToPgtype(minCreatedAt),
	})
	if err != nil {
		return fmt.Errorf("delete old outbox events: %w", err)
	}
	return nil
}

// outboxPartitionPattern validates partition names to prevent SQL injection.
// Only allows names like outbox_2026_q1, outbox_2027_q3, outbox_default.
var outboxPartitionPattern = regexp.MustCompile(`^outbox_(\d{4}_q[1-4]|default)$`)

// monthsPerQuarter is the number of months in a calendar quarter.
const monthsPerQuarter = 3

// quartersPerYear is the number of quarters in a year.
const quartersPerYear = 4

// EnsurePartitions creates outbox partitions for the next N quarters from the
// given start time. Existing partitions are silently skipped (IF NOT EXISTS).
// This should be called on startup to prevent events falling into the default
// partition after pre-created partitions from migrations are exhausted.
func (r *OutboxRepository) EnsurePartitions(ctx context.Context, from time.Time, quartersAhead int) error {
	for i := range quartersAhead {
		year, quarter := quarterOf(from, i)
		start := quarterStart(year, quarter)
		end := nextQuarterStart(year, quarter)
		name := fmt.Sprintf("outbox_%d_q%d", year, quarter)

		if !outboxPartitionPattern.MatchString(name) {
			return fmt.Errorf("invalid partition name: %s", name)
		}

		sql := fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS %s PARTITION OF public.outbox FOR VALUES FROM ('%s') TO ('%s')",
			name,
			start.Format(time.DateOnly),
			end.Format(time.DateOnly),
		)
		if _, err := r.pool.Exec(ctx, sql); err != nil {
			return fmt.Errorf("create partition %s: %w", name, err)
		}
	}
	return nil
}

// quarterOf returns the year and quarter (1-4) for the quarter that is offset
// quarters ahead of the given time.
func quarterOf(t time.Time, offset int) (year int, quarter int) {
	// Current quarter: (month-1)/3 + 1 gives 1-4.
	currentQuarter := (int(t.Month()) - 1) / monthsPerQuarter
	totalQuarters := currentQuarter + offset
	year = t.Year() + totalQuarters/quartersPerYear
	quarter = totalQuarters%quartersPerYear + 1
	return year, quarter
}

// quarterStart returns the first day of the given quarter.
func quarterStart(year, quarter int) time.Time {
	month := time.Month((quarter-1)*monthsPerQuarter + 1)
	return time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
}

// nextQuarterStart returns the first day of the quarter following the given one.
func nextQuarterStart(year, quarter int) time.Time {
	if quarter == quartersPerYear {
		return quarterStart(year+1, 1)
	}
	return quarterStart(year, quarter+1)
}

// DetachAndDropPartition detaches and drops an old outbox partition by name.
// This is the PG18 partition-based cleanup path — instant, no vacuum bloat.
// Example: DetachAndDropPartition(ctx, "outbox_2026_q1") after the quarter ends
// and all events in it are published and past the retention period.
// The partition name is validated against a strict pattern to prevent SQL injection.
func (r *OutboxRepository) DetachAndDropPartition(ctx context.Context, partitionName string) error {
	if !outboxPartitionPattern.MatchString(partitionName) {
		return fmt.Errorf("invalid partition name: %s", partitionName)
	}
	// CONCURRENTLY avoids blocking concurrent DML on the parent table.
	detachSQL := fmt.Sprintf("ALTER TABLE public.outbox DETACH PARTITION %s CONCURRENTLY", partitionName)
	if _, err := r.pool.Exec(ctx, detachSQL); err != nil {
		return fmt.Errorf("detach partition %s: %w", partitionName, err)
	}
	dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS %s", partitionName)
	if _, err := r.pool.Exec(ctx, dropSQL); err != nil {
		return fmt.Errorf("drop partition %s: %w", partitionName, err)
	}
	return nil
}

// countUnpublishedSQL returns the number of active unpublished events across
// all partitions. Events marked as relay_failed are excluded because they are
// no longer being actively retried. This is intentionally a simple count(*)
// without FOR UPDATE — it is used by the OutboxRelay backlog gauge and does
// not need transactional isolation.
const countUnpublishedSQL = `SELECT count(*) FROM public.outbox WHERE published = false AND relay_failed = false`

// CountUnpublished returns the number of events in the outbox that have not
// yet been published. Used for the outbox backlog Prometheus gauge.
func (r *OutboxRepository) CountUnpublished(ctx context.Context) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, countUnpublishedSQL).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count unpublished outbox events: %w", err)
	}
	return count, nil
}

// outboxCriticalThreshold is the number of unpublished events that indicates
// the outbox is severely backed up. CheckBackpressure returns
// ErrOutboxBackpressure when the count exceeds this value.
const outboxCriticalThreshold int64 = 50000

// CheckBackpressure returns ErrOutboxBackpressure if the outbox unpublished
// event count exceeds the critical threshold. Callers can use this for
// circuit-breaking observability at the publisher side.
//
// This is informational only — events should never be rejected based on this
// signal, as that would violate transactional guarantees. If the count query
// fails, nil is returned (fail-open: do not block publishing on count query
// failure).
func (r *OutboxRepository) CheckBackpressure(ctx context.Context) error {
	count, err := r.CountUnpublished(ctx)
	if err != nil {
		return nil // Fail open — don't block publishing on count query failure.
	}
	if count > outboxCriticalThreshold {
		return ErrOutboxBackpressure
	}
	return nil
}

// GetLastPublishedSequence returns the highest sequence_number among published
// outbox events. Returns 0 if no published events exist. Used by the outbox
// reconciliation check to compare PG outbox state with JetStream stream state.
func (r *OutboxRepository) GetLastPublishedSequence(ctx context.Context) (int64, error) {
	seq, err := r.queries(ctx).GetLastPublishedOutboxSequence(ctx)
	if err != nil {
		return 0, fmt.Errorf("get last published outbox sequence: %w", err)
	}
	return seq, nil
}

// rowToOutboxEvent converts a sqlc-generated row to the adapter-level OutboxEvent.
func rowToOutboxEvent(row gen.GetUnpublishedOutboxEventsRow) OutboxEvent {
	var seqNum int64
	if row.SequenceNumber != nil {
		seqNum = *row.SequenceNumber
	}
	return OutboxEvent{
		ID:             pgutil.PgtypeToUUID(row.ID),
		EventType:      row.EventType,
		Payload:        row.Payload,
		CreatedAt:      pgutil.PgtypeToTime(row.CreatedAt),
		SequenceNumber: seqNum,
		RelayAttempts:  int(row.RelayAttempts),
	}
}

// workerRowToOutboxEvent converts a GetUnpublishedForWorkerRow to OutboxEvent.
// The row shape is identical to GetUnpublishedOutboxEventsRow but sqlc generates
// a separate type for each query.
func workerRowToOutboxEvent(row gen.GetUnpublishedForWorkerRow) OutboxEvent {
	var seqNum int64
	if row.SequenceNumber != nil {
		seqNum = *row.SequenceNumber
	}
	return OutboxEvent{
		ID:             pgutil.PgtypeToUUID(row.ID),
		EventType:      row.EventType,
		Payload:        row.Payload,
		CreatedAt:      pgutil.PgtypeToTime(row.CreatedAt),
		SequenceNumber: seqNum,
		RelayAttempts:  int(row.RelayAttempts),
	}
}
