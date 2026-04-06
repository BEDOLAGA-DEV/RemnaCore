package nats

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/sony/gobreaker/v2"

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
// MarkPublishedBatch fails, the transaction rolls back and the event is
// re-published on the next tick. Consumers must be idempotent.
//
// Circuit breaker: the relay wraps NATS publishes in a circuit breaker. When
// NATS is unreachable, the breaker opens after relayCBConsecutiveFailures
// consecutive failures and the relay skips DB polling entirely until the
// breaker transitions to half-open after relayCBTimeout. This prevents
// wasteful DB locks and log spam during NATS outages.
//
// JetStream deduplication: each message is published with the outbox event ID
// as the Watermill message UUID. Because TrackMsgId is enabled on the
// publisher, Watermill sets the Nats-Msg-Id header to this UUID, enabling
// server-side deduplication of retransmissions after transaction rollbacks.
type OutboxRelay struct {
	outbox      *postgres.OutboxRepository
	publisher   *EventPublisher
	txRunner    txmanager.Runner
	logger      *slog.Logger
	workerCount int
	natsBreaker *gobreaker.CircuitBreaker[struct{}]
}

const (
	// MinOutboxRelayWorkers is the lower bound for worker count to ensure at
	// least one goroutine always processes the outbox.
	MinOutboxRelayWorkers = 1

	// MaxOutboxRelayWorkers caps the number of parallel relay goroutines to
	// prevent exhausting the database connection pool.
	MaxOutboxRelayWorkers = 16

	// relayCBName is the circuit breaker instance name for the outbox relay.
	relayCBName = "outbox-relay-nats"

	// relayCBMaxRequests is the number of probe requests allowed in the
	// half-open state to test whether NATS has recovered.
	relayCBMaxRequests = 1

	// relayCBTimeout is the duration the circuit stays open before
	// transitioning to half-open.
	relayCBTimeout = 10 * time.Second

	// relayCBConsecutiveFailures is the number of consecutive NATS publish
	// failures required to trip the breaker from closed to open.
	relayCBConsecutiveFailures = 5
)

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
	if workerCount > MaxOutboxRelayWorkers {
		workerCount = MaxOutboxRelayWorkers
	}

	cb := gobreaker.NewCircuitBreaker[struct{}](gobreaker.Settings{
		Name:        relayCBName,
		MaxRequests: relayCBMaxRequests,
		Interval:    0, // Never reset counts in closed state; only consecutive failures matter.
		Timeout:     relayCBTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= relayCBConsecutiveFailures
		},
	})

	return &OutboxRelay{
		outbox:      outbox,
		publisher:   publisher,
		txRunner:    txRunner,
		logger:      logger,
		workerCount: workerCount,
		natsBreaker: cb,
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
// FOR UPDATE SKIP LOCKED row locks), publishes each to NATS, and marks all
// successfully published events as published in a single MERGE statement.
// The transaction ensures that locked rows are invisible to other relay
// instances.
//
// Circuit breaker: if the NATS breaker is open, relay returns 0 immediately
// without polling the database. This prevents wasteful DB locks during NATS
// outages.
//
// JetStream dedup: each event is published with its outbox ID as the
// Watermill message UUID. Because TrackMsgId is enabled, the NATS publisher
// sets Nats-Msg-Id to this UUID, enabling server-side deduplication.
//
// If a NATS publish fails for a specific event, that event is skipped and
// will be retried on the next tick (the row lock is released on commit).
// If MarkPublishedBatch fails, the entire transaction is rolled back; events
// that were already published to NATS will be deduplicated by JetStream.
//
// The logger parameter carries the worker ID so log lines can be correlated
// to a specific worker.
func (r *OutboxRelay) relay(ctx context.Context, logger *slog.Logger) int {
	// Skip DB polling entirely when NATS is known to be unreachable.
	if r.natsBreaker.State() == gobreaker.StateOpen {
		logger.Debug("outbox relay: NATS circuit open, skipping poll")
		return 0
	}

	var published int

	err := r.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		events, err := r.outbox.GetUnpublished(txCtx, OutboxRelayBatchSize)
		if err != nil {
			return fmt.Errorf("get unpublished: %w", err)
		}

		if len(events) == 0 {
			return nil
		}

		// Publish to NATS, collect successfully published event references.
		publishedIDs := make([]string, 0, len(events))
		publishedTimes := make([]time.Time, 0, len(events))

		for _, event := range events {
			// Wrap publish in the circuit breaker. On success the breaker
			// records a success; on failure it records a failure and will
			// trip open after relayCBConsecutiveFailures consecutive errors.
			// The event ID is used as the Watermill message UUID so
			// JetStream can deduplicate retransmissions via Nats-Msg-Id.
			_, cbErr := r.natsBreaker.Execute(func() (struct{}, error) {
				return struct{}{}, r.publisher.PublishWithID(txCtx, event.ID, event.EventType, event.Payload)
			})
			if cbErr != nil {
				logger.Warn("outbox relay: failed to publish event, will retry",
					slog.String("event_id", event.ID),
					slog.String("event_type", event.EventType),
					slog.Any("error", cbErr),
				)
				// Skip this event — row lock released on commit, retry next tick.
				continue
			}
			publishedIDs = append(publishedIDs, event.ID)
			publishedTimes = append(publishedTimes, event.CreatedAt)
		}

		if len(publishedIDs) == 0 {
			return nil
		}

		// Single MERGE statement marks all published events at once.
		count, err := r.outbox.MarkPublishedBatch(txCtx, publishedIDs, publishedTimes)
		if err != nil {
			return fmt.Errorf("mark published batch: %w", err)
		}

		published = count

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
