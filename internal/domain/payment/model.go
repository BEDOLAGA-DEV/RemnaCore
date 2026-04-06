package payment

import (
	"time"

	"github.com/google/uuid"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// PaymentStatus represents the current state of a payment record.
type PaymentStatus string

const (
	PaymentPending   PaymentStatus = "pending"
	PaymentCompleted PaymentStatus = "completed"
	PaymentFailed    PaymentStatus = "failed"
	PaymentRefunded  PaymentStatus = "refunded"
)

// WebhookStatus represents the processing state of a webhook log entry.
type WebhookStatus string

const (
	WebhookPending   WebhookStatus = "pending"
	WebhookProcessed WebhookStatus = "processed"
	WebhookFailed    WebhookStatus = "failed"
	WebhookDuplicate WebhookStatus = "duplicate"
)

// PaymentRecord represents a payment attempt against an invoice.
// It embeds EventRecorder to accumulate domain events during mutations.
// Services must call DomainEvents() after persisting the aggregate to
// retrieve and publish all pending events.
type PaymentRecord struct {
	domainevent.EventRecorder

	ID         string
	InvoiceID  string
	Provider   string
	ExternalID string
	Amount     int64
	Currency   string
	Status     PaymentStatus
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewPaymentRecord creates a new PaymentRecord in pending status and records
// a ChargeCreated event.
func NewPaymentRecord(invoiceID, provider, externalID string, amount int64, currency string, now time.Time) *PaymentRecord {
	p := &PaymentRecord{
		ID:         uuid.Must(uuid.NewV7()).String(),
		InvoiceID:  invoiceID,
		Provider:   provider,
		ExternalID: externalID,
		Amount:     amount,
		Currency:   currency,
		Status:     PaymentPending,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	p.RecordEvent(domainevent.NewAtWithEntity(EventChargeCreated, ChargeCreatedPayload{
		PaymentID:  p.ID,
		InvoiceID:  p.InvoiceID,
		Provider:   p.Provider,
		ExternalID: p.ExternalID,
		Amount:     p.Amount,
	}, now, p.ID))
	return p
}

// MarkCompleted transitions a pending payment to completed.
func (p *PaymentRecord) MarkCompleted(now time.Time) error {
	if p.Status != PaymentPending {
		return ErrInvalidPaymentState
	}
	p.Status = PaymentCompleted
	p.UpdatedAt = now
	p.RecordEvent(domainevent.NewAtWithEntity(EventChargeCompleted, ChargeCompletedPayload{
		PaymentID: p.ID,
		InvoiceID: p.InvoiceID,
		Provider:  p.Provider,
		Amount:    p.Amount,
	}, now, p.ID))
	return nil
}

// MarkFailed transitions a pending payment to failed.
func (p *PaymentRecord) MarkFailed(reason string, now time.Time) error {
	if p.Status != PaymentPending {
		return ErrInvalidPaymentState
	}
	p.Status = PaymentFailed
	p.UpdatedAt = now
	p.RecordEvent(domainevent.NewAtWithEntity(EventChargeFailed, ChargeFailedPayload{
		PaymentID: p.ID,
		InvoiceID: p.InvoiceID,
		Provider:  p.Provider,
		Reason:    reason,
	}, now, p.ID))
	return nil
}

// MarkRefunded transitions a completed payment to refunded.
func (p *PaymentRecord) MarkRefunded(amount int64, now time.Time) error {
	if p.Status != PaymentCompleted {
		return ErrInvalidPaymentState
	}
	p.Status = PaymentRefunded
	p.UpdatedAt = now
	p.RecordEvent(domainevent.NewAtWithEntity(EventRefundCompleted, RefundCompletedPayload{
		PaymentID: p.ID,
		InvoiceID: p.InvoiceID,
		Provider:  p.Provider,
		Amount:    amount,
	}, now, p.ID))
	return nil
}

// WebhookLog records the receipt and processing of an inbound payment webhook.
type WebhookLog struct {
	ID          string
	Provider    string
	ExternalID  string // idempotency key
	RawBody     []byte
	Status      WebhookStatus
	ProcessedAt *time.Time
	CreatedAt   time.Time
}

// NewWebhookLog creates a new WebhookLog entry in pending status.
func NewWebhookLog(provider, externalID string, rawBody []byte, now time.Time) *WebhookLog {
	return &WebhookLog{
		ID:         uuid.Must(uuid.NewV7()).String(),
		Provider:   provider,
		ExternalID: externalID,
		RawBody:    rawBody,
		Status:     WebhookPending,
		CreatedAt:  now,
	}
}

// MarkProcessed transitions the webhook log to processed status.
func (w *WebhookLog) MarkProcessed(now time.Time) {
	w.Status = WebhookProcessed
	w.ProcessedAt = &now
}

// MarkFailed transitions the webhook log to failed status.
func (w *WebhookLog) MarkFailed(now time.Time) {
	w.Status = WebhookFailed
	w.ProcessedAt = &now
}

// CreateChargeRequest holds the parameters for creating a payment charge.
type CreateChargeRequest struct {
	InvoiceID string
	Amount    int64
	Currency  string
	UserID    string
	UserEmail string
	PlanName  string
	ReturnURL string
	CancelURL string
}

// CreateChargeResult holds the response from the payment plugin.
type CreateChargeResult struct {
	Provider    string `json:"provider"`
	ExternalID  string `json:"external_id"`
	CheckoutURL string `json:"checkout_url"`
	Status      string `json:"status"`
}

// VerifiedStatusSucceeded is the status string returned by payment plugins
// when a webhook confirms a successful payment.
const VerifiedStatusSucceeded = "succeeded"

// VerifiedPayment holds the result of verifying an inbound payment webhook.
type VerifiedPayment struct {
	Provider   string `json:"provider"`
	ExternalID string `json:"external_id"`
	InvoiceID  string `json:"invoice_id"`
	Amount     int64  `json:"amount"`
	Currency   string `json:"currency"`
	Status     string `json:"status"` // succeeded, failed
}
