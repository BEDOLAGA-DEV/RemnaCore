package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tracing"
)

// insertOutboxBatchSQL inserts multiple outbox events in a single statement
// using UNNEST over parallel arrays. All events with an ID use this path;
// events without an ID (backward compat) are inserted individually via
// the existing sqlc-generated InsertOutboxEvent query.
//
// The UNNEST approach ensures all events are inserted in one round-trip within
// the same transaction, preventing partial-publish when one insert fails.
const insertOutboxBatchSQL = `
INSERT INTO public.outbox (id, event_type, payload, entity_id, trace_parent)
SELECT unnest($1::uuid[]), unnest($2::text[]), unnest($3::jsonb[]), unnest($4::text[]), unnest($5::text[])`

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

// Publish serializes a single domain event to JSON and stores it in the outbox
// table. If the context carries a transaction (set by TxManager.RunInTx), the
// insert uses that transaction, ensuring the outbox write is atomic with the
// business logic. Otherwise the insert goes directly to the pool.
//
// When the event carries an ID (all events created via constructors since the
// UUIDv7 migration), the outbox row uses that ID as its primary key. This
// ensures the outbox row ID, NATS Msg-Id, and consumer idempotency key are all
// the same domain event ID. Events without an ID (backward compat) fall back
// to the DB-generated gen_random_uuid() default.
//
// For multiple events, prefer PublishBatch which inserts all rows in a single
// round-trip, preventing partial-publish on error.
func (p *OutboxPublisher) Publish(ctx context.Context, event domainevent.Event) error {
	return p.PublishBatch(ctx, []domainevent.Event{event})
}

// PublishBatch serializes all domain events to JSON and inserts them into the
// outbox table in a single round-trip. Events with an ID are batch-inserted
// via INSERT ... SELECT UNNEST; events without an ID (backward compat) fall
// back to individual inserts. If the context carries a transaction, all
// inserts participate in that transaction, guaranteeing atomicity.
//
// This prevents the partial-publish problem where the Nth individual Publish
// fails after N-1 rows are already committed. PublishAll delegates here.
func (p *OutboxPublisher) PublishBatch(ctx context.Context, events []domainevent.Event) error {
	if len(events) == 0 {
		return nil
	}

	// Inject W3C traceparent once from the active span.
	traceParent := tracing.FormatTraceParent(ctx)

	db := DBFromContext(ctx, p.pool)

	// Partition events into those with IDs (batch-insertable) and those
	// without (backward compat, individual insert).
	var (
		batchIDs         []pgtype.UUID
		batchEventTypes  []string
		batchPayloads    [][]byte
		batchEntityIDs   []string
		batchTraceParent []string
	)

	for i := range events {
		if events[i].TraceParent == "" {
			events[i].TraceParent = traceParent
		}

		payload, err := json.Marshal(events[i])
		if err != nil {
			return fmt.Errorf("marshal outbox event %s: %w", events[i].Type, err)
		}

		if events[i].ID != "" {
			batchIDs = append(batchIDs, pgutil.UUIDToPgtype(events[i].ID))
			batchEventTypes = append(batchEventTypes, string(events[i].Type))
			batchPayloads = append(batchPayloads, payload)
			batchEntityIDs = append(batchEntityIDs, events[i].EntityID)
			batchTraceParent = append(batchTraceParent, events[i].TraceParent)
		} else {
			// Backward compat: old events without ID use DB-generated default.
			queries := gen.New(db)
			if err := queries.InsertOutboxEvent(ctx, gen.InsertOutboxEventParams{
				EventType:   string(events[i].Type),
				Payload:     payload,
				EntityID:    events[i].EntityID,
				TraceParent: events[i].TraceParent,
			}); err != nil {
				return fmt.Errorf("outbox publish (no-id): %w", err)
			}
		}
	}

	if len(batchIDs) > 0 {
		if _, err := db.Exec(ctx, insertOutboxBatchSQL,
			batchIDs, batchEventTypes, batchPayloads, batchEntityIDs, batchTraceParent,
		); err != nil {
			return fmt.Errorf("outbox publish batch: %w", err)
		}
	}

	return nil
}

// compile-time interface check
var _ domainevent.Publisher = (*OutboxPublisher)(nil)
