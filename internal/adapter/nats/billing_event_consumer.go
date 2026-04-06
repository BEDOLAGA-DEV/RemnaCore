package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	nc "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/propagation"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/observability"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tracing"
)

// errMissingSubscriptionID is returned when a billing event lacks the required
// subscription_id field in its data payload.
var errMissingSubscriptionID = errors.New("subscription_id missing from event data")

// Dead-letter queue and retry constants.
const (
	// MaxMessageRetries is the maximum number of processing attempts before a
	// message is routed to the dead-letter queue.
	MaxMessageRetries = 3

	// DLQSubjectPrefix is prepended to the original subject when publishing
	// failed messages to the dead-letter stream.
	DLQSubjectPrefix = "dlq."

	// retryKeyPrefix is prepended to the idempotency key to form a separate
	// key that tracks retry attempts in the database. Using a separate key
	// prevents conflicts with the idempotency key lifecycle (acquire/release).
	retryKeyPrefix = "retry:"
)

// SubscriptionEventHandler defines the contract for handling billing
// subscription lifecycle events. The MultiSubOrchestrator satisfies this
// interface, keeping the NATS adapter decoupled from the multisub domain.
//
// Plan data is passed as multisub.PlanSnapshot (an Anti-Corruption Layer type)
// so that the handler never depends on billing/aggregate types.
type SubscriptionEventHandler interface {
	OnSubscriptionActivated(
		ctx context.Context,
		subscriptionID string,
		platformUserID string,
		plan multisub.PlanSnapshot,
		addonIDs []string,
		familyMemberIDs []string,
	) error
	OnSubscriptionCancelled(ctx context.Context, subscriptionID string) error
	OnSubscriptionPaused(ctx context.Context, subscriptionID string) error
	OnSubscriptionResumed(ctx context.Context, subscriptionID string) error
}

// NOTE: Subscription and plan lookup interfaces are defined in the multisub
// domain as multisub.PlanProvider and multisub.SubscriptionProvider. The NATS
// adapter (BillingSubscriptionLookup) implements those domain ports.

// IdempotencyChecker provides event-level deduplication and retry tracking.
// The adapter layer owns this interface; the postgres.IdempotencyRepository
// satisfies it.
//
// Keys are the domain event ID (UUIDv7), unique per event instance. This
// deduplicates at the event level rather than the transport level (Watermill
// message UUID). Events without an ID (backward compat) use
// "{event_type}:{entity_id}" as the key.
type IdempotencyChecker interface {
	// TryAcquire returns true if the key is new, false if it was already seen.
	TryAcquire(ctx context.Context, key string) (bool, error)

	// Release removes an idempotency key so that a redelivered message can be
	// processed again. This MUST be called when event processing fails,
	// otherwise the redelivered message will be silently skipped as a
	// duplicate.
	Release(ctx context.Context, key string) error

	// IncrementRetry atomically increments and returns the retry count for
	// the given key. The count is persisted in the database so it survives
	// NATS redeliveries (Watermill metadata is lost across Nack cycles).
	IncrementRetry(ctx context.Context, key string) (int, error)
}

// Consumer lifecycle constants.
const (
	// entityLockTTL is the duration after which an idle entity lock is eligible for
	// eviction. This prevents unbounded growth of the entityLocks map.
	entityLockTTL = 10 * time.Minute

	// ConsumerDrainTimeout is the maximum duration Drain waits for in-flight
	// messages to finish processing before returning. This bounds the shutdown
	// delay when handlers are blocked on slow external calls (e.g. Remnawave).
	ConsumerDrainTimeout = 30 * time.Second

	// DLQDepthPollInterval is how often the consumer queries JetStream for
	// the current DLQ stream message count and updates the DLQDepth gauge.
	DLQDepthPollInterval = 30 * time.Second

	// DefaultMessageProcessingTimeout bounds the time for processing a single
	// event. If a handler exceeds this timeout (e.g., due to a hung DB query
	// or unresponsive Remnawave API), the context is cancelled, the handler
	// returns an error, and the message is Nack'd for retry.
	//
	// Default: 60 seconds — covers DB lookup + Remnawave API call + retry.
	// Configure via BillingEventConsumer.messageTimeout for environments
	// with slower upstream APIs.
	DefaultMessageProcessingTimeout = 60 * time.Second
)

