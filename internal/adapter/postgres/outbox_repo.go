package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
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
type OutboxRepository struct {
	pool    *pgxpool.Pool
	queries *gen.Queries
}

// NewOutboxRepository returns a new OutboxRepository using the given pool.
func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{
		pool:    pool,
		queries: gen.New(pool),
	}
}

// Store saves an event to the outbox table. This should be called within
// the same database transaction as the business logic that produced the event.
func (r *OutboxRepository) Store(ctx context.Context, eventType string, payload []byte) error {
	err := r.queries.InsertOutboxEvent(ctx, gen.InsertOutboxEventParams{
		EventType: eventType,
		Payload:   payload,
	})
	if err != nil {
		return fmt.Errorf("store outbox event: %w", err)
	}
	return nil
}

// GetUnpublished returns up to limit unpublished events ordered by created_at
// (oldest first). The relay calls this on each tick to find events that need
// forwarding to the message broker.
func (r *OutboxRepository) GetUnpublished(ctx context.Context, limit int) ([]OutboxEvent, error) {
	rows, err := r.queries.GetUnpublishedOutboxEvents(ctx, int32(limit))
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
func (r *OutboxRepository) MarkPublished(ctx context.Context, id string) error {
	err := r.queries.MarkOutboxEventPublished(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return fmt.Errorf("mark outbox event published: %w", err)
	}
	return nil
}

// DeleteOld removes published events whose published_at is older than the
// given duration. This keeps the outbox table from growing unbounded.
func (r *OutboxRepository) DeleteOld(ctx context.Context, olderThan time.Duration) error {
	cutoff := pgutil.TimeToPgtype(time.Now().Add(-olderThan))
	err := r.queries.DeleteOldPublishedOutboxEvents(ctx, cutoff)
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
