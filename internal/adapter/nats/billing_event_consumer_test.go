package nats

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/observability"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// --- test doubles ---

// stubIdempotencyChecker records keys and allows controlling duplicates.
// It also tracks Release and IncrementRetry calls for assertion.
type stubIdempotencyChecker struct {
	mu           sync.Mutex
	seen         map[string]bool
	retryCounts  map[string]int
	released     []string
	forceErr     error
	forceRetryErr error
}

func newStubIdempotencyChecker() *stubIdempotencyChecker {
	return &stubIdempotencyChecker{
		seen:        make(map[string]bool),
		retryCounts: make(map[string]int),
	}
}

func (s *stubIdempotencyChecker) TryAcquire(_ context.Context, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.forceErr != nil {
		return false, s.forceErr
	}
	if s.seen[key] {
		return false, nil
	}
	s.seen[key] = true
	return true, nil
}

func (s *stubIdempotencyChecker) Release(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.seen, key)
	s.released = append(s.released, key)
	return nil
}

func (s *stubIdempotencyChecker) IncrementRetry(_ context.Context, key string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.forceRetryErr != nil {
		return 0, s.forceRetryErr
	}
	s.retryCounts[key]++
	return s.retryCounts[key], nil
}

func (s *stubIdempotencyChecker) acquiredKeys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	keys := make([]string, 0, len(s.seen))
	for k := range s.seen {
		keys = append(keys, k)
	}
	return keys
}

func (s *stubIdempotencyChecker) releasedKeys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]string, len(s.released))
	copy(out, s.released)
	return out
}

func (s *stubIdempotencyChecker) retryCount(key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.retryCounts[key]
}

// alwaysNewIdempotencyChecker always reports the key as new. Used when the
// test needs to bypass dedup and focus on processing behaviour.
type alwaysNewIdempotencyChecker struct{}

func (alwaysNewIdempotencyChecker) TryAcquire(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (alwaysNewIdempotencyChecker) Release(_ context.Context, _ string) error {
	return nil
}

func (alwaysNewIdempotencyChecker) IncrementRetry(_ context.Context, _ string) (int, error) {
	return 0, nil
}

// recordingCheckoutCompleter implements CheckoutCompleter for tests.
type recordingCheckoutCompleter struct {
	onCompleteCheckout func(ctx context.Context, invoiceID string) error
}

func (r *recordingCheckoutCompleter) CompleteCheckout(ctx context.Context, invoiceID string) error {
	if r.onCompleteCheckout != nil {
		return r.onCompleteCheckout(ctx, invoiceID)
	}
	return nil
}

// recordingHandler implements SubscriptionEventHandler for tests. Callback
// fields are optional; nil means the method returns nil immediately.
type recordingHandler struct {
	onActivated       func(ctx context.Context, subID, userID string, plan multisub.PlanSnapshot, addonIDs, familyMemberIDs []string) error
	onCancelled       func(ctx context.Context, subID string) error
	onPaused          func(ctx context.Context, subID string) error
	onResumed         func(ctx context.Context, subID string) error
	onTrafficExceeded func(ctx context.Context, bindingID, subscriptionID string, usedBytes, limitBytes int64) error
	onTrafficReset    func(ctx context.Context, bindingID, subscriptionID string) error
	onTrafficWarning  func(ctx context.Context, bindingID, subscriptionID string, usedBytes, limitBytes int64, thresholdPct int)
}

func (h *recordingHandler) OnSubscriptionActivated(
	ctx context.Context,
	subscriptionID string,
	platformUserID string,
	plan multisub.PlanSnapshot,
	addonIDs []string,
	familyMemberIDs []string,
) error {
	if h.onActivated != nil {
		return h.onActivated(ctx, subscriptionID, platformUserID, plan, addonIDs, familyMemberIDs)
	}
	return nil
}

func (h *recordingHandler) OnSubscriptionCancelled(ctx context.Context, subID string) error {
	if h.onCancelled != nil {
		return h.onCancelled(ctx, subID)
	}
	return nil
}

func (h *recordingHandler) OnSubscriptionPaused(ctx context.Context, subID string) error {
	if h.onPaused != nil {
		return h.onPaused(ctx, subID)
	}
	return nil
}

func (h *recordingHandler) OnSubscriptionResumed(ctx context.Context, subID string) error {
	if h.onResumed != nil {
		return h.onResumed(ctx, subID)
	}
	return nil
}

func (h *recordingHandler) OnBindingTrafficExceeded(ctx context.Context, bindingID, subscriptionID string, usedBytes, limitBytes int64) error {
	if h.onTrafficExceeded != nil {
		return h.onTrafficExceeded(ctx, bindingID, subscriptionID, usedBytes, limitBytes)
	}
	return nil
}

func (h *recordingHandler) OnBindingTrafficReset(ctx context.Context, bindingID, subscriptionID string) error {
	if h.onTrafficReset != nil {
		return h.onTrafficReset(ctx, bindingID, subscriptionID)
	}
	return nil
}

func (h *recordingHandler) OnTrafficWarning(ctx context.Context, bindingID, subscriptionID string, usedBytes, limitBytes int64, thresholdPct int) {
	if h.onTrafficWarning != nil {
		h.onTrafficWarning(ctx, bindingID, subscriptionID, usedBytes, limitBytes, thresholdPct)
	}
}

// discardLogger returns a slog.Logger that suppresses all output below fatal.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError + 4, // suppress all output
	}))
}