// entityLock serialises event processing for a single entity (e.g. one
// subscription). This prevents race conditions when events like
// subscription.activated and subscription.cancelled arrive on different NATS
// subjects concurrently for the same aggregate.
type entityLock struct {
	mu       sync.Mutex
	lastUsed atomic.Int64 // UnixNano timestamp; atomic to avoid data races with evictor
}

// BillingEventConsumer subscribes to billing domain events on NATS and routes
// them to the SubscriptionEventHandler (MultiSubOrchestrator) for Remnawave
// provisioning and deprovisioning.
//
// Correctness guarantees:
//   - Per-event-instance idempotency: events are deduplicated by the domain
//     event ID (UUIDv7), not Watermill message UUID. This allows legitimate
//     repeated operations on the same aggregate to be processed independently.
//   - Per-entity ordering: events for the same entity are processed serially
//     via entityLocks, while different entities run concurrently.
//   - Retry + DLQ: failed messages are retried up to MaxMessageRetries times;
//     permanently failing messages are sent to the dead-letter queue.
type BillingEventConsumer struct {
	subscriber     *EventSubscriber
	handler        SubscriptionEventHandler
	plans          multisub.PlanProvider
	subs           multisub.SubscriptionProvider
	idempotency    IdempotencyChecker
	publisher      *EventPublisher
	schemaRegistry *domainevent.SchemaRegistry
	logger         *slog.Logger
	clock          clock.Clock
	metrics        *observability.Metrics
	natsConn       *nc.Conn
	messageTimeout time.Duration
	entityLocks    sync.Map // map[string]*entityLock — per-entity serialisation
	inflightWg     sync.WaitGroup
}

// NewBillingEventConsumer creates a BillingEventConsumer with the given
// dependencies. The publisher is used to route permanently failed messages to
// the dead-letter queue. Plan and subscription data are resolved through
// multisub domain ports (PlanProvider + SubscriptionProvider). The schema
// registry upcasts old event payloads to the latest version before processing.
// The NATS connection is used for DLQ depth polling via JetStream API.
func NewBillingEventConsumer(
	subscriber *EventSubscriber,
	handler SubscriptionEventHandler,
	plans multisub.PlanProvider,
	subs multisub.SubscriptionProvider,
	idempotency IdempotencyChecker,
	publisher *EventPublisher,
	schemaRegistry *domainevent.SchemaRegistry,
	logger *slog.Logger,
	clk clock.Clock,
	metrics *observability.Metrics,
	conn *nc.Conn,
) *BillingEventConsumer {
	return &BillingEventConsumer{
		subscriber:     subscriber,
		handler:        handler,
		plans:          plans,
		subs:           subs,
		idempotency:    idempotency,
		publisher:      publisher,
		schemaRegistry: schemaRegistry,
		logger:         logger,
		clock:          clk,
		metrics:        metrics,
		natsConn:       conn,
		messageTimeout: DefaultMessageProcessingTimeout,
	}
}

// getEntityLock returns (or creates) the mutex for the given entity ID. This
// ensures that concurrent events targeting the same aggregate are serialised.
func (c *BillingEventConsumer) getEntityLock(entityID string) *entityLock {
	val, _ := c.entityLocks.LoadOrStore(entityID, &entityLock{})
	lock := val.(*entityLock)
	lock.lastUsed.Store(c.clock.Now().UnixNano())
	return lock
}

// evictStaleLocks periodically removes entity locks that have not been used
// within entityLockTTL. This prevents unbounded memory growth in long-running
// processes. After each eviction pass, the EntityLocksActive gauge is updated
// with the number of remaining locks.
func (c *BillingEventConsumer) evictStaleLocks(ctx context.Context) {
	ticker := time.NewTicker(entityLockTTL)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := c.clock.Now().Add(-entityLockTTL).UnixNano()
			c.entityLocks.Range(func(key, value any) bool {
				lock := value.(*entityLock)
				if lock.lastUsed.Load() < cutoff {
					c.entityLocks.Delete(key)
				}
				return true
			})

			var remaining int
			c.entityLocks.Range(func(_, _ any) bool {
				remaining++
				return true
			})
			c.recordMetric(func(m *observability.Metrics) {
				m.EntityLocksActive.Set(float64(remaining))
			})
		}
	}
}

