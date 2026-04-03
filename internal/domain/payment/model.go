package payment

import (
	"time"

	"github.com/google/uuid"
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
type PaymentRecord struct {
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

// NewPaymentRecord creates a new PaymentRecord in pending status.
func NewPaymentRecord(invoiceID, provider, externalID string, amount int64, currency string, now time.Time) *PaymentRecord {
	return &PaymentRecord{
		ID:         uuid.New().String(),
		InvoiceID:  invoiceID,
		Provider:   provider,
		ExternalID: externalID,
		Amount:     amount,
		Currency:   currency,
		Status:     PaymentPending,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// MarkCompleted transitions a pending payment to completed.
func (p *PaymentRecord) MarkCompleted(now time.Time) error {
	if p.Status != PaymentPending {
		return ErrInvalidPaymentState
	}
	p.Status = PaymentCompleted
	p.UpdatedAt = now
	return nil
}

// MarkFailed transitions a pending payment to failed.
func (p *PaymentRecord) MarkFailed(now time.Time) error {
	if p.Status != PaymentPending {
		return ErrInvalidPaymentState
	}
	p.Status = PaymentFailed
	p.UpdatedAt = now
	return nil
}

// MarkRefunded transitions a completed payment to refunded.
func (p *PaymentRecord) MarkRefunded(now time.Time) error {
	if p.Status != PaymentCompleted {
		return ErrInvalidPaymentState
	}
	p.Status = PaymentRefunded
	p.UpdatedAt = now
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
		ID:         uuid.New().String(),
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