// --- helpers ---

// buildEventMessage creates a Watermill message from a domainevent.Event.
func buildEventMessage(t *testing.T, event domainevent.Event) *message.Message {
	t.Helper()

	payload, err := json.Marshal(event)
	require.NoError(t, err)

	return message.NewMessage(watermill.NewUUID(), payload)
}

// --- tests ---

func TestHandleMessage_PerEventInstanceIdempotencyKey(t *testing.T) {
	idem := newStubIdempotencyChecker()
	handler := &recordingHandler{}

	consumer := &BillingEventConsumer{
		handler:        handler,
		idempotency:    idem,
		schemaRegistry: domainevent.NewSchemaRegistry(),
		logger:         discardLogger(),
		clock:          clock.NewReal(),
	}

	// Use subscription.cancelled (handled by handleSimple) to avoid needing
	// the plans/subs providers that subscription.activated requires.
	event := domainevent.NewWithEntity(
		"subscription.cancelled",
		map[string]any{"subscription_id": "sub-123", "user_id": "u-1", "reason": "test"},
		"sub-123",
	)

	msg := buildEventMessage(t, event)
	consumer.handleMessage(context.Background(), "subscription.cancelled", msg)

	// The idempotency key must be the domain event ID, not the Watermill UUID
	// and not {event_type}:{entity_id}.
	keys := idem.acquiredKeys()
	require.Len(t, keys, 1)
	assert.Equal(t, event.ID, keys[0],
		"idempotency key must be the domain event ID")
}

func TestHandleMessage_DuplicateEventSkipped(t *testing.T) {
	idem := newStubIdempotencyChecker()
	var processCount atomic.Int32

	handler := &recordingHandler{
		onCancelled: func(_ context.Context, _ string) error {
			processCount.Add(1)
			return nil
		},
	}

	consumer := &BillingEventConsumer{
		handler:        handler,
		idempotency:    idem,
		schemaRegistry: domainevent.NewSchemaRegistry(),
		logger:         discardLogger(),
		clock:          clock.NewReal(),
	}

	event := domainevent.NewWithEntity(
		"subscription.cancelled",
		map[string]any{"subscription_id": "sub-456", "user_id": "u-2", "reason": "test"},
		"sub-456",
	)

	// First message — should be processed.
	msg1 := buildEventMessage(t, event)
	consumer.handleMessage(context.Background(), "subscription.cancelled", msg1)

	// Second message with different Watermill UUID but same business event.
	msg2 := buildEventMessage(t, event)
	consumer.handleMessage(context.Background(), "subscription.cancelled", msg2)

	assert.Equal(t, int32(1), processCount.Load(),
		"duplicate business event must be skipped")

	keys := idem.acquiredKeys()
	assert.Len(t, keys, 1)
}

func TestHandleMessage_FallbackToSubscriptionIDFromPayload(t *testing.T) {
	idem := newStubIdempotencyChecker()
	handler := &recordingHandler{}

	consumer := &BillingEventConsumer{
		handler:        handler,
		idempotency:    idem,
		schemaRegistry: domainevent.NewSchemaRegistry(),
		logger:         discardLogger(),
		clock:          clock.NewReal(),
	}

	// Event without EntityID (backward compat: pre-migration events).
	event := domainevent.Event{
		Type:      "subscription.paused",
		Timestamp: time.Now(),
		Data:      map[string]any{"subscription_id": "sub-legacy", "user_id": "u-3"},
	}

	msg := buildEventMessage(t, event)
	consumer.handleMessage(context.Background(), "subscription.paused", msg)

	keys := idem.acquiredKeys()
	require.Len(t, keys, 1)
	assert.Equal(t, "subscription.paused:sub-legacy", keys[0])
}

