package observability

import (
	"log/slog"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPgStatCollector_Describe_ReturnsAllDescriptors(t *testing.T) {
	collector := NewPgStatCollector(nil, slog.Default())

	ch := make(chan *prometheus.Desc, 10)
	collector.Describe(ch)
	close(ch)

	var descs []*prometheus.Desc
	for desc := range ch {
		descs = append(descs, desc)
	}

	expectedCount := 3
	require.Len(t, descs, expectedCount, "expected %d metric descriptors", expectedCount)
}

func TestPgStatCollector_Describe_ContainsExpectedMetrics(t *testing.T) {
	collector := NewPgStatCollector(nil, slog.Default())

	ch := make(chan *prometheus.Desc, 10)
	collector.Describe(ch)
	close(ch)

	var descriptions []string
	for desc := range ch {
		descriptions = append(descriptions, desc.String())
	}

	tests := []struct {
		name       string
		metricName string
	}{
		{name: "total_time", metricName: MetricPgStatQueryTotalTime},
		{name: "calls", metricName: MetricPgStatQueryCalls},
		{name: "mean_time", metricName: MetricPgStatQueryMeanTime},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found := false
			for _, desc := range descriptions {
				if assert.Condition(t, func() bool {
					return len(desc) > 0
				}) {
					// prometheus.Desc.String() includes the fqName.
					if contains(desc, tt.metricName) {
						found = true
						break
					}
				}
			}
			assert.True(t, found, "expected descriptor for %s", tt.metricName)
		})
	}
}

func TestPgStatCollector_Collect_NilPool_NoMetrics(t *testing.T) {
	// With a nil pool, Collect should gracefully handle the error and emit
	// no metrics rather than panicking.
	collector := NewPgStatCollector(nil, slog.Default())

	ch := make(chan prometheus.Metric, 10)
	assert.NotPanics(t, func() {
		collector.Collect(ch)
	})
	close(ch)

	var metrics []prometheus.Metric
	for m := range ch {
		metrics = append(metrics, m)
	}

	assert.Empty(t, metrics, "expected no metrics when pool is nil")
}

func TestPgStatCollector_ImplementsCollectorInterface(t *testing.T) {
	collector := NewPgStatCollector(nil, slog.Default())
	var _ prometheus.Collector = collector
	assert.NotNil(t, collector)
}

// contains checks if substr is in s. Extracted as a helper to avoid importing
// strings in a test file for a single use.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := range len(s) - len(substr) + 1 {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
