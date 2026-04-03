package nats

import (
	"context"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
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

// billingEventTopics returns the NATS subjects that carry billing domain events.
// Used by subscribers to know which topics to listen to.
func billingEventTopics() []string {
	return []string{
		string(billing.EventInvoiceCreated),
		string(billing.EventInvoicePaid),
		string(billing.EventInvoiceFailed),
		string(billing.EventInvoiceRefunded),
		string(billing.EventSubCreated),
		string(billing.EventSubActivated),
		string(billing.EventSubCancelled),
		string(billing.EventSubRenewed),
		string(billing.EventSubUpgraded),
		string(billing.EventSubDowngraded),
		string(billing.EventSubTrialStarted),
		string(billing.EventSubTrialEnding),
		string(billing.EventSubPaused),
		string(billing.EventSubResumed),
		string(billing.EventFamilyMemberAdded),
		string(billing.EventFamilyMemberRemoved),
	}
}