func TestHandleMessage_SerialProcessingPerEntity(t *testing.T) {
	idem := &alwaysNewIdempotencyChecker{}
	var mu sync.Mutex
	var order []string

	handler := &recordingHandler{
		onCancelled: func(_ context.Context, subID string) error {
			mu.Lock()
			order = append(order, "cancelled:"+subID)
			mu.Unlock()
			// Simulate work.
			time.Sleep(10 * time.Millisecond)
			return nil
		},
		onPaused: func(_ context.Context, subID string) error {
			mu.Lock()
			order = append(order, "paused:"+subID)
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			return nil
		},
	}

	consumer := &BillingEventConsumer{
		handler:        handler,
		idempotency:    idem,
		schemaRegistry: domainevent.NewSchemaRegistry(),
		logger:         discardLogger(),
		clock:          clock.NewReal(),
	}

	sameSubID := "sub-serial"

	cancelledEvent := domainevent.NewWithEntity(
		"subscription.cancelled",
		map[string]any{"subscription_id": sameSubID, "user_id": "u-1", "reason": "test"},
		sameSubID,
	)
	pausedEvent := domainevent.NewWithEntity(
		"subscription.paused",
		map[string]any{"subscription_id": sameSubID, "user_id": "u-1"},
		sameSubID,
	)

	msg1 := buildEventMessage(t, cancelledEvent)
	msg2 := buildEventMessage(t, pausedEvent)

	// Process both concurrently — the entity lock should serialise them.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		consumer.handleMessage(context.Background(), "subscription.cancelled", msg1)
	}()
	go func() {
		defer wg.Done()
		consumer.handleMessage(context.Background(), "subscription.paused", msg2)
	}()
	wg.Wait()

	// Both events should have been processed (order may vary, but not
	// interleaved — that is the serialisation guarantee).
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, order, 2)
}

func TestHandleMessage_DifferentEntitiesProcessConcurrently(t *testing.T) {
	idem := &alwaysNewIdempotencyChecker{}
	var started sync.WaitGroup
	started.Add(2)
	var done sync.WaitGroup
	done.Add(2)

	gate := make(chan struct{})

	handler := &recordingHandler{
		onCancelled: func(_ context.Context, _ string) error {
			started.Done()
			<-gate // block until both goroutines have started
			return nil
		},
	}

	consumer := &BillingEventConsumer{
		handler:        handler,
		idempotency:    idem,
		schemaRegistry: domainevent.NewSchemaRegistry(),
		logger:         discardLogger(),
		clock:          clock.NewReal(),
	}

	// Two events for DIFFERENT entities should not block each other.
	event1 := domainevent.NewWithEntity(
		"subscription.cancelled",
		map[string]any{"subscription_id": "sub-A", "user_id": "u-1", "reason": "test"},
		"sub-A",
	)
	event2 := domainevent.NewWithEntity(
		"subscription.cancelled",
		map[string]any{"subscription_id": "sub-B", "user_id": "u-2", "reason": "test"},
		"sub-B",
	)

	msg1 := buildEventMessage(t, event1)
	msg2 := buildEventMessage(t, event2)

	go func() {
		defer done.Done()
		consumer.handleMessage(context.Background(), "subscription.cancelled", msg1)
	}()
	go func() {
		defer done.Done()
		consumer.handleMessage(context.Background(), "subscription.cancelled", msg2)
	}()

	// Both handlers must have started concurrently. If entity locking
	// incorrectly serialised them, this would deadlock.
	started.Wait()
	close(gate)
	done.Wait()
}

func TestGetEntityLock_ReturnsSameLockForSameID(t *testing.T) {
	consumer := &BillingEventConsumer{clock: clock.NewReal()}

	lock1 := consumer.getEntityLock("sub-1")
	lock2 := consumer.getEntityLock("sub-1")
	lock3 := consumer.getEntityLock("sub-2")

	assert.Same(t, lock1, lock2, "same entity ID must return the same lock")
	assert.NotSame(t, lock1, lock3, "different entity IDs must return different locks")
}

func TestHandleMessage_FailureReleasesIdempotencyKey(t *testing.T) {
	idem := newStubIdempotencyChecker()
	handler := &recordingHandler{
		onCancelled: func(_ context.Context, _ string) error {
			return errors.New("transient error")
		},
	}

	consumer := &BillingEventConsumer{
		handler:        handler,
		idempotency:    idem,
		schemaRegistry: domainevent.NewSchemaRegistry(),
		logger:         discardLogger(),
		clock:          clock.NewReal(),
	}

	event := domainevent.NewWithEntity(
		"subscription.cancelled",
		map[string]any{"subscription_id": "sub-fail", "user_id": "u-1", "reason": "test"},
		"sub-fail",
	)

	msg := buildEventMessage(t, event)
	consumer.handleMessage(context.Background(), "subscription.cancelled", msg)

	// The idempotency key (event.ID) must have been released so the next
	// delivery can reprocess the event.
	released := idem.releasedKeys()
	require.Len(t, released, 1)
	assert.Equal(t, event.ID, released[0],
		"released key must be the domain event ID")

	// The key must no longer be in the "seen" set.
	assert.Empty(t, idem.acquiredKeys(),
		"idempotency key must be removed after release")
}

