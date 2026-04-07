package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/sony/gobreaker/v2"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/observability"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// OutboxBacklogPollInterval is how often the relay queries the outbox table for
// the count of unpublished events and updates the OutboxUnpublishedCount gauge.
//
// Suggested Prometheus alert:
//
//	- alert: OutboxBacklogGrowing
//	  expr: delta(platform_outbox_unpublished_count[5m]) > 100
//	  for: 10m
const OutboxBacklogPollInterval = 30 * time.Second

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

	// OutboxRelayBatchSize is the base number of events fetched per tick.
	// During burst load the relay dynamically scales this up to
	// OutboxRelayMaxBatchSize.
	OutboxRelayBatchSize = 100

	// OutboxRelayMaxBatchSize is the upper bound for dynamic batch scaling.
	// When consecutive batches are full, the relay doubles the batch size
	// each tick up to this cap.
	OutboxRelayMaxBatchSize = 500

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
	metrics     *observability.Metrics
	workerCount int
	natsBreaker *gobreaker.CircuitBreaker[struct{}]
}

// NATS message metadata header keys for outbox relay.
const (
	// HeaderOutboxSequence carries the outbox sequence number for ordering
	// verification and debugging on the consumer side.
	HeaderOutboxSequence = "X-Outbox-Sequence"

	// HeaderEventType carries the domain event type so consumers can inspect
	// it without deserialising the payload.
	HeaderEventType = "X-Event-Type"

	// HeaderTraceParent carries the W3C traceparent value extracted from the
	// domain event payload. Consumers use this header (via OTel's
	// TextMapPropagator) to link their processing spans to the originating
	// business operation's trace. The header name follows the W3C Trace
	// Context specification.
	HeaderTraceParent = "traceparent"
)

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
// metrics may be nil; metric recording is skipped when nil (safe for tests).
func NewOutboxRelay(
	outbox *postgres.OutboxRepository,
	publisher *EventPublisher,
	txRunner txmanager.Runner,
	logger *slog.Logger,
	workerCount int,
	metrics *observability.Metrics,
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
		metrics:     metrics,
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

	// Single backlog polling goroutine to keep the OutboxUnpublishedCount gauge current.
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.pollOutboxBacklog(ctx)
	}()

	wg.Wait()
}

