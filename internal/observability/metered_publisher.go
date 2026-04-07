package observability

import (
	"context"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// MeteredPublisher wraps a domainevent.Publisher and increments a Prometheus
// counter on every publish failure. Domain services remain unaware of metrics
// because they depend only on the domainevent.Publisher interface.
type MeteredPublisher struct {
	inner   domainevent.Publisher
	metrics *Metrics
}

// NewMeteredPublisher creates a MeteredPublisher that delegates to inner and
// records failures via metrics.EventPublishFailures.
func NewMeteredPublisher(inner domainevent.Publisher, metrics *Metrics) *MeteredPublisher {
	return &MeteredPublisher{inner: inner, metrics: metrics}
}

// Publish delegates to the inner publisher. On error, it increments the
// EventPublishFailures counter with the event type as a label and returns
// the original error unchanged.
func (p *MeteredPublisher) Publish(ctx context.Context, event domainevent.Event) error {
	err := p.inner.Publish(ctx, event)
	if err != nil {
		p.metrics.EventPublishFailures.WithLabelValues(string(event.Type)).Inc()
	}
	return err
}

// PublishBatch delegates to the inner publisher. On error, it increments the
// EventPublishFailures counter once per event in the batch (all share the same
// failure) and returns the original error unchanged.
//
// PublishBatch increments failure counters for ALL events if the batch fails.
// This assumes all-or-nothing semantics (correct for outbox UNNEST INSERT).
// For sequential publishers, this may overcount — acceptable for alerting purposes.
func (p *MeteredPublisher) PublishBatch(ctx context.Context, events []domainevent.Event) error {
	err := p.inner.PublishBatch(ctx, events)
	if err != nil {
		for _, event := range events {
			p.metrics.EventPublishFailures.WithLabelValues(string(event.Type)).Inc()
		}
	}
	return err
}