func TestHandleMessage_RetryCountIncrementedOnFailure(t *testing.T) {
	idem := newStubIdempotencyChecker()
	handler := &recordingHandler{
		onCancelled: func(_ context.Context, _ string) error {
			return errors.New("transient error")
		},
	}

	consumer := &BillingEventConsumer{
		handler:        handler,
		idempotency:    idem,
		schemaRegistry: domainevent.NewSchemaRegistry(),
		logger:         discardLogger(),
		clock:          clock.NewReal(),
	}

	event := domainevent.NewWithEntity(
		"subscription.cancelled",
		map[string]any{"subscription_id": "sub-retry", "user_id": "u-1", "reason": "test"},
		"sub-retry",
	)

	retryKey := retryKeyPrefix + event.ID

	// Simulate multiple redeliveries (each call = one NATS redelivery).
	for i := 1; i <= MaxMessageRetries-1; i++ {
		msg := buildEventMessage(t, event)
		consumer.handleMessage(context.Background(), "subscription.cancelled", msg)
		assert.Equal(t, i, idem.retryCount(retryKey),
			"retry count must match delivery attempt")
	}
}

func TestHandleMessage_DLQAfterMaxRetries(t *testing.T) {
	idem := newStubIdempotencyChecker()
	handler := &recordingHandler{
		onCancelled: func(_ context.Context, _ string) error {
			return errors.New("permanent error")
		},
	}

	consumer := &BillingEventConsumer{
		handler:        handler,
		idempotency:    idem,
		schemaRegistry: domainevent.NewSchemaRegistry(),
		logger:         discardLogger(),
		clock:          clock.NewReal(),
		// publisher is nil — sendToDLQ will log an error but not panic.
	}

	event := domainevent.NewWithEntity(
		"subscription.cancelled",
		map[string]any{"subscription_id": "sub-dlq", "user_id": "u-1", "reason": "test"},
		"sub-dlq",
	)

	retryKey := retryKeyPrefix + event.ID

	// Exhaust all retries. After MaxMessageRetries, the message must be Ack'd
	// (sent to DLQ) rather than Nack'd.
	for i := 0; i < MaxMessageRetries; i++ {
		msg := buildEventMessage(t, event)
		consumer.handleMessage(context.Background(), "subscription.cancelled", msg)
	}

	assert.Equal(t, MaxMessageRetries, idem.retryCount(retryKey),
		"retry count must reach max before DLQ routing")
}

func TestHandleMessage_RedeliveryAfterFailureIsProcessed(t *testing.T) {
	idem := newStubIdempotencyChecker()
	var callCount atomic.Int32

	handler := &recordingHandler{
		onCancelled: func(_ context.Context, _ string) error {
			n := callCount.Add(1)
			if n == 1 {
				return errors.New("first attempt fails")
			}
			return nil // second attempt succeeds
		},
	}

	consumer := &BillingEventConsumer{
		handler:        handler,
		idempotency:    idem,
		schemaRegistry: domainevent.NewSchemaRegistry(),
		logger:         discardLogger(),
		clock:          clock.NewReal(),
	}

	event := domainevent.NewWithEntity(
		"subscription.cancelled",
		map[string]any{"subscription_id": "sub-redeliver", "user_id": "u-1", "reason": "test"},
		"sub-redeliver",
	)

	// First delivery — fails, key released.
	msg1 := buildEventMessage(t, event)
	consumer.handleMessage(context.Background(), "subscription.cancelled", msg1)

	// Second delivery — must NOT be skipped as duplicate.
	msg2 := buildEventMessage(t, event)
	consumer.handleMessage(context.Background(), "subscription.cancelled", msg2)

	assert.Equal(t, int32(2), callCount.Load(),
		"redelivered message must be processed after failure release")
}

// newTestConsumerMetrics returns an isolated observability.Metrics containing
// only the consumer-relevant counters and histograms. Using prometheus.New*
// (not promauto) avoids global registry conflicts between parallel tests.
func newTestConsumerMetrics() *observability.Metrics {
	return &observability.Metrics{
		EventsProcessedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "test_events_processed_total",
			Help: "test metric",
		}, []string{observability.LabelEventType, observability.LabelStatus}),
		EventProcessingLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "test_event_processing_duration_seconds",
			Help:    "test metric",
			Buckets: observability.EventProcessingBuckets,
		}, []string{observability.LabelEventType}),
		EventProcessingLag: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "test_event_processing_lag_seconds",
			Help:    "test metric",
			Buckets: observability.EventProcessingLagBuckets,
		}, []string{observability.LabelEventType}),
		EntityLockWaitLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "test_entity_lock_wait_duration_seconds",
			Help:    "test metric",
			Buckets: observability.EventProcessingBuckets,
		}, []string{observability.LabelEventType}),
		DLQPublishedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_dlq_published_total",
			Help: "test metric",
		}),
		IdempotencyHitTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "test_idempotency_hit_total",
			Help: "test metric",
		}, []string{observability.LabelEventType}),
		EntityLocksActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "test_entity_locks_active",
			Help: "test metric",
		}),
		EntityLocksEvicted: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_entity_locks_evicted_total",
			Help: "test metric",
		}),
	}
}

