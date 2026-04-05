package postgres

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
)

// OutboxEvent represents a single event stored in the transactional outbox
// table, awaiting relay to the message broker.
type OutboxEvent struct {
	ID        string
	EventType string
	Payload   []byte
	CreatedAt time.Time
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
func (r *OutboxRepository) Store(ctx context.Context, eventType string, payload []byte) error {
	err := r.queries(ctx).InsertOutboxEvent(ctx, gen.InsertOutboxEventParams{
		EventType: eventType,
		Payload:   payload,
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

// MarkPublished sets the published flag and timestamp for the given event.
// The relay calls this after successfully publishing to the message broker.
// When called within RunInTx, uses the same transaction that holds the row lock.
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

// DeleteOld removes published events whose published_at is older than the
// given duration. This keeps the outbox table from growing unbounded.
func (r *OutboxRepository) DeleteOld(ctx context.Context, olderThan time.Duration) error {
	cutoff := pgutil.TimeToPgtype(r.clock.Now().Add(-olderThan))
	err := r.queries(ctx).DeleteOldPublishedOutboxEvents(ctx, cutoff)
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

// rowToOutboxEvent converts a sqlc-generated row to the adapter-level OutboxEvent.
func rowToOutboxEvent(row gen.GetUnpublishedOutboxEventsRow) OutboxEvent {
	return OutboxEvent{
		ID:        pgutil.PgtypeToUUID(row.ID),
		EventType: row.EventType,
		Payload:   row.Payload,
		CreatedAt: pgutil.PgtypeToTime(row.CreatedAt),
	}
}
