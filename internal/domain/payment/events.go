package payment

import "github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"

// Payment-specific event types.
const (
	EventChargeCreated   domainevent.EventType = "payment.charge_created"
	EventChargeCompleted domainevent.EventType = "payment.charge_completed"
	EventChargeFailed    domainevent.EventType = "payment.charge_failed"
	EventRefundCompleted domainevent.EventType = "payment.refund_completed"
	EventWebhookReceived domainevent.EventType = "payment.webhook_received"
)

// Event is an alias for the shared domainevent.Event so that callers within the
// payment context can reference payment.Event without importing pkg/domainevent.
type Event = domainevent.Event

// NewChargeCreatedEvent creates an event for a newly created payment charge.
func NewChargeCreatedEvent(paymentID, invoiceID, provider, externalID string, amount int64) Event {
	return domainevent.New(EventChargeCreated, map[string]any{
		"payment_id":  paymentID,
		"invoice_id":  invoiceID,
		"provider":    provider,
		"external_id": externalID,
		"amount":      amount,
	})
}

// NewChargeCompletedEvent creates an event for a successfully completed payment.
func NewChargeCompletedEvent(paymentID, invoiceID, provider string, amount int64) Event {
	return domainevent.New(EventChargeCompleted, map[string]any{
		"payment_id": paymentID,
		"invoice_id": invoiceID,
		"provider":   provider,
		"amount":     amount,
	})
}

// NewChargeFailedEvent creates an event for a failed payment charge.
func NewChargeFailedEvent(paymentID, invoiceID, provider, reason string) Event {
	return domainevent.New(EventChargeFailed, map[string]any{
		"payment_id": paymentID,
		"invoice_id": invoiceID,
		"provider":   provider,
		"reason":     reason,
	})
}

// NewRefundCompletedEvent creates an event for a completed refund.
func NewRefundCompletedEvent(paymentID, invoiceID, provider string, amount int64) Event {
	return domainevent.New(EventRefundCompleted, map[string]any{
		"payment_id": paymentID,
		"invoice_id": invoiceID,
		"provider":   provider,
		"amount":     amount,
	})
}

// NewWebhookReceivedEvent creates an event for a received payment webhook.
func NewWebhookReceivedEvent(provider, externalID, status string) Event {
	return domainevent.New(EventWebhookReceived, map[string]any{
		"provider":    provider,
		"external_id": externalID,
		"status":      status,
	})
}
