package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"go.opentelemetry.io/otel/propagation"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	billingaggregate "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
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

// errMissingInvoiceID is returned when a payment.charge_completed event lacks
// the required invoice_id field in its data payload.
var errMissingInvoiceID = errors.New("invoice_id missing from event data")

// Traffic lifecycle NATS subjects. These originate from Remnawave webhooks,
// are translated by the ACL (SyncSaga.HandleWebhookEvent) into domain events,
// and published to NATS with BindingWebhookPayload data.
const (
	subjectBindingTrafficExceeded = "binding.traffic_exceeded"
	subjectTrafficCycleReset      = "subscription.traffic_cycle_reset"
	subjectTrafficWarning         = "subscription.traffic_warning"
)

// subscriptionIDPayload is a minimal typed payload used to extract
// subscription_id from events handled by handleSimple (cancelled, paused,
// resumed). These domain events all share a subscription_id field in their
// payload even though the concrete types differ.
type subscriptionIDPayload struct {
	SubscriptionID string `json:"subscription_id"`
}

// trafficWarningPayload extends BindingWebhookPayload with traffic metrics.
// The ACL does not currently populate these fields (zeros are passed), but the
// payload struct is forward-compatible for when the ACL enriches the payload.
type trafficWarningPayload struct {
	BindingID      string `json:"binding_id"`
	SubscriptionID string `json:"subscription_id"`
	UsedBytes      int64  `json:"used_bytes"`
	LimitBytes     int64  `json:"limit_bytes"`
	ThresholdPct   int    `json:"threshold_pct"`
}