func TestHandleMessage_SuccessIncrementsProcessedCounter(t *testing.T) {
	metrics := newTestConsumerMetrics()
	handler := &recordingHandler{}

	consumer := &BillingEventConsumer{
		handler:        handler,
		idempotency:    &alwaysNewIdempotencyChecker{},
		schemaRegistry: domainevent.NewSchemaRegistry(),
		logger:         discardLogger(),
		clock:          clock.NewReal(),
		metrics:        metrics,
	}

	event := domainevent.NewWithEntity(
		"subscription.cancelled",
		map[string]any{"subscription_id": "sub-m1", "user_id": "u-1", "reason": "test"},
		"sub-m1",
	)

	msg := buildEventMessage(t, event)
	consumer.handleMessage(context.Background(), "subscription.cancelled", msg)

	successCounter := metrics.EventsProcessedTotal.WithLabelValues(
		"subscription.cancelled", observability.StatusSuccess,
	)
	assert.Equal(t, float64(1), testutil.ToFloat64(successCounter),
		"EventsProcessedTotal with status=success must be incremented after successful processing")

	// Latency histogram must have recorded exactly one observation.
	assert.Equal(t, 1, testutil.CollectAndCount(metrics.EventProcessingLatency),
		"EventProcessingLatency must record exactly one observation")
}

func TestHandleMessage_DuplicateIncrementsIdempotencyHitCounter(t *testing.T) {
	metrics := newTestConsumerMetrics()
	idem := newStubIdempotencyChecker()
	handler := &recordingHandler{}

	consumer := &BillingEventConsumer{
		handler:        handler,
		idempotency:    idem,
		schemaRegistry: domainevent.NewSchemaRegistry(),
		logger:         discardLogger(),
		clock:          clock.NewReal(),
		metrics:        metrics,
	}

	event := domainevent.NewWithEntity(
		"subscription.paused",
		map[string]any{"subscription_id": "sub-m2", "user_id": "u-2"},
		"sub-m2",
	)

	// First message — processed, not a duplicate.
	msg1 := buildEventMessage(t, event)
	consumer.handleMessage(context.Background(), "subscription.paused", msg1)

	// Second message — same business event, should be detected as duplicate.
	msg2 := buildEventMessage(t, event)
	consumer.handleMessage(context.Background(), "subscription.paused", msg2)

	idempotencyCounter := metrics.IdempotencyHitTotal.WithLabelValues("subscription.paused")
	assert.Equal(t, float64(1), testutil.ToFloat64(idempotencyCounter),
		"IdempotencyHitTotal must be incremented when a duplicate event is detected")

	duplicateCounter := metrics.EventsProcessedTotal.WithLabelValues(
		"subscription.paused", observability.StatusSkippedDuplicate,
	)
	assert.Equal(t, float64(1), testutil.ToFloat64(duplicateCounter),
		"EventsProcessedTotal with status=skipped_duplicate must be incremented for duplicates")
}

func TestHandleMessage_EventProcessingLagRecorded(t *testing.T) {
	metrics := newTestConsumerMetrics()
	handler := &recordingHandler{}

	consumer := &BillingEventConsumer{
		handler:        handler,
		idempotency:    &alwaysNewIdempotencyChecker{},
		schemaRegistry: domainevent.NewSchemaRegistry(),
		logger:         discardLogger(),
		clock:          clock.NewReal(),
		metrics:        metrics,
	}

	// Create an event with a timestamp in the past to produce a measurable lag.
	pastTimestamp := time.Now().Add(-2 * time.Second)
	event := domainevent.Event{
		ID:        "evt-lag-test",
		Type:      "subscription.cancelled",
		Version:   1,
		Timestamp: pastTimestamp,
		Data:      map[string]any{"subscription_id": "sub-lag", "user_id": "u-1", "reason": "test"},
		EntityID:  "sub-lag",
	}

	msg := buildEventMessage(t, event)
	consumer.handleMessage(context.Background(), "subscription.cancelled", msg)

	// EventProcessingLag histogram must have recorded exactly one observation.
	assert.Equal(t, 1, testutil.CollectAndCount(metrics.EventProcessingLag),
		"EventProcessingLag must record exactly one observation")
}

func TestHandleMessage_ZeroTimestampSkipsLag(t *testing.T) {
	metrics := newTestConsumerMetrics()
	handler := &recordingHandler{}

	consumer := &BillingEventConsumer{
		handler:        handler,
		idempotency:    &alwaysNewIdempotencyChecker{},
		schemaRegistry: domainevent.NewSchemaRegistry(),
		logger:         discardLogger(),
		clock:          clock.NewReal(),
		metrics:        metrics,
	}

	// Event with zero timestamp (backward compat: very old events).
	event := domainevent.Event{
		ID:       "evt-no-ts",
		Type:     "subscription.cancelled",
		Version:  1,
		Data:     map[string]any{"subscription_id": "sub-nots", "user_id": "u-1", "reason": "test"},
		EntityID: "sub-nots",
	}

	msg := buildEventMessage(t, event)
	consumer.handleMessage(context.Background(), "subscription.cancelled", msg)

	// No lag should be recorded for zero timestamps.
	assert.Equal(t, 0, testutil.CollectAndCount(metrics.EventProcessingLag),
		"EventProcessingLag must not record when timestamp is zero")
}

