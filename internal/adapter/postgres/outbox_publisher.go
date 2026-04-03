package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
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
func (p *OutboxPublisher) Publish(ctx context.Context, event domainevent.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal outbox event: %w", err)
	}

	db := DBFromContext(ctx, p.pool)
	queries := gen.New(db)

	if err := queries.InsertOutboxEvent(ctx, gen.InsertOutboxEventParams{
		EventType: string(event.Type),
		Payload:   payload,
	}); err != nil {
		return fmt.Errorf("outbox publish: %w", err)
	}

	return nil
}

// compile-time interface check
var _ domainevent.Publisher = (*OutboxPublisher)(nil)