// runWorker is the per-worker relay loop. It executes an immediate catch-up
// pass, then enters a timer-based loop with exponential backoff on idle.
//
// Adaptive batch sizing: when the last batch returned exactly the current
// batch size (indicating more events are likely waiting), the worker doubles
// the batch size for the next poll, up to OutboxRelayMaxBatchSize. When the
// batch returns fewer events or is empty, the batch size resets to
// OutboxRelayBatchSize.
func (r *OutboxRelay) runWorker(ctx context.Context, workerID int) {
	workerLabel := strconv.Itoa(workerID)
	logger := r.logger.With(slog.Int("worker_id", workerID))
	logger.Info("outbox relay worker started")

	currentBatchSize := OutboxRelayBatchSize

	// Immediate catch-up for events stuck from a prior crash.
	r.relay(ctx, logger, workerLabel, currentBatchSize)

	currentInterval := OutboxRelayBaseInterval
	relayTimer := time.NewTimer(currentInterval)
	defer relayTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("outbox relay worker stopping")
			return
		case <-relayTimer.C:
			published := r.relay(ctx, logger, workerLabel, currentBatchSize)

			switch {
			case published == currentBatchSize:
				// Batch was full — likely more events waiting. Scale up.
				currentBatchSize = min(currentBatchSize*OutboxRelayBackoffMultiplier, OutboxRelayMaxBatchSize)
				currentInterval = OutboxRelayBaseInterval
			case published == 0:
				// Empty — backoff interval, reset batch size.
				currentBatchSize = OutboxRelayBatchSize
				currentInterval *= OutboxRelayBackoffMultiplier
				if currentInterval > OutboxRelayMaxInterval {
					currentInterval = OutboxRelayMaxInterval
				}
			default:
				// Partial batch — reset batch size and interval.
				currentBatchSize = OutboxRelayBatchSize
				currentInterval = OutboxRelayBaseInterval
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
// to a specific worker. The workerLabel is the stringified worker ID used as
// a Prometheus label value. The batchSize parameter controls how many events
// are fetched — the caller (runWorker) adjusts this dynamically based on
// whether previous batches were full.
func (r *OutboxRelay) relay(ctx context.Context, logger *slog.Logger, workerLabel string, batchSize int) int {
	// Skip DB polling entirely when NATS is known to be unreachable.
	if r.natsBreaker.State() == gobreaker.StateOpen {
		logger.Debug("outbox relay: NATS circuit open, skipping poll")
		return 0
	}

	start := time.Now()
	var published int
	var fetchedCount int

	err := r.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		events, err := r.outbox.GetUnpublished(txCtx, batchSize)
		if err != nil {
			return fmt.Errorf("get unpublished: %w", err)
		}

		fetchedCount = len(events)

		if fetchedCount == 0 {
			if r.metrics != nil {
				r.metrics.OutboxRelayEmptyPolls.Inc()
			}
			return nil
		}

		// Publish to NATS, collect successfully published event references.
		publishedIDs := make([]string, 0, len(events))
		publishedTimes := make([]time.Time, 0, len(events))

		// Track events that failed to publish so we can increment their
		// relay_attempts within this transaction. This is safe because the
		// row locks (FOR UPDATE SKIP LOCKED) are held until commit.
		type failedEvent struct {
			event postgres.OutboxEvent
			err   error
		}
		var failedEvents []failedEvent

		for _, event := range events {
			// Build the Watermill message with outbox metadata headers.
			// The event ID is used as the Watermill message UUID so
			// JetStream can deduplicate retransmissions via Nats-Msg-Id.
			msg := message.NewMessage(event.ID, event.Payload)
			msg.Metadata.Set(HeaderOutboxSequence, strconv.FormatInt(event.SequenceNumber, 10))
			msg.Metadata.Set(HeaderEventType, event.EventType)

			// Propagate W3C traceparent from the event payload to NATS
			// message metadata so consumers can reconstruct the trace
			// context without parsing the full payload first.
			if tp := extractTraceParent(event.Payload); tp != "" {
				msg.Metadata.Set(HeaderTraceParent, tp)
			}

			// Wrap publish in the circuit breaker. On success the breaker
			// records a success; on failure it records a failure and will
			// trip open after relayCBConsecutiveFailures consecutive errors.
			_, cbErr := r.natsBreaker.Execute(func() (struct{}, error) {
				return struct{}{}, r.publisher.PublishRaw(event.EventType, msg)
			})
			if cbErr != nil {
				logger.Warn("outbox relay: failed to publish event",
					slog.String("event_id", event.ID),
					slog.String("event_type", event.EventType),
					slog.Int("attempt", event.RelayAttempts+1),
					slog.Any("error", cbErr),
				)
				if r.metrics != nil {
					r.metrics.OutboxRelayPublishErrors.Inc()
				}
				failedEvents = append(failedEvents, failedEvent{event: event, err: cbErr})
				continue
			}
			publishedIDs = append(publishedIDs, event.ID)
			publishedTimes = append(publishedTimes, event.CreatedAt)
		}

		// Increment relay_attempts for each failed event. When the new
		// count reaches MaxRelayAttempts, the SQL sets relay_failed=true
		// and the event is permanently excluded from future relay polls.
		for _, fe := range failedEvents {
			if incErr := r.outbox.IncrementRelayAttempts(txCtx, fe.event.ID, fe.event.CreatedAt); incErr != nil {
				logger.Error("outbox relay: failed to increment relay attempts",
					slog.String("event_id", fe.event.ID),
					slog.Any("error", incErr),
				)
				continue
			}

			newAttempts := fe.event.RelayAttempts + 1
			if newAttempts >= postgres.MaxRelayAttempts {
				logger.Error("outbox relay: event dead-lettered after max attempts",
					slog.String("event_id", fe.event.ID),
					slog.String("event_type", fe.event.EventType),
					slog.Int("attempts", newAttempts),
					slog.Any("last_error", fe.err),
				)
				if r.metrics != nil {
					r.metrics.OutboxRelayDeadEvents.Inc()
				}
			}
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

	// Record metrics after the transaction completes (success or failure).
	if r.metrics != nil && fetchedCount > 0 {
		r.metrics.OutboxRelayBatchSize.WithLabelValues(workerLabel).Observe(float64(fetchedCount))
		r.metrics.OutboxRelayBatchLatency.WithLabelValues(workerLabel).Observe(time.Since(start).Seconds())
	}

	return published
}

// pollOutboxBacklog periodically queries the outbox table for the count of
// unpublished events and updates the OutboxUnpublishedCount Prometheus gauge.
// This runs as a single background goroutine started by Run and exits when
// the context is cancelled. If the metrics dependency is nil, the goroutine
// returns immediately to avoid panics in test environments.
func (r *OutboxRelay) pollOutboxBacklog(ctx context.Context) {
	if r.metrics == nil {
		return
	}

	ticker := time.NewTicker(OutboxBacklogPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := r.outbox.CountUnpublished(ctx)
			if err != nil {
				r.logger.Warn("outbox backlog poll failed", slog.Any("error", err))
				continue
			}
			r.metrics.OutboxUnpublishedCount.Set(float64(count))
		}
	}
}

// cleanup purges published events from the CURRENT partition that are past
// retention. This complements PartitionManager, which drops entire expired
// partitions (instant O(1) via DETACH + DROP).
//
// Division of responsibility:
//   - OutboxRelay.cleanup: intra-partition cleanup for the active quarter.
//     Uses DELETE WHERE published=true AND published_at < retention AND
//     created_at < retention (partition pruning hint). Generates dead tuples
//     but only for recently-published events, keeping vacuum scope small.
//   - PartitionManager.cleanup: drops entire past-retention partitions that
//     contain no unpublished events and pass the sequence safety check.
//     Instant, no vacuum.
//
// Both run concurrently: relay every hour, PartitionManager every 24 hours.
// Removing either would leave a gap: relay-only means old partitions persist
// forever; PM-only means the active partition accumulates published events
// for the entire quarter before the first drop opportunity.
func (r *OutboxRelay) cleanup(ctx context.Context) {
	if err := r.outbox.DeleteOld(ctx, OutboxRetentionPeriod); err != nil {
		r.logger.Error("outbox relay: failed to clean up old events",
			slog.Any("error", err),
		)
	}
}

// traceParentEnvelope is a minimal struct for extracting only the trace_parent
// field from a serialised domain event without fully unmarshalling the payload.
type traceParentEnvelope struct {
	TraceParent string `json:"trace_parent"`
}

// extractTraceParent extracts the trace_parent field from a JSON-encoded domain
// event payload. Returns an empty string if the field is missing, empty, or the
// payload is not valid JSON. This avoids a full unmarshal of the event payload
// in the relay hot path.
func extractTraceParent(payload []byte) string {
	var envelope traceParentEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return ""
	}
	return envelope.TraceParent
}
