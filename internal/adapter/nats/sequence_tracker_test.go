package nats

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/observability"
)

// testSequenceGapMetric creates a fresh counter for sequence gap testing.
func testSequenceGapMetric(t *testing.T) (*observability.Metrics, prometheus.Counter) {
	t.Helper()
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: observability.MetricOutboxSequenceGaps,
		Help: "test",
	})
	return &observability.Metrics{OutboxSequenceGaps: counter}, counter
}

// seqGapCounterValue extracts the current value from the gap counter.
func seqGapCounterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	var m io_prometheus_client.Metric
	require.NoError(t, c.(prometheus.Metric).Write(&m))
	return m.GetCounter().GetValue()
}

func TestSequenceTracker_CheckAndUpdate(t *testing.T) {
	tests := []struct {
		name     string
		updates  []struct{ entityID string; seq int64 }
		wantGaps float64
	}{
		{
			name: "sequential events produce no gaps",
			updates: []struct{ entityID string; seq int64 }{
				{"sub-1", 1},
				{"sub-1", 2},
				{"sub-1", 3},
			},
			wantGaps: 0,
		},
		{
			name: "gap of 1 detected",
			updates: []struct{ entityID string; seq int64 }{
				{"sub-1", 1},
				{"sub-1", 3}, // gap: seq 2 missing
			},
			wantGaps: 1,
		},
		{
			name: "gap of 3 detected",
			updates: []struct{ entityID string; seq int64 }{
				{"sub-1", 10},
				{"sub-1", 14}, // gap: seq 11, 12, 13 missing
			},
			wantGaps: 1, // one gap event, not 3 individual gaps
		},
		{
			name: "different entities tracked independently",
			updates: []struct{ entityID string; seq int64 }{
				{"sub-1", 1},
				{"sub-2", 1},
				{"sub-1", 2},
				{"sub-2", 5}, // gap only for sub-2
			},
			wantGaps: 1,
		},
		{
			name: "zero sequence ignored",
			updates: []struct{ entityID string; seq int64 }{
				{"sub-1", 0},
				{"sub-1", 1},
			},
			wantGaps: 0,
		},
		{
			name: "negative sequence ignored",
			updates: []struct{ entityID string; seq int64 }{
				{"sub-1", -1},
				{"sub-1", 1},
			},
			wantGaps: 0,
		},
		{
			name: "empty entity ID ignored",
			updates: []struct{ entityID string; seq int64 }{
				{"", 1},
				{"", 5},
			},
			wantGaps: 0,
		},
		{
			name: "first event for entity never triggers gap",
			updates: []struct{ entityID string; seq int64 }{
				{"sub-1", 100},
			},
			wantGaps: 0,
		},
		{
			name: "duplicate sequence does not trigger gap",
			updates: []struct{ entityID string; seq int64 }{
				{"sub-1", 5},
				{"sub-1", 5}, // same as last — no gap
			},
			wantGaps: 0,
		},
		{
			name: "out of order lower sequence does not trigger gap",
			updates: []struct{ entityID string; seq int64 }{
				{"sub-1", 5},
				{"sub-1", 3}, // lower than last — no gap (just updates)
			},
			wantGaps: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics, counter := testSequenceGapMetric(t)
			tracker := newSequenceTracker(discardLogger(), metrics)

			for _, u := range tt.updates {
				tracker.checkAndUpdate(u.entityID, u.seq)
			}

			assert.Equal(t, tt.wantGaps, seqGapCounterValue(t, counter),
				"unexpected number of gap increments")
		})
	}
}

func TestSequenceTracker_NilMetrics(t *testing.T) {
	// Verify that checkAndUpdate does not panic with nil metrics.
	tracker := newSequenceTracker(discardLogger(), nil)

	assert.NotPanics(t, func() {
		tracker.checkAndUpdate("sub-1", 1)
		tracker.checkAndUpdate("sub-1", 5) // gap, but metrics is nil — should not panic
	})
}