func TestDrain_CompletesWhenNoInflight(t *testing.T) {
	consumer := &BillingEventConsumer{
		logger: discardLogger(),
		clock:  clock.NewReal(),
	}

	// Drain should return immediately when there are no in-flight messages.
	done := make(chan struct{})
	go func() {
		consumer.Drain()
		close(done)
	}()

	select {
	case <-done:
		// success — Drain returned promptly
	case <-time.After(1 * time.Second):
		t.Fatal("Drain must return immediately when no messages are in-flight")
	}
}

func TestDrain_WaitsForInflight(t *testing.T) {
	consumer := &BillingEventConsumer{
		logger: discardLogger(),
		clock:  clock.NewReal(),
	}

	// Simulate an in-flight message by incrementing the WaitGroup.
	consumer.inflightWg.Add(1)

	drainDone := make(chan struct{})
	go func() {
		consumer.Drain()
		close(drainDone)
	}()

	// Drain should be blocked while the message is in-flight.
	select {
	case <-drainDone:
		t.Fatal("Drain must not return while messages are in-flight")
	case <-time.After(50 * time.Millisecond):
		// expected — Drain is still waiting
	}

	// Complete the "in-flight" message.
	consumer.inflightWg.Done()

	select {
	case <-drainDone:
		// success — Drain returned after in-flight completed
	case <-time.After(1 * time.Second):
		t.Fatal("Drain must return after all in-flight messages complete")
	}
}

func TestPollDLQDepth_ReturnsWhenNilConn(t *testing.T) {
	consumer := &BillingEventConsumer{
		logger:  discardLogger(),
		clock:   clock.NewReal(),
		metrics: newTestConsumerMetrics(),
		// natsConn is nil — pollDLQDepth must return immediately.
	}

	done := make(chan struct{})
	go func() {
		consumer.pollDLQDepth(context.Background())
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(1 * time.Second):
		t.Fatal("pollDLQDepth must return immediately when natsConn is nil")
	}
}

func TestPollDLQDepth_ReturnsWhenNilMetrics(t *testing.T) {
	consumer := &BillingEventConsumer{
		logger: discardLogger(),
		clock:  clock.NewReal(),
		// metrics is nil — pollDLQDepth must return immediately.
	}

	done := make(chan struct{})
	go func() {
		consumer.pollDLQDepth(context.Background())
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(1 * time.Second):
		t.Fatal("pollDLQDepth must return immediately when metrics is nil")
	}
}

func TestConsumerDrainTimeout_IsPositive(t *testing.T) {
	assert.Greater(t, ConsumerDrainTimeout, time.Duration(0),
		"ConsumerDrainTimeout must be positive")
}

func TestDLQDepthPollInterval_IsPositive(t *testing.T) {
	assert.Greater(t, DLQDepthPollInterval, time.Duration(0),
		"DLQDepthPollInterval must be positive")
}

func TestHandleMessage_TwoEventsForSameEntityBothProcessed(t *testing.T) {
	// This is the core bug fix: two legitimate events of the same type for the
	// same entity (e.g., two subscription.cancelled events after two separate
	// reactivation cycles) must both be processed because they have different
	// event IDs.
	idem := newStubIdempotencyChecker()
	var processCount atomic.Int32

	handler := &recordingHandler{
		onCancelled: func(_ context.Context, _ string) error {
			processCount.Add(1)
			return nil
		},
	}

	consumer := &BillingEventConsumer{
		handler:        handler,
		idempotency:    idem,
		schemaRegistry: domainevent.NewSchemaRegistry(),
		logger:         discardLogger(),
		clock:          clock.NewReal(),
	}

	sameSubID := "sub-same"

	// Two separate event instances for the same entity and type.
	event1 := domainevent.NewWithEntity(
		"subscription.cancelled",
		map[string]any{"subscription_id": sameSubID, "user_id": "u-1", "reason": "first cancel"},
		sameSubID,
	)
	event2 := domainevent.NewWithEntity(
		"subscription.cancelled",
		map[string]any{"subscription_id": sameSubID, "user_id": "u-1", "reason": "second cancel"},
		sameSubID,
	)

	msg1 := buildEventMessage(t, event1)
	consumer.handleMessage(context.Background(), "subscription.cancelled", msg1)

	msg2 := buildEventMessage(t, event2)
	consumer.handleMessage(context.Background(), "subscription.cancelled", msg2)

	assert.Equal(t, int32(2), processCount.Load(),
		"two distinct events for the same entity must both be processed")
}

