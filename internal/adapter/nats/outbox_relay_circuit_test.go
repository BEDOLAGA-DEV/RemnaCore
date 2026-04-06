package nats

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/sony/gobreaker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- test doubles ---

// stubTxRunner implements txmanager.Runner for testing by executing the
// function directly without a real database transaction.
type stubTxRunner struct {
	called int
}

func (s *stubTxRunner) RunInTx(_ context.Context, fn func(ctx context.Context) error) error {
	s.called++
	return fn(context.Background())
}

// errNATSDown is a sentinel error simulating a NATS connection failure.
var errNATSDown = errors.New("nats: connection refused")

// --- circuit breaker tests ---

func TestCircuitBreaker_TripsAfterConsecutiveFailures(t *testing.T) {
	// Verify that the circuit breaker transitions to open after
	// relayCBConsecutiveFailures consecutive failures, matching the
	// production configuration.
	cb := gobreaker.NewCircuitBreaker[struct{}](gobreaker.Settings{
		Name:        relayCBName,
		MaxRequests: relayCBMaxRequests,
		Interval:    0,
		Timeout:     relayCBTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= relayCBConsecutiveFailures
		},
	})

	// Record consecutive failures up to the threshold.
	for i := range relayCBConsecutiveFailures {
		t.Run(fmt.Sprintf("failure_%d", i+1), func(t *testing.T) {
			_, err := cb.Execute(func() (struct{}, error) {
				return struct{}{}, errNATSDown
			})
			require.Error(t, err)
		})
	}

	// After exactly relayCBConsecutiveFailures failures, the breaker must be open.
	assert.Equal(t, gobreaker.StateOpen, cb.State(),
		"breaker should be open after %d consecutive failures", relayCBConsecutiveFailures)

	// Subsequent Execute calls must fail immediately without calling the function.
	functionCalled := false
	_, err := cb.Execute(func() (struct{}, error) {
		functionCalled = true
		return struct{}{}, nil
	})
	assert.Error(t, err)
	assert.False(t, functionCalled, "function must not be called when breaker is open")
}

func TestCircuitBreaker_SuccessResetsConsecutiveFailures(t *testing.T) {
	cb := gobreaker.NewCircuitBreaker[struct{}](gobreaker.Settings{
		Name:        relayCBName,
		MaxRequests: relayCBMaxRequests,
		Interval:    0,
		Timeout:     relayCBTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= relayCBConsecutiveFailures
		},
	})

	// Record failures just below the threshold.
	for range relayCBConsecutiveFailures - 1 {
		_, _ = cb.Execute(func() (struct{}, error) {
			return struct{}{}, errNATSDown
		})
	}

	// One success should reset the consecutive failure count.
	_, err := cb.Execute(func() (struct{}, error) {
		return struct{}{}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, gobreaker.StateClosed, cb.State(),
		"breaker should remain closed after a success resets consecutive failures")

	// Now the same number of failures should be needed again to trip.
	for range relayCBConsecutiveFailures - 1 {
		_, _ = cb.Execute(func() (struct{}, error) {
			return struct{}{}, errNATSDown
		})
	}
	assert.Equal(t, gobreaker.StateClosed, cb.State(),
		"breaker should still be closed (one fewer than threshold)")
}

