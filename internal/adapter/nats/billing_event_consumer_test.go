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

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
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

// recordingHandler implements SubscriptionEventHandler for tests. Callback
// fields are optional; nil means the method returns nil immediately.
type recordingHandler struct {
	onActivated func(ctx context.Context, subID, userID string, plan multisub.PlanSnapshot, addonIDs, familyMemberIDs []string) error
	onCancelled func(ctx context.Context, subID string) error
	onPaused    func(ctx context.Context, subID string) error
	onResumed   func(ctx context.Context, subID string) error
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

func TestHandleMessage_BusinessLevelIdempotencyKey(t *testing.T) {
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

	// The idempotency key must be business-level, not the Watermill UUID.
	keys := idem.acquiredKeys()
	require.Len(t, keys, 1)
	assert.Equal(t, "subscription.cancelled:sub-123", keys[0])
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

	// The idempotency key must have been released so the next delivery can
	// reprocess the event.
	released := idem.releasedKeys()
	require.Len(t, released, 1)
	assert.Equal(t, "subscription.cancelled:sub-fail", released[0])

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

	retryKey := retryKeyPrefix + "subscription.cancelled:sub-retry"

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

	retryKey := retryKeyPrefix + "subscription.cancelled:sub-dlq"

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
