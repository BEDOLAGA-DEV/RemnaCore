package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// OutboxPublisher implements domainevent.Publisher by writing events to the
// transactional outbox table instead of directly to the message broker. The
// OutboxRelay asynchronously forwards stored events to NATS, guaranteeing
// at-least-once delivery even when the broker is unavailable.
type OutboxPublisher struct {
	repo *OutboxRepository
}

// NewOutboxPublisher creates an OutboxPublisher backed by the given
// OutboxRepository.
func NewOutboxPublisher(repo *OutboxRepository) *OutboxPublisher {
	return &OutboxPublisher{repo: repo}
}

// Publish serializes the domain event to JSON and stores it in the outbox
// table. The actual message broker publish happens asynchronously via the
// OutboxRelay.
func (p *OutboxPublisher) Publish(ctx context.Context, event domainevent.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal outbox event: %w", err)
	}
	return p.repo.Store(ctx, string(event.Type), payload)
}

// compile-time interface check
var _ domainevent.Publisher = (*OutboxPublisher)(nil)
