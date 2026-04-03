package payment

import "errors"

var (
	ErrPaymentNotFound     = errors.New("payment record not found")
	ErrWebhookNotFound     = errors.New("webhook log not found")
	ErrWebhookDuplicate    = errors.New("duplicate webhook already processed")
	ErrNoPaymentPlugin     = errors.New("no payment plugin registered")
	ErrPaymentFailed       = errors.New("payment creation failed")
	ErrVerificationFailed  = errors.New("webhook verification failed")
	ErrRefundFailed        = errors.New("refund failed")
	ErrInvalidProvider     = errors.New("invalid payment provider")
	ErrMissingInvoiceID    = errors.New("invoice ID is required")
	ErrMissingAmount       = errors.New("payment amount must be positive")
	ErrMissingCurrency     = errors.New("currency is required")
	ErrMissingExternalID   = errors.New("external ID is required")
	ErrInvalidPaymentState = errors.New("invalid payment state transition")
)
