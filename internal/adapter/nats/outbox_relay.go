package nats

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
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
// Row locking: each relay batch runs inside a database transaction with
// FOR UPDATE SKIP LOCKED, ensuring multiple relay instances never process
// the same rows concurrently.
//
// Startup catch-up: Run executes one immediate relay pass before entering
// the ticker loop, so events stuck from a prior crash are forwarded without
// waiting for the first tick.
//
// Delivery guarantee: at-least-once. If NATS publish succeeds but
// MarkPublished fails, the transaction rolls back and the event is
// re-published on the next tick. Consumers must be idempotent.
type OutboxRelay struct {
	outbox    *postgres.OutboxRepository
	publisher *EventPublisher
	txRunner  txmanager.Runner
	logger    *slog.Logger
}

// NewOutboxRelay creates an OutboxRelay with the given dependencies.
func NewOutboxRelay(
	outbox *postgres.OutboxRepository,
	publisher *EventPublisher,
	txRunner txmanager.Runner,
	logger *slog.Logger,
) *OutboxRelay {
	return &OutboxRelay{
		outbox:    outbox,
		publisher: publisher,
		txRunner:  txRunner,
		logger:    logger,
	}
}

// Run starts the relay loop that polls the outbox table and publishes events
// to NATS. It also periodically purges old published events. Run blocks until
// the context is cancelled.
//
// An immediate relay pass is executed on startup to catch up on any events
// that were written but not yet relayed before a previous shutdown or crash.
func (r *OutboxRelay) Run(ctx context.Context) {
	// Immediate catch-up for events stuck from a prior crash.
	r.relay(ctx)

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

// relay fetches a batch of unpublished events within a transaction (holding
// FOR UPDATE SKIP LOCKED row locks), publishes each to NATS, and marks
// successfully published events as published. The transaction ensures that
// locked rows are invisible to other relay instances.
//
// If a NATS publish fails for a specific event, that event is skipped and
// will be retried on the next tick (the row lock is released on commit).
// If MarkPublished fails, the entire transaction is rolled back; events that
// were already published to NATS will be re-delivered (at-least-once).
func (r *OutboxRelay) relay(ctx context.Context) {
	err := r.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		events, err := r.outbox.GetUnpublished(txCtx, OutboxRelayBatchSize)
		if err != nil {
			return fmt.Errorf("get unpublished: %w", err)
		}

		if len(events) == 0 {
			return nil
		}

		published := 0
		for _, event := range events {
			if err := r.publisher.Publish(ctx, event.EventType, event.Payload); err != nil {
				r.logger.Warn("outbox relay: failed to publish event, will retry",
					slog.String("event_id", event.ID),
					slog.String("event_type", event.EventType),
					slog.Any("error", err),
				)
				// Skip this event — row lock released on commit, retry next tick.
				continue
			}

			if err := r.outbox.MarkPublished(txCtx, event.ID); err != nil {
				return fmt.Errorf("mark published event %s: %w", event.ID, err)
			}

			published++
		}

		if published > 0 {
			r.logger.Info("outbox relay: batch completed",
				slog.Int("published", published),
				slog.Int("total", len(events)),
			)
		}

		return nil
	})

	if err != nil {
		r.logger.Error("outbox relay: batch failed",
			slog.Any("error", err),
		)
	}
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
