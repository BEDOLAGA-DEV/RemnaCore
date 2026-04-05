package nats

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// Outbox relay constants control polling frequency, batch size, and retention.
const (
	// OutboxRelayBaseInterval is the starting poll interval. The relay
	// doubles this on each empty batch up to OutboxRelayMaxInterval, and
	// resets to base on any non-empty batch.
	OutboxRelayBaseInterval = 1 * time.Second

	// OutboxRelayMaxInterval caps exponential backoff so idle polling never
	// exceeds this frequency.
	OutboxRelayMaxInterval = 30 * time.Second

	// OutboxRelayBackoffMultiplier doubles the interval on each empty poll.
	OutboxRelayBackoffMultiplier = 2

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
	outbox      *postgres.OutboxRepository
	publisher   *EventPublisher
	txRunner    txmanager.Runner
	logger      *slog.Logger
	workerCount int
}

// MinOutboxRelayWorkers is the lower bound for worker count to ensure at
// least one goroutine always processes the outbox.
const MinOutboxRelayWorkers = 1

// NewOutboxRelay creates an OutboxRelay with the given dependencies.
// workerCount controls the number of parallel relay goroutines; values
// below MinOutboxRelayWorkers are clamped to MinOutboxRelayWorkers.
func NewOutboxRelay(
	outbox *postgres.OutboxRepository,
	publisher *EventPublisher,
	txRunner txmanager.Runner,
	logger *slog.Logger,
	workerCount int,
) *OutboxRelay {
	if workerCount < MinOutboxRelayWorkers {
		workerCount = MinOutboxRelayWorkers
	}
	return &OutboxRelay{
		outbox:      outbox,
		publisher:   publisher,
		txRunner:    txRunner,
		logger:      logger,
		workerCount: workerCount,
	}
}

// Run spawns workerCount relay goroutines plus a single cleanup goroutine.
// Each worker independently polls the outbox table with FOR UPDATE SKIP
// LOCKED, so rows are never processed by more than one worker. Run blocks
// until the context is cancelled and all goroutines have exited.
//
// An immediate relay pass is executed by each worker on startup to catch up
// on any events that were written but not yet relayed before a previous
// shutdown or crash.
func (r *OutboxRelay) Run(ctx context.Context) {
	var wg sync.WaitGroup

	// Spawn relay workers — each polls independently with FOR UPDATE SKIP LOCKED.
	for i := range r.workerCount {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			r.runWorker(ctx, workerID)
		}(i)
	}

	// Single cleanup goroutine (no need to parallelise cleanup).
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.runCleanup(ctx)
	}()

	wg.Wait()
}

// runWorker is the per-worker relay loop. It executes an immediate catch-up
// pass, then enters a timer-based loop with exponential backoff on idle.
func (r *OutboxRelay) runWorker(ctx context.Context, workerID int) {
	logger := r.logger.With(slog.Int("worker_id", workerID))
	logger.Info("outbox relay worker started")

	// Immediate catch-up for events stuck from a prior crash.
	r.relay(ctx, logger)

	currentInterval := OutboxRelayBaseInterval
	relayTimer := time.NewTimer(currentInterval)
	defer relayTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("outbox relay worker stopping")
			return
		case <-relayTimer.C:
			published := r.relay(ctx, logger)
			if published > 0 {
				// Reset to base interval when events were found.
				currentInterval = OutboxRelayBaseInterval
			} else {
				// Exponential backoff on empty batch, capped.
				currentInterval *= OutboxRelayBackoffMultiplier
				if currentInterval > OutboxRelayMaxInterval {
					currentInterval = OutboxRelayMaxInterval
				}
			}
			relayTimer.Reset(currentInterval)
		}
	}
}

// runCleanup periodically purges old published events. It runs as a single
// goroutine regardless of worker count.
func (r *OutboxRelay) runCleanup(ctx context.Context) {
	cleanupTicker := time.NewTicker(OutboxCleanupInterval)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
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
//
// The logger parameter carries the worker ID so log lines can be correlated
// to a specific worker.
func (r *OutboxRelay) relay(ctx context.Context, logger *slog.Logger) int {
	var published int

	err := r.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		events, err := r.outbox.GetUnpublished(txCtx, OutboxRelayBatchSize)
		if err != nil {
			return fmt.Errorf("get unpublished: %w", err)
		}

		if len(events) == 0 {
			return nil
		}

		for _, event := range events {
			if err := r.publisher.Publish(ctx, event.EventType, event.Payload); err != nil {
				logger.Warn("outbox relay: failed to publish event, will retry",
					slog.String("event_id", event.ID),
					slog.String("event_type", event.EventType),
					slog.Any("error", err),
				)
				// Skip this event — row lock released on commit, retry next tick.
				continue
			}

			if err := r.outbox.MarkPublished(txCtx, event.ID, event.CreatedAt); err != nil {
				return fmt.Errorf("mark published event %s: %w", event.ID, err)
			}

			published++
		}

		if published > 0 {
			logger.Info("outbox relay: batch completed",
				slog.Int("published", published),
				slog.Int("total", len(events)),
			)
		}

		return nil
	})

	if err != nil {
		logger.Error("outbox relay: batch failed",
			slog.Any("error", err),
		)
	}

	return published
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