// trafficPayload extends BindingWebhookPayload with traffic byte counters.
// Forward-compatible for ACL enrichment.
type trafficPayload struct {
	BindingID      string `json:"binding_id"`
	SubscriptionID string `json:"subscription_id"`
	UsedBytes      int64  `json:"used_bytes"`
	LimitBytes     int64  `json:"limit_bytes"`
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

	// Warn if the event version exceeds the latest known version. This can
	// happen when a producer has been upgraded before all consumers. The event
	// is still processed best-effort — we never reject unknown versions.
	if latestVersion := c.schemaRegistry.LatestVersion(event.Type); event.Version > latestVersion {
		c.logger.Warn("event version exceeds latest known schema version, processing best-effort",
			slog.String("event_type", string(event.Type)),
			slog.Int("event_version", event.Version),
			slog.Int("latest_version", latestVersion),
		)
	}

	// Record event processing lag: the time between when the event was created
	// (by the domain service) and when the consumer begins processing it. This
	// captures outbox relay delay + NATS delivery latency.
	if !event.Timestamp.IsZero() {
		lag := time.Since(event.Timestamp).Seconds()
		c.recordMetric(func(m *observability.Metrics) {
			m.EventProcessingLag.WithLabelValues(string(event.Type)).Observe(lag)
		})
	}

	// Resolve entity ID: prefer the top-level EntityID field (set by
	// NewTyped/NewWithEntity at event construction time). Fall back to
	// extracting subscription_id or binding_id from the data payload for
	// backward compat with events published before the EntityID migration.
	//
	// New events MUST set EntityID via NewTyped or NewWithEntity. The Data
	// fallback exists only for legacy events still in NATS retention and
	// should be removed once all pre-migration events have expired.
	entityID := event.EntityID
	if entityID == "" {
		entityID = extractStringFromData(event.Data, "subscription_id")
		if entityID == "" {
			entityID = extractStringFromData(event.Data, "binding_id")
		}
		if entityID != "" {
			c.logger.Debug("event missing EntityID, resolved from Data fallback",
				slog.String("event_type", string(event.Type)),
				slog.String("entity_id", entityID),
			)
		}
	}

	// Consumer-side sequence gap detection: extract the outbox sequence
	// number from NATS message metadata and check for gaps against the
	// last seen sequence for this entity. This is observability-only —
	// it logs and increments a counter but never blocks processing.
	if seqStr := msg.Metadata.Get(HeaderOutboxSequence); seqStr != "" {
		if seq, parseErr := strconv.ParseInt(seqStr, 10, 64); parseErr == nil {
			c.seqTracker.checkAndUpdate(entityID, seq)
		}
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
		delay := retryBackoffDelay(retryCount)
		c.logger.Warn("event processing failed, will retry after backoff",
			slog.String("subject", subject),
			slog.String("msg_id", msg.UUID),
			slog.Int("retry", retryCount),
			slog.Int("max_retries", MaxMessageRetries),
			slog.Duration("backoff", delay),
			slog.String("error", handleErr.Error()),
		)
		c.recordMetric(func(m *observability.Metrics) {
			m.EventsProcessedTotal.WithLabelValues(string(event.Type), observability.StatusFailed).Inc()
		})
		// Application-level backoff: Watermill's Message interface does not
		// expose NakWithDelay, so we sleep before Nack to prevent rapid
		// redelivery. The sleep is bounded by the message processing timeout
		// (context deadline) set in consumeLoop.
		time.Sleep(delay)
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
	case string(payment.EventChargeCompleted):
		return c.handleChargeCompleted(ctx, event)
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
	payload, err := domainevent.UnmarshalPayload[billingaggregate.SubActivatedPayload](event)
	if err != nil {
		return fmt.Errorf("unmarshal activated payload: %w", err)
	}
	if payload.SubscriptionID == "" {
		return errMissingSubscriptionID
	}

	subInfo, err := c.subs.GetSubscriptionInfo(ctx, payload.SubscriptionID)
	if err != nil {
		return fmt.Errorf("lookup subscription %s: %w", payload.SubscriptionID, err)
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

// handleChargeCompleted handles the payment.charge_completed event published by
// the payment context after a successful payment. It extracts the invoice_id
// from the event payload and delegates to the CheckoutCompleter (billing's
// CheckoutService) to mark the invoice as paid and activate the subscription.
// This is the asynchronous bridge between the payment and billing contexts,
// replacing the previous synchronous cross-context call in the webhook handler.
func (c *BillingEventConsumer) handleChargeCompleted(ctx context.Context, event domainevent.Event) error {
	payload, err := domainevent.UnmarshalPayload[payment.ChargeCompletedPayload](event)
	if err != nil {
		return fmt.Errorf("unmarshal charge_completed payload: %w", err)
	}
	if payload.InvoiceID == "" {
		return errMissingInvoiceID
	}

	return c.checkout.CompleteCheckout(ctx, payload.InvoiceID)
}

// handleSimple handles events that only require a subscription_id (cancelled,
// paused, resumed). The payload is unmarshalled into subscriptionIDPayload
// which captures the shared subscription_id field from any of these event types.
func (c *BillingEventConsumer) handleSimple(
	ctx context.Context,
	event domainevent.Event,
	fn func(ctx context.Context, subscriptionID string) error,
) error {
	payload, err := domainevent.UnmarshalPayload[subscriptionIDPayload](event)
	if err != nil {
		return fmt.Errorf("unmarshal %s payload: %w", event.Type, err)
	}
	if payload.SubscriptionID == "" {
		return errMissingSubscriptionID
	}

	return fn(ctx, payload.SubscriptionID)
}

// handleTrafficExceeded handles the binding.traffic_exceeded event. It extracts
// binding and subscription identifiers from the BindingWebhookPayload and
// delegates to the orchestrator to limit the binding via Remnawave.
//
// Traffic usage fields (usedBytes, limitBytes) are not available in the current
// webhook payload; zeros are passed to the orchestrator. If the orchestrator
// needs precise values in the future, the ACL should enrich the payload.
func (c *BillingEventConsumer) handleTrafficExceeded(ctx context.Context, event domainevent.Event) error {
	payload, err := domainevent.UnmarshalPayload[trafficPayload](event)
	if err != nil {
		return fmt.Errorf("unmarshal traffic_exceeded payload: %w", err)
	}
	if payload.BindingID == "" {
		return errMissingBindingID
	}
	if payload.SubscriptionID == "" {
		return errMissingSubscriptionID
	}

	return c.handler.OnBindingTrafficExceeded(ctx, payload.BindingID, payload.SubscriptionID, payload.UsedBytes, payload.LimitBytes)
}

// handleTrafficReset handles the subscription.traffic_cycle_reset event. It
// extracts binding and subscription identifiers and delegates to the
// orchestrator to unlimit the binding.
func (c *BillingEventConsumer) handleTrafficReset(ctx context.Context, event domainevent.Event) error {
	payload, err := domainevent.UnmarshalPayload[multisub.BindingWebhookPayload](event)
	if err != nil {
		return fmt.Errorf("unmarshal traffic_cycle_reset payload: %w", err)
	}
	if payload.BindingID == "" {
		return errMissingBindingID
	}
	if payload.SubscriptionID == "" {
		return errMissingSubscriptionID
	}

	return c.handler.OnBindingTrafficReset(ctx, payload.BindingID, payload.SubscriptionID)
}

// handleTrafficWarning handles the subscription.traffic_warning event. It
// extracts binding identifiers and traffic metrics, then delegates to the
// orchestrator which fires an async hook for plugin notification. Since
// OnTrafficWarning is fire-and-forget (no error return), this handler always
// returns nil on successful payload extraction.
func (c *BillingEventConsumer) handleTrafficWarning(ctx context.Context, event domainevent.Event) error {
	payload, err := domainevent.UnmarshalPayload[trafficWarningPayload](event)
	if err != nil {
		return fmt.Errorf("unmarshal traffic_warning payload: %w", err)
	}
	if payload.BindingID == "" {
		return errMissingBindingID
	}
	if payload.SubscriptionID == "" {
		return errMissingSubscriptionID
	}

	c.handler.OnTrafficWarning(ctx, payload.BindingID, payload.SubscriptionID, payload.UsedBytes, payload.LimitBytes, payload.ThresholdPct)
	return nil
}
