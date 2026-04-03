package payment

import "context"

// PaymentRepository defines persistence operations for payment records and
// webhook logs.
type PaymentRepository interface {
	// CreatePayment persists a new payment record.
	CreatePayment(ctx context.Context, record *PaymentRecord) error

	// GetPaymentByID retrieves a payment record by its domain ID.
	GetPaymentByID(ctx context.Context, id string) (*PaymentRecord, error)

	// GetPaymentByExternalID retrieves a payment record by provider + external ID.
	GetPaymentByExternalID(ctx context.Context, provider, externalID string) (*PaymentRecord, error)

	// UpdatePayment persists status changes on an existing payment record.
	UpdatePayment(ctx context.Context, record *PaymentRecord) error

	// CreateWebhookLog inserts a webhook log entry. Returns ErrWebhookDuplicate
	// if the (provider, external_id) pair already exists.
	CreateWebhookLog(ctx context.Context, log *WebhookLog) error

	// GetWebhookLog retrieves a webhook log by provider + external ID.
	GetWebhookLog(ctx context.Context, provider, externalID string) (*WebhookLog, error)

	// UpdateWebhookLog persists status changes on an existing webhook log.
	UpdateWebhookLog(ctx context.Context, log *WebhookLog) error
}
