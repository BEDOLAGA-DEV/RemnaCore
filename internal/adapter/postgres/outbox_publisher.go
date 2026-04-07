package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tracing"
)

// OutboxPublisher implements domainevent.Publisher by writing events to the
// transactional outbox table instead of directly to the message broker. When
// called within a TxManager.RunInTx context, it participates in the same
// database transaction as the business logic, guaranteeing atomicity. The
// OutboxRelay asynchronously forwards stored events to NATS, providing
// at-least-once delivery even when the broker is unavailable.
type OutboxPublisher struct {
	pool *pgxpool.Pool
}

// NewOutboxPublisher creates an OutboxPublisher backed by the given pool.
// The publisher uses DBFromContext to detect an active transaction in the
// context; if none exists it falls back to the pool.
func NewOutboxPublisher(pool *pgxpool.Pool) *OutboxPublisher {
	return &OutboxPublisher{pool: pool}
}

// Publish serializes the domain event to JSON and stores it in the outbox
// table. If the context carries a transaction (set by TxManager.RunInTx), the
// insert uses that transaction, ensuring the outbox write is atomic with the
// business logic. Otherwise the insert goes directly to the pool.
//
// When the event carries an ID (all events created via constructors since the
// UUIDv7 migration), the outbox row uses that ID as its primary key. This
// ensures the outbox row ID, NATS Msg-Id, and consumer idempotency key are all
// the same domain event ID. Events without an ID (backward compat) fall back
// to the DB-generated gen_random_uuid() default.
func (p *OutboxPublisher) Publish(ctx context.Context, event domainevent.Event) error {
	// Inject W3C traceparent from the active span so that downstream consumers
	// (via the outbox relay) can correlate their processing spans with the
	// originating business operation.
	if event.TraceParent == "" {
		event.TraceParent = tracing.FormatTraceParent(ctx)
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal outbox event: %w", err)
	}

	db := DBFromContext(ctx, p.pool)
	queries := gen.New(db)

	if event.ID != "" {
		if err := queries.InsertOutboxEventWithID(ctx, gen.InsertOutboxEventWithIDParams{
			ID:          pgutil.UUIDToPgtype(event.ID),
			EventType:   string(event.Type),
			Payload:     payload,
			EntityID:    event.EntityID,
			TraceParent: event.TraceParent,
		}); err != nil {
			return fmt.Errorf("outbox publish: %w", err)
		}
	} else {
		// Backward compat: old events without ID use DB-generated default.
		if err := queries.InsertOutboxEvent(ctx, gen.InsertOutboxEventParams{
			EventType:   string(event.Type),
			Payload:     payload,
			EntityID:    event.EntityID,
			TraceParent: event.TraceParent,
		}); err != nil {
			return fmt.Errorf("outbox publish: %w", err)
		}
	}

	return nil
}

// compile-time interface check
var _ domainevent.Publisher = (*OutboxPublisher)(nil)