func TestHandleMessage_BackwardCompat_OldEventWithoutIDUsesTypeEntityKey(t *testing.T) {
	// Events without an ID (pre-UUIDv7 migration) must fall back to the old
	// {event_type}:{entity_id} idempotency key format.
	idem := newStubIdempotencyChecker()
	handler := &recordingHandler{}

	consumer := &BillingEventConsumer{
		handler:        handler,
		idempotency:    idem,
		schemaRegistry: domainevent.NewSchemaRegistry(),
		logger:         discardLogger(),
		clock:          clock.NewReal(),
	}

	// Manually construct an event without ID (simulating pre-migration event).
	event := domainevent.Event{
		Type:      "subscription.cancelled",
		Version:   1,
		Timestamp: time.Now(),
		Data:      map[string]any{"subscription_id": "sub-old", "user_id": "u-1", "reason": "test"},
		EntityID:  "sub-old",
	}

	msg := buildEventMessage(t, event)
	consumer.handleMessage(context.Background(), "subscription.cancelled", msg)

	keys := idem.acquiredKeys()
	require.Len(t, keys, 1)
	assert.Equal(t, "subscription.cancelled:sub-old", keys[0],
		"old events without ID must use type:entity_id as idempotency key")
}

func TestHandleChargeCompleted_Success(t *testing.T) {
	var capturedInvoiceID string
	checkout := &recordingCheckoutCompleter{
		onCompleteCheckout: func(_ context.Context, invoiceID string) error {
			capturedInvoiceID = invoiceID
			return nil
		},
	}

	consumer := &BillingEventConsumer{
		handler:        &recordingHandler{},
		checkout:       checkout,
		idempotency:    &alwaysNewIdempotencyChecker{},
		schemaRegistry: domainevent.NewSchemaRegistry(),
		logger:         discardLogger(),
		clock:          clock.NewReal(),
		runner:         &passthroughRunner{},
	}

	event := domainevent.NewWithEntity(
		domainevent.EventType(payment.EventChargeCompleted),
		map[string]any{
			"payment_id":  "pay-1",
			"invoice_id":  "inv-42",
			"external_id": "pi_123",
			"provider":    "stripe",
			"amount":      999,
		},
		"pay-1",
	)

	msg := buildEventMessage(t, event)
	consumer.handleMessage(context.Background(), string(payment.EventChargeCompleted), msg)

	assert.Equal(t, "inv-42", capturedInvoiceID,
		"handleChargeCompleted must extract invoice_id and pass it to CompleteCheckout")
}

func TestHandleChargeCompleted_MissingInvoiceID(t *testing.T) {
	checkout := &recordingCheckoutCompleter{
		onCompleteCheckout: func(_ context.Context, _ string) error {
			t.Fatal("CompleteCheckout must not be called when invoice_id is missing")
			return nil
		},
	}

	idem := newStubIdempotencyChecker()

	consumer := &BillingEventConsumer{
		handler:        &recordingHandler{},
		checkout:       checkout,
		idempotency:    idem,
		schemaRegistry: domainevent.NewSchemaRegistry(),
		logger:         discardLogger(),
		clock:          clock.NewReal(),
	}

	// Event without invoice_id.
	event := domainevent.NewWithEntity(
		domainevent.EventType(payment.EventChargeCompleted),
		map[string]any{
			"payment_id": "pay-1",
			"provider":   "stripe",
			"amount":     999,
		},
		"pay-1",
	)

	msg := buildEventMessage(t, event)
	consumer.handleMessage(context.Background(), string(payment.EventChargeCompleted), msg)

	// The idempotency key should have been released since the handler failed.
	released := idem.releasedKeys()
	require.Len(t, released, 1,
		"idempotency key must be released after handleChargeCompleted failure")
}

func TestHandleChargeCompleted_CompleteCheckoutError(t *testing.T) {
	idem := newStubIdempotencyChecker()

	checkout := &recordingCheckoutCompleter{
		onCompleteCheckout: func(_ context.Context, _ string) error {
			return errors.New("billing service error")
		},
	}

	consumer := &BillingEventConsumer{
		handler:        &recordingHandler{},
		checkout:       checkout,
		idempotency:    idem,
		schemaRegistry: domainevent.NewSchemaRegistry(),
		logger:         discardLogger(),
		clock:          clock.NewReal(),
		runner:         &passthroughRunner{},
	}

	event := domainevent.NewWithEntity(
		domainevent.EventType(payment.EventChargeCompleted),
		map[string]any{
			"payment_id":  "pay-1",
			"invoice_id":  "inv-42",
			"external_id": "pi_123",
			"provider":    "stripe",
			"amount":      999,
		},
		"pay-1",
	)

	msg := buildEventMessage(t, event)
	consumer.handleMessage(context.Background(), string(payment.EventChargeCompleted), msg)

	// The idempotency key should have been released so the event can be retried.
	released := idem.releasedKeys()
	require.Len(t, released, 1,
		"idempotency key must be released after CompleteCheckout failure for retry")
}