// Drain waits for all in-flight message handlers to complete, bounded by
// ConsumerDrainTimeout. Call Drain after cancelling the context passed to
// Start so that consumeLoop stops reading new messages while existing handlers
// finish. If the timeout expires, a warning is logged — this is a best-effort
// mechanism that avoids blocking shutdown indefinitely.
func (c *BillingEventConsumer) Drain() {
	done := make(chan struct{})
	go func() {
		c.inflightWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		c.logger.Info("billing event consumer: drain complete")
	case <-time.After(ConsumerDrainTimeout):
		c.logger.Warn("billing event consumer: drain timeout, some messages may still be in-flight")
	}
}

// pollDLQDepth periodically queries the JetStream DLQ stream for its current
// message count and updates the DLQDepth Prometheus gauge. This runs as a
// background goroutine started by Start and exits when the context is cancelled.
// If the NATS connection or metrics dependency is nil, the goroutine returns
// immediately to avoid panics in test environments.
func (c *BillingEventConsumer) pollDLQDepth(ctx context.Context) {
	if c.natsConn == nil || c.metrics == nil {
		return
	}

	js, err := jetstream.New(c.natsConn)
	if err != nil {
		c.logger.Warn("billing event consumer: failed to init JetStream for DLQ polling",
			slog.Any("error", err),
		)
		return
	}

	ticker := time.NewTicker(DLQDepthPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stream, streamErr := js.Stream(ctx, StreamDLQ)
			if streamErr != nil {
				c.logger.Debug("billing event consumer: failed to get DLQ stream info",
					slog.Any("error", streamErr),
				)
				continue
			}
			info, infoErr := stream.Info(ctx)
			if infoErr != nil {
				c.logger.Debug("billing event consumer: failed to get DLQ stream info",
					slog.Any("error", infoErr),
				)
				continue
			}
			c.metrics.DLQDepth.Set(float64(info.State.Msgs))
		}
	}
}

// billingSubscriptionSubjects returns the NATS subjects this consumer listens to.
func billingSubscriptionSubjects() []string {
	return []string{
		string(billing.EventSubActivated),
		string(billing.EventSubCancelled),
		string(billing.EventSubPaused),
		string(billing.EventSubResumed),
	}
}

// Start subscribes to billing subscription events and processes them in
// background goroutines. It returns immediately; the goroutines run until the
// context is cancelled.
func (c *BillingEventConsumer) Start(ctx context.Context) error {
	subscribed := 0
	for _, subject := range billingSubscriptionSubjects() {
		ch, err := c.subscriber.Subscribe(ctx, subject)
		if err != nil {
			c.logger.Warn("failed to subscribe to billing event, will retry on next restart",
				slog.String("subject", subject),
				slog.String("error", err.Error()),
			)
			continue
		}

		go c.consumeLoop(ctx, subject, ch)
		subscribed++
	}

	// Start background goroutine to evict stale entity locks.
	go c.evictStaleLocks(ctx)

	// Start DLQ depth polling goroutine to keep the DLQDepth gauge current.
	go c.pollDLQDepth(ctx)

	c.logger.Info("billing event consumer started",
		slog.Int("subscribed", subscribed),
		slog.Int("total", len(billingSubscriptionSubjects())),
	)
	return nil
}

// consumeLoop reads messages from a single subscription channel until the
// context is cancelled or the channel is closed. Each in-flight message is
// tracked by inflightWg so that Drain can wait for completion on shutdown.
// When the context is cancelled, the loop stops reading new messages but any
// message already being processed continues with a detached (Background)
// context so that in-flight work is not aborted mid-processing.
func (c *BillingEventConsumer) consumeLoop(ctx context.Context, subject string, ch <-chan *message.Message) {
	c.logger.Info("billing event consumer started", slog.String("subject", subject))

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			c.inflightWg.Add(1)
			// Use context.Background() so that a cancelled parent context does
			// not abort an in-flight handler mid-processing (e.g. during a
			// Remnawave provisioning call). The WaitGroup ensures Drain blocks
			// until this handler completes. The timeout prevents handlers from
			// blocking indefinitely on hung DB queries or unresponsive APIs.
			msgCtx, cancel := context.WithTimeout(context.Background(), c.messageTimeout)
			c.handleMessage(msgCtx, subject, msg)
			cancel()
			c.inflightWg.Done()
		}
	}
}