func TestRelay_SkipsDBPollWhenCircuitOpen(t *testing.T) {
	// Verify that relay() returns 0 immediately without calling txRunner
	// when the NATS circuit breaker is open.
	txRunner := &stubTxRunner{}

	relay := &OutboxRelay{
		outbox:      nil, // Not called because breaker skips DB poll.
		publisher:   nil, // Not called because breaker skips DB poll.
		txRunner:    txRunner,
		logger:      discardLogger(),
		workerCount: MinOutboxRelayWorkers,
		natsBreaker: gobreaker.NewCircuitBreaker[struct{}](gobreaker.Settings{
			Name:        relayCBName,
			MaxRequests: relayCBMaxRequests,
			Interval:    0,
			Timeout:     relayCBTimeout,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				return counts.ConsecutiveFailures >= relayCBConsecutiveFailures
			},
		}),
	}

	// Trip the breaker by recording enough failures.
	for range relayCBConsecutiveFailures {
		_, _ = relay.natsBreaker.Execute(func() (struct{}, error) {
			return struct{}{}, errNATSDown
		})
	}
	require.Equal(t, gobreaker.StateOpen, relay.natsBreaker.State())

	// relay() should return 0 without touching txRunner.
	published := relay.relay(context.Background(), discardLogger(), "0")

	assert.Equal(t, 0, published, "relay must return 0 when breaker is open")
	assert.Equal(t, 0, txRunner.called, "txRunner must not be called when breaker is open")
}

func TestNewOutboxRelay_InitializesCircuitBreaker(t *testing.T) {
	relay := NewOutboxRelay(nil, nil, nil, discardLogger(), MinOutboxRelayWorkers, nil)
	require.NotNil(t, relay.natsBreaker, "circuit breaker must be initialized")
	assert.Equal(t, gobreaker.StateClosed, relay.natsBreaker.State(),
		"circuit breaker should start in closed state")
}

// --- circuit breaker constants tests ---

func TestCircuitBreakerConstants(t *testing.T) {
	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "name is non-empty",
			check: func(t *testing.T) {
				assert.NotEmpty(t, relayCBName)
			},
		},
		{
			name: "max requests is positive",
			check: func(t *testing.T) {
				assert.Greater(t, int(relayCBMaxRequests), 0)
			},
		},
		{
			name: "timeout is positive",
			check: func(t *testing.T) {
				assert.Greater(t, relayCBTimeout, time.Duration(0))
			},
		},
		{
			name: "consecutive failures threshold is positive",
			check: func(t *testing.T) {
				assert.Greater(t, int(relayCBConsecutiveFailures), 0)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.check)
	}
}

// --- JetStream dedup tests ---

// These tests verify that the outbox event ID flows through to the Watermill
// message UUID, which Watermill's NATS publisher maps to the Nats-Msg-Id
// header (via TrackMsgId: true) for JetStream server-side deduplication.

func TestPublishWithID_MessageUUIDMatchesEventID(t *testing.T) {
	// PublishWithID calls message.NewMessage(id, payload). The Watermill
	// NATS publisher then maps msg.UUID to the Nats-Msg-Id header when
	// TrackMsgId is true. We verify the contract: message.NewMessage
	// preserves the caller-supplied ID as the message UUID.
	tests := []struct {
		name    string
		eventID string
		payload []byte
	}{
		{
			name:    "standard UUID",
			eventID: "550e8400-e29b-41d4-a716-446655440000",
			payload: []byte(`{"type":"subscription.activated"}`),
		},
		{
			name:    "short ID",
			eventID: "evt-001",
			payload: []byte(`{}`),
		},
		{
			name:    "empty payload",
			eventID: "9f86d081-884c-7d65-9a2f-ea0be46e9753",
			payload: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate the exact call from PublishWithID to verify the contract.
			msg := message.NewMessage(tt.eventID, tt.payload)
			assert.Equal(t, tt.eventID, msg.UUID,
				"Watermill message UUID must equal the outbox event ID for JetStream dedup")
			assert.Equal(t, tt.payload, []byte(msg.Payload))
		})
	}
}

func TestPublishWithID_DeterministicDedup(t *testing.T) {
	// When the same outbox event is published twice (e.g. after a tx
	// rollback), both messages must have the same UUID so JetStream
	// deduplicates the second one.
	eventID := "00000000-0000-0000-0000-000000000001"
	payload := []byte(`{"type":"billing.invoice_paid"}`)

	msg1 := message.NewMessage(eventID, payload)
	msg2 := message.NewMessage(eventID, payload)

	assert.Equal(t, msg1.UUID, msg2.UUID,
		"re-publishing the same event must produce the same Nats-Msg-Id for JetStream dedup")
}
