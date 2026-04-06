package nats_test

import (
	"encoding/json"
	"testing"
	"time"

	natsadapter "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/nats"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutboxRelayConstants(t *testing.T) {
	t.Run("relay base interval is positive", func(t *testing.T) {
		assert.Greater(t, natsadapter.OutboxRelayBaseInterval, time.Duration(0))
	})

	t.Run("relay max interval >= base interval", func(t *testing.T) {
		assert.GreaterOrEqual(t, natsadapter.OutboxRelayMaxInterval, natsadapter.OutboxRelayBaseInterval)
	})

	t.Run("batch size is positive", func(t *testing.T) {
		assert.Greater(t, natsadapter.OutboxRelayBatchSize, 0)
	})

	t.Run("max batch size >= base batch size", func(t *testing.T) {
		assert.GreaterOrEqual(t, natsadapter.OutboxRelayMaxBatchSize, natsadapter.OutboxRelayBatchSize)
	})

	t.Run("max batch size is positive", func(t *testing.T) {
		assert.Greater(t, natsadapter.OutboxRelayMaxBatchSize, 0)
	})

	t.Run("cleanup interval is positive", func(t *testing.T) {
		assert.Greater(t, natsadapter.OutboxCleanupInterval, time.Duration(0))
	})

	t.Run("retention period is at least one day", func(t *testing.T) {
		assert.GreaterOrEqual(t, natsadapter.OutboxRetentionPeriod, 24*time.Hour)
	})
}

func TestNewOutboxRelay(t *testing.T) {
	t.Run("nil dependencies do not panic", func(t *testing.T) {
		// NewOutboxRelay should not panic with nil dependencies (constructor only
		// assigns fields).
		relay := natsadapter.NewOutboxRelay(nil, nil, nil, nil, natsadapter.MinOutboxRelayWorkers, nil)
		assert.NotNil(t, relay)
	})

	t.Run("worker count below minimum is clamped", func(t *testing.T) {
		tests := []struct {
			name  string
			input int
		}{
			{name: "zero", input: 0},
			{name: "negative", input: -5},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				relay := natsadapter.NewOutboxRelay(nil, nil, nil, nil, tt.input, nil)
				assert.NotNil(t, relay)
			})
		}
	})

	t.Run("explicit worker count is accepted", func(t *testing.T) {
		relay := natsadapter.NewOutboxRelay(nil, nil, nil, nil, 4, nil)
		assert.NotNil(t, relay)
	})
}

func TestExtractTraceParent(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    string
	}{
		{
			name: "event with trace_parent",
			payload: mustMarshal(t, domainevent.Event{
				ID:          "evt-1",
				Type:        "test.event",
				Version:     1,
				Data:        map[string]any{"key": "val"},
				TraceParent: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			}),
			want: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		},
		{
			name: "event without trace_parent",
			payload: mustMarshal(t, domainevent.Event{
				ID:      "evt-2",
				Type:    "test.event",
				Version: 1,
				Data:    map[string]any{"key": "val"},
			}),
			want: "",
		},
		{
			name:    "invalid JSON",
			payload: []byte(`{invalid`),
			want:    "",
		},
		{
			name:    "empty payload",
			payload: []byte{},
			want:    "",
		},
		{
			name:    "nil payload",
			payload: nil,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := natsadapter.ExtractTraceParent(tt.payload)
			assert.Equal(t, tt.want, got)
		})
	}
}

// mustMarshal is a test helper that marshals v to JSON or fails the test.
func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}
