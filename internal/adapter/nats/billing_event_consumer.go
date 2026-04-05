package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
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

	// MetadataRetryCount is the Watermill metadata key used to track how many
	// times a message has been redelivered by the consumer.
	MetadataRetryCount = "retry_count"
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

// IdempotencyChecker provides event-level deduplication. The adapter layer
// owns this interface; the postgres.IdempotencyRepository satisfies it.
//
// Keys are composed as "{event_type}:{entity_id}" to deduplicate at the
// business level rather than the transport level (Watermill message UUID).
type IdempotencyChecker interface {
	// TryAcquire returns true if the key is new, false if it was already seen.
	TryAcquire(ctx context.Context, key string) (bool, error)
}

// entityLockTTL is the duration after which an idle entity lock is eligible for
// eviction. This prevents unbounded growth of the entityLocks map.
const entityLockTTL = 10 * time.Minute

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
//   - Business-level idempotency: events are deduplicated by
//     {event_type}:{entity_id}, not Watermill message UUID.
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
	entityLocks    sync.Map // map[string]*entityLock — per-entity serialisation
}

// NewBillingEventConsumer creates a BillingEventConsumer with the given
// dependencies. The publisher is used to route permanently failed messages to
// the dead-letter queue. Plan and subscription data are resolved through
// multisub domain ports (PlanProvider + SubscriptionProvider). The schema
// registry upcasts old event payloads to the latest version before processing.
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
// processes.
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

	c.logger.Info("billing event consumer started",
		slog.Int("subscribed", subscribed),
		slog.Int("total", len(billingSubscriptionSubjects())),
	)
	return nil
}

// consumeLoop reads messages from a single subscription channel until the
// context is cancelled or the channel is closed.
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
			c.handleMessage(ctx, subject, msg)
		}
	}
}

// handleMessage parses and routes a single billing event message.
//
// Idempotency: deduplicates by {event_type}:{entity_id} (business-level key),
// not by Watermill message UUID. This catches outbox relay re-publishes where
// the same business event gets a new transport UUID.
//
// Ordering: acquires a per-entity lock so that concurrent events for the same
// subscription (arriving on different NATS subjects) are processed serially.
//
// On processing failure, messages are Nack'd for retry up to MaxMessageRetries
// times. Messages that exceed the retry limit are sent to the dead-letter queue
// and Ack'd to prevent infinite redelivery.
func (c *BillingEventConsumer) handleMessage(ctx context.Context, subject string, msg *message.Message) {
	// Parse the event first so we can extract EntityID for idempotency.
	var event domainevent.Event
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		c.logger.Error("failed to unmarshal billing event, sending to DLQ",
			slog.String("subject", subject),
			slog.Any("error", err),
		)
		c.sendToDLQ(subject, msg, err)
		msg.Ack()
		return
	}

	// Upcast old event payloads to the latest schema version before processing.
	event = c.schemaRegistry.Upcast(event)

	// Resolve entity ID: prefer the top-level EntityID field; fall back to
	// extracting subscription_id from the data payload for backward compat
	// with events published before the EntityID migration.
	entityID := event.EntityID
	if entityID == "" {
		entityID = extractString(event.Data, "subscription_id")
	}

	// Business-level idempotency key: {event_type}:{entity_id}. If the
	// idempotency check fails (DB error), we fail open — at-least-once
	// delivery is safer than silently dropping.
	idempotencyKey := fmt.Sprintf("%s:%s", event.Type, entityID)
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
		msg.Ack()
		return
	}

	// Serialise processing for the same entity to guarantee ordering.
	// Events for different entities run concurrently.
	if entityID != "" {
		lock := c.getEntityLock(entityID)
		lock.mu.Lock()
		defer lock.mu.Unlock()
	}

	handleErr := c.processEvent(ctx, subject, event)
	if handleErr == nil {
		msg.Ack()
		return
	}

	retryCount := getRetryCount(msg)
	if retryCount < MaxMessageRetries {
		c.logger.Warn("event processing failed, will retry",
			slog.String("subject", subject),
			slog.String("msg_id", msg.UUID),
			slog.Int("retry", retryCount+1),
			slog.Int("max_retries", MaxMessageRetries),
			slog.String("error", handleErr.Error()),
		)
		msg.Nack()
		return
	}

	// Max retries exceeded — send to DLQ and acknowledge to stop redelivery.
	c.sendToDLQ(subject, msg, handleErr)
	c.logger.Error("event processing failed permanently, sent to DLQ",
		slog.String("subject", subject),
		slog.String("msg_id", msg.UUID),
		slog.Int("retries_exhausted", MaxMessageRetries),
		slog.String("error", handleErr.Error()),
	)
	msg.Ack()
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
// message preserves the original payload and adds diagnostic metadata.
func (c *BillingEventConsumer) sendToDLQ(subject string, msg *message.Message, processingErr error) {
	dlqSubject := DLQSubjectPrefix + subject
	dlqPayload := DLQPayload{
		OriginalSubject: subject,
		OriginalPayload: string(msg.Payload),
		Error:           processingErr.Error(),
		MsgID:           msg.UUID,
		FailedAt:        c.clock.Now().Format(time.RFC3339),
		RetryCount:      getRetryCount(msg),
	}

	data, err := json.Marshal(dlqPayload)
	if err != nil {
		c.logger.Error("failed to marshal DLQ payload",
			slog.String("subject", dlqSubject),
			slog.Any("error", err),
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
}

// getRetryCount reads the retry count from Watermill message metadata. Returns
// 0 if the metadata key is absent or unparseable.
func getRetryCount(msg *message.Message) int {
	countStr := msg.Metadata.Get(MetadataRetryCount)
	if countStr == "" {
		return 0
	}
	count, err := strconv.Atoi(countStr)
	if err != nil {
		return 0
	}
	return count
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
