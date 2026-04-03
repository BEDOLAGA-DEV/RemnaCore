package nats

import (
	"context"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// MultiSubEventPublisher adapts the EventPublisher to the
// domainevent.Publisher interface, routing multisub domain events to the
// appropriate NATS topics.
type MultiSubEventPublisher struct {
	publisher *EventPublisher
}

// NewMultiSubEventPublisher creates a MultiSubEventPublisher backed by the
// given NATS EventPublisher.
func NewMultiSubEventPublisher(publisher *EventPublisher) *MultiSubEventPublisher {
	return &MultiSubEventPublisher{publisher: publisher}
}

// Publish routes a domain event to the correct NATS topic based on its type.
func (p *MultiSubEventPublisher) Publish(ctx context.Context, event domainevent.Event) error {
	topic := string(event.Type)
	return p.publisher.Publish(ctx, topic, event)
}

// multiSubEventTopics returns the NATS subjects that carry multisub domain
// events. Used by subscribers to know which topics to listen to.
func multiSubEventTopics() []string {
	return []string{
		string(multisub.EventBindingProvisioned),
		string(multisub.EventBindingDeprovisioned),
		string(multisub.EventBindingSyncFailed),
		string(multisub.EventBindingSyncCompleted),
		string(multisub.EventBindingTrafficExceeded),
	}
}
