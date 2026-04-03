package nats

import (
	"context"
	"log/slog"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
)

// Outbox relay constants control polling frequency, batch size, and retention.
const (
	// OutboxRelayInterval is how often the relay polls for unpublished events.
	OutboxRelayInterval = 1 * time.Second

	// OutboxRelayBatchSize is the maximum number of events fetched per tick.
	OutboxRelayBatchSize = 100

	// OutboxCleanupInterval is how often the relay purges old published events.
	OutboxCleanupInterval = 1 * time.Hour

	// OutboxRetentionPeriod is how long published events are kept before deletion.
	OutboxRetentionPeriod = 7 * 24 * time.Hour
)

// OutboxRelay polls the transactional outbox table for unpublished domain
// events and forwards them to NATS via the EventPublisher. It runs as a
// background goroutine managed by the Fx lifecycle.
//
// The relay is idempotent: re-publishing the same event is safe because NATS
// JetStream deduplicates by message ID, and consumers are designed for
// at-least-once delivery.
type OutboxRelay struct {
	outbox    *postgres.OutboxRepository
	publisher *EventPublisher
	logger    *slog.Logger
}

// NewOutboxRelay creates an OutboxRelay with the given dependencies.
func NewOutboxRelay(
	outbox *postgres.OutboxRepository,
	publisher *EventPublisher,
	logger *slog.Logger,
) *OutboxRelay {
	return &OutboxRelay{
		outbox:    outbox,
		publisher: publisher,
		logger:    logger,
	}
}

// Run starts the relay loop that polls the outbox table and publishes events
// to NATS. It also periodically purges old published events. Run blocks until
// the context is cancelled.
func (r *OutboxRelay) Run(ctx context.Context) {
	relayTicker := time.NewTicker(OutboxRelayInterval)
	cleanupTicker := time.NewTicker(OutboxCleanupInterval)
	defer relayTicker.Stop()
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-relayTicker.C:
			r.relay(ctx)
		case <-cleanupTicker.C:
			r.cleanup(ctx)
		}
	}
}

// relay fetches a batch of unpublished events, publishes each to NATS, and
// marks successfully published events as published. Failed publishes are
// left unpublished so they retry on the next tick.
func (r *OutboxRelay) relay(ctx context.Context) {
	events, err := r.outbox.GetUnpublished(ctx, OutboxRelayBatchSize)
	if err != nil {
		r.logger.Error("outbox relay: failed to fetch unpublished events",
			slog.Any("error", err),
		)
		return
	}

	if len(events) == 0 {
		return
	}

	published := 0
	for _, event := range events {
		if err := r.publisher.Publish(ctx, event.EventType, event.Payload); err != nil {
			r.logger.Warn("outbox relay: failed to publish event, will retry",
				slog.String("event_id", event.ID),
				slog.String("event_type", event.EventType),
				slog.Any("error", err),
			)
			// Do NOT mark as published — retry on next tick.
			continue
		}

		if err := r.outbox.MarkPublished(ctx, event.ID); err != nil {
			r.logger.Error("outbox relay: failed to mark event as published",
				slog.String("event_id", event.ID),
				slog.Any("error", err),
			)
			// Event was published to NATS but not marked — it will be
			// re-published on next tick. Consumers must be idempotent.
			continue
		}

		published++
	}

	r.logger.Info("outbox relay: batch completed",
		slog.Int("published", published),
		slog.Int("total", len(events)),
	)
}

// cleanup removes published events older than the retention period to prevent
// unbounded table growth.
func (r *OutboxRelay) cleanup(ctx context.Context) {
	if err := r.outbox.DeleteOld(ctx, OutboxRetentionPeriod); err != nil {
		r.logger.Error("outbox relay: failed to clean up old events",
			slog.Any("error", err),
		)
	}
}