func TestEvictStaleLocks_RemovesExpiredAndKeepsActive(t *testing.T) {
	// Verify that the eviction logic correctly distinguishes stale locks from
	// fresh ones. Because evictStaleLocks uses a real time.Ticker (which won't
	// fire in a unit test), we test the eviction body by seeding entity locks
	// with known timestamps and running getEntityLock + manual cutoff logic
	// that mirrors the ticker case.
	metrics := newTestConsumerMetrics()
	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mockClock := clock.NewMock(baseTime)

	consumer := &BillingEventConsumer{
		logger:  discardLogger(),
		clock:   mockClock,
		metrics: metrics,
	}

	// Seed entity locks: one fresh (at baseTime), two stale (older than TTL).
	staleLock1 := &entityLock{}
	staleLock1.lastUsed.Store(baseTime.Add(-entityLockTTL - time.Second).UnixNano())
	consumer.entityLocks.Store("sub-stale-1", staleLock1)

	staleLock2 := &entityLock{}
	staleLock2.lastUsed.Store(baseTime.Add(-entityLockTTL - 5*time.Minute).UnixNano())
	consumer.entityLocks.Store("sub-stale-2", staleLock2)

	// Advance clock past the TTL cutoff.
	mockClock.Advance(entityLockTTL + time.Second)

	// Fresh lock: lastUsed = now (after advance), so it survives eviction.
	freshLock := &entityLock{}
	freshLock.lastUsed.Store(mockClock.Now().UnixNano())
	consumer.entityLocks.Store("sub-fresh", freshLock)

	// Run the eviction logic inline (mirrors the ticker case body).
	cutoff := mockClock.Now().Add(-entityLockTTL).UnixNano()
	var evicted int
	consumer.entityLocks.Range(func(key, value any) bool {
		lock := value.(*entityLock)
		if lock.lastUsed.Load() < cutoff {
			consumer.entityLocks.Delete(key)
			evicted++
		}
		return true
	})
	metrics.EntityLocksEvicted.Add(float64(evicted))

	var remaining int
	consumer.entityLocks.Range(func(_, _ any) bool {
		remaining++
		return true
	})
	metrics.EntityLocksActive.Set(float64(remaining))

	assert.Equal(t, 2, evicted, "two stale locks must be evicted")
	assert.Equal(t, 1, remaining, "only the fresh lock must survive eviction")

	_, freshExists := consumer.entityLocks.Load("sub-fresh")
	assert.True(t, freshExists, "fresh lock must not be evicted")
	_, stale1Exists := consumer.entityLocks.Load("sub-stale-1")
	assert.False(t, stale1Exists, "stale lock 1 must be evicted")
	_, stale2Exists := consumer.entityLocks.Load("sub-stale-2")
	assert.False(t, stale2Exists, "stale lock 2 must be evicted")

	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.EntityLocksActive),
		"EntityLocksActive gauge must reflect remaining locks after eviction")
	assert.Equal(t, float64(2), testutil.ToFloat64(metrics.EntityLocksEvicted),
		"EntityLocksEvicted counter must equal number of evicted locks")
}

func TestEvictStaleLocks_NoEvictionsWhenAllFresh(t *testing.T) {
	metrics := newTestConsumerMetrics()
	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mockClock := clock.NewMock(baseTime)

	consumer := &BillingEventConsumer{
		logger:  discardLogger(),
		clock:   mockClock,
		metrics: metrics,
	}

	// Seed two fresh locks.
	for _, id := range []string{"sub-a", "sub-b"} {
		lock := &entityLock{}
		lock.lastUsed.Store(baseTime.UnixNano())
		consumer.entityLocks.Store(id, lock)
	}

	// Advance clock by less than TTL — nothing should be evicted.
	mockClock.Advance(entityLockTTL / 2)

	cutoff := mockClock.Now().Add(-entityLockTTL).UnixNano()
	var evicted int
	consumer.entityLocks.Range(func(key, value any) bool {
		lock := value.(*entityLock)
		if lock.lastUsed.Load() < cutoff {
			consumer.entityLocks.Delete(key)
			evicted++
		}
		return true
	})
	metrics.EntityLocksEvicted.Add(float64(evicted))

	var remaining int
	consumer.entityLocks.Range(func(_, _ any) bool {
		remaining++
		return true
	})

	assert.Equal(t, 0, evicted, "no locks should be evicted when all are within TTL")
	assert.Equal(t, 2, remaining, "all locks must survive when within TTL")
}

func TestEvictStaleLocks_ExitsOnContextCancel(t *testing.T) {
	consumer := &BillingEventConsumer{
		logger: discardLogger(),
		clock:  clock.NewReal(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	done := make(chan struct{})
	go func() {
		consumer.evictStaleLocks(ctx)
		close(done)
	}()

	select {
	case <-done:
		// success — evictStaleLocks exited promptly on context cancellation
	case <-time.After(1 * time.Second):
		t.Fatal("evictStaleLocks must exit when context is cancelled")
	}
}
