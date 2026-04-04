package payment

import "github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"

// ChargeCreatedPayload is the typed payload for EventChargeCreated.
type ChargeCreatedPayload struct {
	PaymentID  string `json:"payment_id"`
	InvoiceID  string `json:"invoice_id"`
	Provider   string `json:"provider"`
	ExternalID string `json:"external_id"`
	Amount     int64  `json:"amount"`
}

// ChargeCompletedPayload is the typed payload for EventChargeCompleted.
type ChargeCompletedPayload struct {
	PaymentID string `json:"payment_id"`
	InvoiceID string `json:"invoice_id"`
	Provider  string `json:"provider"`
	Amount    int64  `json:"amount"`
}

// ChargeFailedPayload is the typed payload for EventChargeFailed.
type ChargeFailedPayload struct {
	PaymentID string `json:"payment_id"`
	InvoiceID string `json:"invoice_id"`
	Provider  string `json:"provider"`
	Reason    string `json:"reason"`
}

// RefundCompletedPayload is the typed payload for EventRefundCompleted.
type RefundCompletedPayload struct {
	PaymentID string `json:"payment_id"`
	InvoiceID string `json:"invoice_id"`
	Provider  string `json:"provider"`
	Amount    int64  `json:"amount"`
}

// WebhookReceivedPayload is the typed payload for EventWebhookReceived.
type WebhookReceivedPayload struct {
	Provider   string `json:"provider"`
	ExternalID string `json:"external_id"`
	Status     string `json:"status"`
}

// --- EventPayload interface implementations ---

func (ChargeCreatedPayload) EventType() domainevent.EventType   { return EventChargeCreated }
func (ChargeCompletedPayload) EventType() domainevent.EventType { return EventChargeCompleted }
func (ChargeFailedPayload) EventType() domainevent.EventType    { return EventChargeFailed }
func (RefundCompletedPayload) EventType() domainevent.EventType { return EventRefundCompleted }
func (WebhookReceivedPayload) EventType() domainevent.EventType { return EventWebhookReceived }

// Compile-time interface check.
var _ domainevent.EventPayload = ChargeCreatedPayload{}
