package payment

import (
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

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
func NewChargeCreatedEvent(paymentID, invoiceID, provider, externalID string, amount int64, now time.Time) Event {
	return domainevent.NewAtWithEntity(EventChargeCreated, ChargeCreatedPayload{
		PaymentID:  paymentID,
		InvoiceID:  invoiceID,
		Provider:   provider,
		ExternalID: externalID,
		Amount:     amount,
	}, now, paymentID)
}

// NewChargeCompletedEvent creates an event for a successfully completed payment.
func NewChargeCompletedEvent(paymentID, invoiceID, provider string, amount int64, now time.Time) Event {
	return domainevent.NewAtWithEntity(EventChargeCompleted, ChargeCompletedPayload{
		PaymentID: paymentID,
		InvoiceID: invoiceID,
		Provider:  provider,
		Amount:    amount,
	}, now, paymentID)
}

// NewChargeFailedEvent creates an event for a failed payment charge.
func NewChargeFailedEvent(paymentID, invoiceID, provider, reason string, now time.Time) Event {
	return domainevent.NewAtWithEntity(EventChargeFailed, ChargeFailedPayload{
		PaymentID: paymentID,
		InvoiceID: invoiceID,
		Provider:  provider,
		Reason:    reason,
	}, now, paymentID)
}

// NewRefundCompletedEvent creates an event for a completed refund.
func NewRefundCompletedEvent(paymentID, invoiceID, provider string, amount int64, now time.Time) Event {
	return domainevent.NewAtWithEntity(EventRefundCompleted, RefundCompletedPayload{
		PaymentID: paymentID,
		InvoiceID: invoiceID,
		Provider:  provider,
		Amount:    amount,
	}, now, paymentID)
}

// NewWebhookReceivedEvent creates an event for a received payment webhook.
// Webhook events use the external provider ID as the entity since they
// originate outside the platform and have no internal aggregate ID.
func NewWebhookReceivedEvent(provider, externalID, status string, now time.Time) Event {
	return domainevent.NewAtWithEntity(EventWebhookReceived, WebhookReceivedPayload{
		Provider:   provider,
		ExternalID: externalID,
		Status:     status,
	}, now, externalID)
}