// handleMessage parses and routes a single billing event message.
//
// Idempotency: deduplicates by the domain event ID (unique per event instance).
// This allows legitimate repeated operations on the same aggregate (e.g., two
// subscription.renewed events for the same sub) while still catching outbox
// relay re-publishes. Events without an ID (backward compat) fall back to
// {event_type}:{entity_id}.
//
// Ordering: acquires a per-entity lock so that concurrent events for the same
// subscription (arriving on different NATS subjects) are processed serially.
//
// On processing failure, messages are Nack'd for retry up to MaxMessageRetries
// times. Messages that exceed the retry limit are sent to the dead-letter queue
// and Ack'd to prevent infinite redelivery.
func (c *BillingEventConsumer) handleMessage(ctx context.Context, subject string, msg *message.Message) {
	// Extract W3C trace context from NATS message metadata (set by the outbox
	// relay) and create a linked span. This connects the consumer processing
	// to the originating business operation's trace.
	carrier := propagation.MapCarrier(msg.Metadata)
	ctx = tracing.ExtractTraceContext(ctx, carrier)
	ctx, span := tracing.StartSpan(ctx, "billing_event.process")
	defer span.End()

	start := time.Now()

	// Parse the event first so we can extract EntityID for idempotency.
	var event domainevent.Event
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		c.logger.Error("failed to unmarshal billing event, sending to DLQ",
			slog.String("subject", subject),
			slog.Any("error", err),
		)
		c.sendToDLQ(subject, msg, err, 0, nil)
		msg.Ack()
		return
	}

	// Upcast old event payloads to the latest schema version before processing.
	event = c.schemaRegistry.Upcast(event)

	// Record event processing lag: the time between when the event was created
	// (by the domain service) and when the consumer begins processing it. This
	// captures outbox relay delay + NATS delivery latency.
	if !event.Timestamp.IsZero() {
		lag := time.Since(event.Timestamp).Seconds()
		c.recordMetric(func(m *observability.Metrics) {
			m.EventProcessingLag.WithLabelValues(string(event.Type)).Observe(lag)
		})
	}

	// Resolve entity ID: prefer the top-level EntityID field; fall back to
	// extracting subscription_id from the data payload for backward compat
	// with events published before the EntityID migration.
	entityID := event.EntityID
	if entityID == "" {
		entityID = extractString(event.Data, "subscription_id")
	}

	// Per-event-instance idempotency key: use the domain event ID when
	// available. This prevents false duplicates where two legitimate events
	// of the same type target the same entity (e.g., a second
	// subscription.renewed for the same sub). Events without an ID (backward
	// compat: pre-UUIDv7 migration) fall back to {event_type}:{entity_id}.
	// If the idempotency check fails (DB error), we fail open —
	// at-least-once delivery is safer than silently dropping.
	idempotencyKey := event.ID
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("%s:%s", event.Type, entityID)
	}
	isNew, err := c.idempotency.TryAcquire(ctx, idempotencyKey)
	if err != nil {
		c.logger.Warn("idempotency check failed, processing message anyway",
			slog.String("msg_id", msg.UUID),
			slog.String("subject", subject),
			slog.String("idempotency_key", idempotencyKey),
			slog.Any("error", err),
		)
	} else if !isNew {
		c.logger.Debug("duplicate event, skipping",
			slog.String("msg_id", msg.UUID),
			slog.String("subject", subject),
			slog.String("idempotency_key", idempotencyKey),
		)
		c.recordMetric(func(m *observability.Metrics) {
			m.EventsProcessedTotal.WithLabelValues(string(event.Type), observability.StatusSkippedDuplicate).Inc()
			m.IdempotencyHitTotal.WithLabelValues(string(event.Type)).Inc()
		})
		msg.Ack()
		return
	}

	// Serialise processing for the same entity to guarantee ordering.
	// Events for different entities run concurrently.
	if entityID != "" {
		lockStart := time.Now()
		lock := c.getEntityLock(entityID)
		lock.mu.Lock()
		defer lock.mu.Unlock()
		c.recordMetric(func(m *observability.Metrics) {
			m.EntityLockWaitLatency.WithLabelValues(string(event.Type)).Observe(time.Since(lockStart).Seconds())
		})
	}

	handleErr := c.processEvent(ctx, subject, event)
	if handleErr == nil {
		c.recordMetric(func(m *observability.Metrics) {
			m.EventsProcessedTotal.WithLabelValues(string(event.Type), observability.StatusSuccess).Inc()
			m.EventProcessingLatency.WithLabelValues(string(event.Type)).Observe(time.Since(start).Seconds())
		})
		msg.Ack()
		return
	}

	// Processing failed — release the idempotency key so the next delivery
	// is not silently skipped as a duplicate.
	if releaseErr := c.idempotency.Release(ctx, idempotencyKey); releaseErr != nil {
		c.logger.Warn("failed to release idempotency key after processing error",
			slog.String("msg_id", msg.UUID),
			slog.String("idempotency_key", idempotencyKey),
			slog.Any("error", releaseErr),
		)
	}

	// Track retry attempts via a separate "retry:" key in the idempotency
	// table. This survives NATS redeliveries because it is persisted in
	// PostgreSQL — unlike Watermill message metadata which is lost on Nack.
	retryKey := retryKeyPrefix + idempotencyKey
	retryCount, retryErr := c.idempotency.IncrementRetry(ctx, retryKey)
	if retryErr != nil {
		c.logger.Warn("failed to increment retry count, will retry anyway",
			slog.String("msg_id", msg.UUID),
			slog.String("retry_key", retryKey),
			slog.Any("error", retryErr),
		)
		c.recordMetric(func(m *observability.Metrics) {
			m.EventsProcessedTotal.WithLabelValues(string(event.Type), observability.StatusFailed).Inc()
		})
		// Fail open: allow retry even if we cannot track the count.
		msg.Nack()
		return
	}

	if retryCount < MaxMessageRetries {
		c.logger.Warn("event processing failed, will retry",
			slog.String("subject", subject),
			slog.String("msg_id", msg.UUID),
			slog.Int("retry", retryCount),
			slog.Int("max_retries", MaxMessageRetries),
			slog.String("error", handleErr.Error()),
		)
		c.recordMetric(func(m *observability.Metrics) {
			m.EventsProcessedTotal.WithLabelValues(string(event.Type), observability.StatusFailed).Inc()
		})
		msg.Nack()
		return
	}

	// Max retries exceeded — send to DLQ and acknowledge to stop redelivery.
	c.sendToDLQ(subject, msg, handleErr, retryCount, &event)
	c.logger.Error("event processing failed permanently, sent to DLQ",
		slog.String("subject", subject),
		slog.String("msg_id", msg.UUID),
		slog.Int("retries_exhausted", retryCount),
		slog.String("error", handleErr.Error()),
	)
	c.recordMetric(func(m *observability.Metrics) {
		m.EventsProcessedTotal.WithLabelValues(string(event.Type), observability.StatusDLQ).Inc()
		m.DLQPublishedTotal.Inc()
	})
	msg.Ack()
}

