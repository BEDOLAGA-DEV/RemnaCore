package observability

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// stubPublisher is a test double that returns a fixed error.
type stubPublisher struct {
	err error
}

func (s *stubPublisher) Publish(_ context.Context, _ domainevent.Event) error {
	return s.err
}

func newTestMetrics() *Metrics {
	return &Metrics{
		EventPublishFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "test_event_publish_failures_total",
			Help: "test metric",
		}, []string{LabelEventType}),
	}
}

func TestMeteredPublisher_Success_NoIncrement(t *testing.T) {
	inner := &stubPublisher{err: nil}
	metrics := newTestMetrics()
	mp := NewMeteredPublisher(inner, metrics)

	event := domainevent.New("user.registered", map[string]any{"user_id": "u-1"})
	err := mp.Publish(context.Background(), event)

	require.NoError(t, err)

	// Counter must not have been incremented — collect and check.
	counter := metrics.EventPublishFailures.WithLabelValues("user.registered")
	assert.Equal(t, float64(0), testutil.ToFloat64(counter))
}

func TestMeteredPublisher_Failure_IncrementsCounter(t *testing.T) {
	publishErr := errors.New("nats connection lost")
	inner := &stubPublisher{err: publishErr}
	metrics := newTestMetrics()
	mp := NewMeteredPublisher(inner, metrics)

	event := domainevent.New("subscription.activated", map[string]any{"sub_id": "s-1"})
	err := mp.Publish(context.Background(), event)

	require.ErrorIs(t, err, publishErr)
	assert.Equal(t, float64(1), testutil.ToFloat64(
		metrics.EventPublishFailures.WithLabelValues("subscription.activated"),
	))
}

func TestMeteredPublisher_MultipleFailures_AccumulatesCount(t *testing.T) {
	inner := &stubPublisher{err: errors.New("timeout")}
	metrics := newTestMetrics()
	mp := NewMeteredPublisher(inner, metrics)

	failureCount := 5
	for range failureCount {
		event := domainevent.New("invoice.paid", map[string]any{})
		_ = mp.Publish(context.Background(), event)
	}

	assert.Equal(t, float64(failureCount), testutil.ToFloat64(
		metrics.EventPublishFailures.WithLabelValues("invoice.paid"),
	))
}

func TestMeteredPublisher_SatisfiesPublisherInterface(t *testing.T) {
	inner := &stubPublisher{}
	metrics := newTestMetrics()
	var pub domainevent.Publisher = NewMeteredPublisher(inner, metrics)
	assert.NotNil(t, pub)
}
