package postgres

import (
	"context"
	"fmt"
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

// MarkPublished sets the published flag and timestamp for the given event ID.
// The relay calls this after successfully publishing to the message broker.
// When called within RunInTx, uses the same transaction that holds the row lock.
func (r *OutboxRepository) MarkPublished(ctx context.Context, id string) error {
	err := r.queries(ctx).MarkOutboxEventPublished(ctx, pgutil.UUIDToPgtype(id))
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

// rowToOutboxEvent converts a sqlc-generated row to the adapter-level OutboxEvent.
func rowToOutboxEvent(row gen.GetUnpublishedOutboxEventsRow) OutboxEvent {
	return OutboxEvent{
		ID:        pgutil.PgtypeToUUID(row.ID),
		EventType: row.EventType,
		Payload:   row.Payload,
		CreatedAt: pgutil.PgtypeToTime(row.CreatedAt),
	}
}