// recordMetric is a nil-safe helper that executes metric recording only when
// the metrics dependency is present. Tests pass nil for metrics.
func (c *BillingEventConsumer) recordMetric(fn func(*observability.Metrics)) {
	if c.metrics != nil {
		fn(c.metrics)
	}
}

// processEvent routes the parsed event to the appropriate handler.
func (c *BillingEventConsumer) processEvent(ctx context.Context, subject string, event domainevent.Event) error {
	switch subject {
	case string(billing.EventSubActivated):
		return c.handleActivated(ctx, event)
	case string(billing.EventSubCancelled):
		return c.handleSimple(ctx, event, c.handler.OnSubscriptionCancelled)
	case string(billing.EventSubPaused):
		return c.handleSimple(ctx, event, c.handler.OnSubscriptionPaused)
	case string(billing.EventSubResumed):
		return c.handleSimple(ctx, event, c.handler.OnSubscriptionResumed)
	default:
		c.logger.Warn("unhandled billing event subject", slog.String("subject", subject))
		return nil
	}
}

// sendToDLQ publishes a failed message to the dead-letter queue stream. The DLQ
// message preserves the original payload and adds diagnostic metadata. The
// optional event parameter enriches the DLQ payload with entity ID and event
// type when the event was successfully parsed before the failure occurred.
func (c *BillingEventConsumer) sendToDLQ(subject string, msg *message.Message, processingErr error, retryCount int, event *domainevent.Event) {
	dlqSubject := DLQSubjectPrefix + subject
	dlqPayload := DLQPayload{
		OriginalSubject: subject,
		OriginalPayload: string(msg.Payload),
		Error:           processingErr.Error(),
		MsgID:           msg.UUID,
		FailedAt:        c.clock.Now().Format(time.RFC3339),
		RetryCount:      retryCount,
	}

	if event != nil {
		dlqPayload.EntityID = event.EntityID
		dlqPayload.EventType = string(event.Type)
	}

	data, err := json.Marshal(dlqPayload)
	if err != nil {
		c.logger.Error("failed to marshal DLQ payload",
			slog.String("subject", dlqSubject),
			slog.Any("error", err),
		)
		return
	}

	if c.publisher == nil {
		c.logger.Error("DLQ publisher not configured, message dropped",
			slog.String("subject", dlqSubject),
		)
		return
	}

	dlqMsg := message.NewMessage(watermill.NewUUID(), data)
	if err := c.publisher.PublishRaw(dlqSubject, dlqMsg); err != nil {
		c.logger.Error("failed to publish to DLQ",
			slog.String("subject", dlqSubject),
			slog.Any("error", err),
		)
	}
}

