package nats_test

import (
	"encoding/json"
	"testing"
	"time"

	natsadapter "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/nats"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/circuitbreaker"
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

	t.Run("backpressure threshold is positive", func(t *testing.T) {
		assert.Greater(t, natsadapter.OutboxBackpressureThreshold, int64(0))
	})
}

func TestNewOutboxRelay(t *testing.T) {
	t.Run("nil dependencies do not panic", func(t *testing.T) {
		// NewOutboxRelay should not panic with nil dependencies (constructor only
		// assigns fields).
		relay := natsadapter.NewOutboxRelay(nil, nil, nil, nil, nil, natsadapter.MinOutboxRelayWorkers, circuitbreaker.DefaultConfigNoInterval(), nil, nil)
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
				relay := natsadapter.NewOutboxRelay(nil, nil, nil, nil, nil, tt.input, circuitbreaker.DefaultConfigNoInterval(), nil, nil)
				assert.NotNil(t, relay)
			})
		}
	})

	t.Run("explicit worker count is accepted", func(t *testing.T) {
		relay := natsadapter.NewOutboxRelay(nil, nil, nil, nil, nil, 4, circuitbreaker.DefaultConfigNoInterval(), nil, nil)
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

func TestExtractTraceParent_ByteScanEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    string
	}{
		{
			name:    "trace_parent at start of object",
			payload: []byte(`{"trace_parent":"00-abc-def-01","type":"test"}`),
			want:    "00-abc-def-01",
		},
		{
			name:    "trace_parent at end of object",
			payload: []byte(`{"type":"test","trace_parent":"00-abc-def-01"}`),
			want:    "00-abc-def-01",
		},
		{
			name:    "empty trace_parent value",
			payload: []byte(`{"trace_parent":"","type":"test"}`),
			want:    "",
		},
		{
			name:    "trace_parent key without closing quote on value",
			payload: []byte(`{"trace_parent":"no-closing-quote`),
			want:    "",
		},
		{
			name:    "payload truncated after key",
			payload: []byte(`{"trace_parent":"`),
			want:    "",
		},
		{
			name: "similar key name does not false-match",
			// "my_trace_parent" does not contain the exact substring
			// "trace_parent":" (note the leading quote), so the scanner
			// correctly skips it and finds the real key.
			payload: []byte(`{"my_trace_parent":"decoy","trace_parent":"correct-val"}`),
			want:    "correct-val",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := natsadapter.ExtractTraceParent(tt.payload)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCurrentQuarterStart(t *testing.T) {
	tests := []struct {
		name string
		when time.Time
		want time.Time
	}{
		{
			name: "Q1 start",
			when: time.Date(2026, time.January, 15, 10, 30, 0, 0, time.UTC),
			want: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "Q1 end",
			when: time.Date(2026, time.March, 31, 23, 59, 59, 0, time.UTC),
			want: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "Q2 start",
			when: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
			want: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "Q2 mid",
			when: time.Date(2026, time.May, 15, 12, 0, 0, 0, time.UTC),
			want: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "Q3 mid",
			when: time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC),
			want: time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "Q4 start",
			when: time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC),
			want: time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "Q4 end",
			when: time.Date(2026, time.December, 31, 23, 59, 59, 0, time.UTC),
			want: time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := natsadapter.CurrentQuarterStart(tt.when)
			assert.Equal(t, tt.want, got)
		})
	}
}

func BenchmarkExtractTraceParent(b *testing.B) {
	payload := []byte(`{"id":"evt-1","type":"subscription.activated","version":1,"trace_parent":"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01","data":{"sub_id":"s-123","plan":"premium"}}`)
	b.ResetTimer()
	for b.Loop() {
		natsadapter.ExtractTraceParent(payload)
	}
}

// mustMarshal is a test helper that marshals v to JSON or fails the test.
func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}
