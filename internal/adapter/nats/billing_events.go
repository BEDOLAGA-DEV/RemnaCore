package nats

import (
	"context"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// BillingEventPublisher adapts the EventPublisher to the domainevent.Publisher
// interface, routing billing domain events to the appropriate NATS topics.
type BillingEventPublisher struct {
	publisher *EventPublisher
}

// NewBillingEventPublisher creates a BillingEventPublisher backed by the given
// NATS EventPublisher.
func NewBillingEventPublisher(publisher *EventPublisher) *BillingEventPublisher {
	return &BillingEventPublisher{publisher: publisher}
}

// Publish routes a domain event to the correct NATS topic based on its type.
// The topic is derived directly from the event type string (e.g.
// "invoice.created" -> NATS subject "invoice.created").
func (p *BillingEventPublisher) Publish(ctx context.Context, event domainevent.Event) error {
	topic := string(event.Type)
	return p.publisher.Publish(ctx, topic, event)
}

// PublishBatch publishes events sequentially to NATS. This is used for
// direct NATS publish (plugin async, internal notifications) — NOT for
// domain events which go through the transactional outbox.
// Partial publish is possible on error; callers should treat this as
// best-effort notification, not guaranteed delivery.
func (p *BillingEventPublisher) PublishBatch(ctx context.Context, events []domainevent.Event) error {
	for _, event := range events {
		if err := p.Publish(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