// DLQPayload is the JSON envelope written to dead-letter queue topics.
type DLQPayload struct {
	OriginalSubject string `json:"original_subject"`
	OriginalPayload string `json:"original_payload"`
	Error           string `json:"error"`
	MsgID           string `json:"msg_id"`
	FailedAt        string `json:"failed_at"`
	RetryCount      int    `json:"retry_count"`
	EntityID        string `json:"entity_id,omitempty"`
	EventType       string `json:"event_type,omitempty"`
}

// handleActivated enriches the sparse activated event with subscription, plan,
// and family data before dispatching to the orchestrator.
func (c *BillingEventConsumer) handleActivated(ctx context.Context, event domainevent.Event) error {
	subscriptionID := extractString(event.Data, "subscription_id")
	if subscriptionID == "" {
		return errMissingSubscriptionID
	}

	subInfo, err := c.subs.GetSubscriptionInfo(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("lookup subscription %s: %w", subscriptionID, err)
	}

	plan, err := c.plans.GetPlanSnapshot(ctx, subInfo.PlanID)
	if err != nil {
		return fmt.Errorf("lookup plan %s: %w", subInfo.PlanID, err)
	}

	familyMemberIDs, err := c.subs.GetFamilyMemberIDs(ctx, subInfo.UserID)
	if err != nil {
		c.logger.Warn("failed to lookup family members, proceeding without",
			slog.String("user_id", subInfo.UserID),
			slog.Any("error", err),
		)
		familyMemberIDs = nil
	}

	return c.handler.OnSubscriptionActivated(
		ctx,
		subInfo.ID,
		subInfo.UserID,
		plan,
		subInfo.AddonIDs,
		familyMemberIDs,
	)
}

// handleSimple handles events that only require a subscription_id.
func (c *BillingEventConsumer) handleSimple(
	ctx context.Context,
	event domainevent.Event,
	fn func(ctx context.Context, subscriptionID string) error,
) error {
	subscriptionID := extractString(event.Data, "subscription_id")
	if subscriptionID == "" {
		return errMissingSubscriptionID
	}

	return fn(ctx, subscriptionID)
}

// extractString extracts a string field from event data.
// Data is expected to be map[string]any (from JSON unmarshal of NATS messages).
func extractString(data any, key string) string {
	m, ok := data.(map[string]any)
	if !ok {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
