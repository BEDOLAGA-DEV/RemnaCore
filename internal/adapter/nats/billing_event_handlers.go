package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"go.opentelemetry.io/otel/propagation"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/observability"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tracing"
)

// errMissingSubscriptionID is returned when a billing event lacks the required
// subscription_id field in its data payload.
var errMissingSubscriptionID = errors.New("subscription_id missing from event data")

// errMissingBindingID is returned when a traffic event lacks the required
// binding_id field in its data payload.
var errMissingBindingID = errors.New("binding_id missing from event data")

// Traffic lifecycle NATS subjects. These originate from Remnawave webhooks,
// are translated by the ACL (SyncSaga.HandleWebhookEvent) into domain events,
// and published to NATS with BindingWebhookPayload data.
const (
	subjectBindingTrafficExceeded = "binding.traffic_exceeded"
	subjectTrafficCycleReset      = "subscription.traffic_cycle_reset"
	subjectTrafficWarning         = "subscription.traffic_warning"
)

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

// recordMetric is a nil-safe helper that executes metric recording only when
// the metrics dependency is present. Tests pass nil for metrics.
func (c *BillingEventConsumer) recordMetric(fn func(*observability.Metrics)) {
	if c.metrics != nil {
		fn(c.metrics)
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
	// extracting subscription_id or binding_id from the data payload for
	// backward compat with events published before the EntityID migration
	// and for binding traffic events that use binding_id as the entity key.
	entityID := event.EntityID
	if entityID == "" {
		entityID = extractString(event.Data, "subscription_id")
	}
	if entityID == "" {
		entityID = extractString(event.Data, "binding_id")
	}

	// Per-event-instance idempotency key: use the domain event ID when
	// available. This prevents false duplicates where two legitimate events
	// of the same type target the same entity (e.g., a second
	// subscription.renewed for the same sub). Events without an ID (backward
	// compat: pre-UUIDv7 migration) fall back to {event_type}:{entity_id}.
	// For webhook-originated events where Type may be empty, the NATS
	// subject is used as the type component.
	// If the idempotency check fails (DB error), we fail open —
	// at-least-once delivery is safer than silently dropping.
	idempotencyKey := event.ID
	if idempotencyKey == "" {
		eventType := string(event.Type)
		if eventType == "" {
			eventType = subject
		}
		idempotencyKey = fmt.Sprintf("%s:%s", eventType, entityID)
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
	case subjectBindingTrafficExceeded:
		return c.handleTrafficExceeded(ctx, event)
	case subjectTrafficCycleReset:
		return c.handleTrafficReset(ctx, event)
	case subjectTrafficWarning:
		return c.handleTrafficWarning(ctx, event)
	default:
		c.logger.Warn("unhandled billing event subject", slog.String("subject", subject))
		return nil
	}
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

// handleTrafficExceeded handles the binding.traffic_exceeded event. It extracts
// binding and subscription identifiers from the BindingWebhookPayload and
// delegates to the orchestrator to limit the binding via Remnawave.
//
// Traffic usage fields (usedBytes, limitBytes) are not available in the current
// webhook payload; zeros are passed to the orchestrator. If the orchestrator
// needs precise values in the future, the ACL should enrich the payload.
func (c *BillingEventConsumer) handleTrafficExceeded(ctx context.Context, event domainevent.Event) error {
	bindingID := extractString(event.Data, "binding_id")
	if bindingID == "" {
		return errMissingBindingID
	}
	subscriptionID := extractString(event.Data, "subscription_id")
	if subscriptionID == "" {
		return errMissingSubscriptionID
	}

	usedBytes := extractInt64(event.Data, "used_bytes")
	limitBytes := extractInt64(event.Data, "limit_bytes")

	return c.handler.OnBindingTrafficExceeded(ctx, bindingID, subscriptionID, usedBytes, limitBytes)
}

// handleTrafficReset handles the subscription.traffic_cycle_reset event. It
// extracts binding and subscription identifiers and delegates to the
// orchestrator to unlimit the binding.
func (c *BillingEventConsumer) handleTrafficReset(ctx context.Context, event domainevent.Event) error {
	bindingID := extractString(event.Data, "binding_id")
	if bindingID == "" {
		return errMissingBindingID
	}
	subscriptionID := extractString(event.Data, "subscription_id")
	if subscriptionID == "" {
		return errMissingSubscriptionID
	}

	return c.handler.OnBindingTrafficReset(ctx, bindingID, subscriptionID)
}

// handleTrafficWarning handles the subscription.traffic_warning event. It
// extracts binding identifiers and traffic metrics, then delegates to the
// orchestrator which fires an async hook for plugin notification. Since
// OnTrafficWarning is fire-and-forget (no error return), this handler always
// returns nil on successful payload extraction.
func (c *BillingEventConsumer) handleTrafficWarning(ctx context.Context, event domainevent.Event) error {
	bindingID := extractString(event.Data, "binding_id")
	if bindingID == "" {
		return errMissingBindingID
	}
	subscriptionID := extractString(event.Data, "subscription_id")
	if subscriptionID == "" {
		return errMissingSubscriptionID
	}

	usedBytes := extractInt64(event.Data, "used_bytes")
	limitBytes := extractInt64(event.Data, "limit_bytes")
	thresholdPct := extractInt(event.Data, "threshold_pct")

	c.handler.OnTrafficWarning(ctx, bindingID, subscriptionID, usedBytes, limitBytes, thresholdPct)
	return nil
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

// extractInt64 extracts a numeric field as int64 from event data. JSON numbers
// unmarshal as float64 in map[string]any; this helper handles the conversion.
// Returns 0 if the key is missing, not a number, or the data is not a map.
func extractInt64(data any, key string) int64 {
	m, ok := data.(map[string]any)
	if !ok {
		return 0
	}
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int64(n)
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0
		}
		return i
	default:
		return 0
	}
}

// extractInt extracts a numeric field as int from event data. See extractInt64
// for details on JSON number handling.
func extractInt(data any, key string) int {
	return int(extractInt64(data, key))
}
