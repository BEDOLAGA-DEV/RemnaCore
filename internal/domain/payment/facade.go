package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
)

// Hook names dispatched by the payment facade. All payment logic is delegated
// to plugins registered for these hooks.
const (
	HookCreateCharge  = "payment.create_charge"
	HookVerifyWebhook = "payment.verify_webhook"
	HookRefund        = "payment.refund"
)

// PaymentFacade dispatches payment operations to plugins via the hook
// dispatcher. It contains NO built-in Stripe/BTCPay logic.
type PaymentFacade struct {
	dispatcher hookdispatch.Dispatcher
	repo       PaymentRepository
	publisher  domainevent.Publisher
	logger     *slog.Logger
}

// NewPaymentFacade creates a PaymentFacade with the given dependencies.
func NewPaymentFacade(
	dispatcher hookdispatch.Dispatcher,
	repo PaymentRepository,
	publisher domainevent.Publisher,
	logger *slog.Logger,
) *PaymentFacade {
	return &PaymentFacade{
		dispatcher: dispatcher,
		repo:       repo,
		publisher:  publisher,
		logger:     logger,
	}
}

// CreateCharge dispatches a payment creation request to the registered payment
// plugin and persists the resulting payment record.
func (f *PaymentFacade) CreateCharge(ctx context.Context, req CreateChargeRequest) (*CreateChargeResult, error) {
	if req.InvoiceID == "" {
		return nil, ErrMissingInvoiceID
	}
	if req.Amount <= 0 {
		return nil, ErrMissingAmount
	}
	if req.Currency == "" {
		return nil, ErrMissingCurrency
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal create charge request: %w", err)
	}

	output, err := f.dispatcher.DispatchSync(ctx, HookCreateCharge, payload)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPaymentFailed, err)
	}

	var result CreateChargeResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("unmarshal create charge result: %w", err)
	}

	if result.Provider == "" || result.ExternalID == "" {
		return nil, fmt.Errorf("%w: plugin returned incomplete result", ErrPaymentFailed)
	}

	record := NewPaymentRecord(req.InvoiceID, result.Provider, result.ExternalID, req.Amount, req.Currency)
	if err := f.repo.CreatePayment(ctx, record); err != nil {
		return nil, fmt.Errorf("persist payment record: %w", err)
	}

	if f.publisher != nil {
		_ = f.publisher.Publish(ctx, NewChargeCreatedEvent(
			record.ID, record.InvoiceID, record.Provider, record.ExternalID, record.Amount,
		))
	}

	f.logger.Info("payment charge created",
		slog.String("payment_id", record.ID),
		slog.String("invoice_id", record.InvoiceID),
		slog.String("provider", result.Provider),
		slog.String("external_id", result.ExternalID),
	)

	return &result, nil
}

// VerifyWebhook dispatches a webhook verification request to the registered
// payment plugin and returns the verified payment details.
func (f *PaymentFacade) VerifyWebhook(ctx context.Context, provider string, headers map[string]string, body []byte) (*VerifiedPayment, error) {
	if provider == "" {
		return nil, ErrInvalidProvider
	}

	payload, err := json.Marshal(map[string]any{
		"provider": provider,
		"headers":  headers,
		"body":     body,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal verify webhook request: %w", err)
	}

	output, err := f.dispatcher.DispatchSync(ctx, HookVerifyWebhook, payload)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVerificationFailed, err)
	}

	var verified VerifiedPayment
	if err := json.Unmarshal(output, &verified); err != nil {
		return nil, fmt.Errorf("unmarshal verified payment: %w", err)
	}

	if f.publisher != nil {
		_ = f.publisher.Publish(ctx, NewWebhookReceivedEvent(provider, verified.ExternalID, verified.Status))
	}

	f.logger.Info("webhook verified",
		slog.String("provider", provider),
		slog.String("external_id", verified.ExternalID),
		slog.String("status", verified.Status),
	)

	return &verified, nil
}

// Refund dispatches a refund request to the registered payment plugin.
func (f *PaymentFacade) Refund(ctx context.Context, paymentID string, amount int64, reason string) error {
	record, err := f.repo.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("get payment for refund: %w", err)
	}

	payload, err := json.Marshal(map[string]any{
		"provider":    record.Provider,
		"external_id": record.ExternalID,
		"amount":      amount,
		"reason":      reason,
	})
	if err != nil {
		return fmt.Errorf("marshal refund request: %w", err)
	}

	_, err = f.dispatcher.DispatchSync(ctx, HookRefund, payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRefundFailed, err)
	}

	if err := record.MarkRefunded(); err != nil {
		return fmt.Errorf("mark payment refunded: %w", err)
	}

	if err := f.repo.UpdatePayment(ctx, record); err != nil {
		return fmt.Errorf("persist refunded payment: %w", err)
	}

	if f.publisher != nil {
		_ = f.publisher.Publish(ctx, NewRefundCompletedEvent(
			record.ID, record.InvoiceID, record.Provider, amount,
		))
	}

	f.logger.Info("payment refunded",
		slog.String("payment_id", paymentID),
		slog.String("provider", record.Provider),
		slog.Int64("amount", amount),
	)

	return nil
}

// CompletePayment marks a payment record as completed after webhook confirmation.
func (f *PaymentFacade) CompletePayment(ctx context.Context, provider, externalID string) (*PaymentRecord, error) {
	record, err := f.repo.GetPaymentByExternalID(ctx, provider, externalID)
	if err != nil {
		return nil, fmt.Errorf("get payment by external id: %w", err)
	}

	if err := record.MarkCompleted(); err != nil {
		return nil, fmt.Errorf("mark payment completed: %w", err)
	}

	if err := f.repo.UpdatePayment(ctx, record); err != nil {
		return nil, fmt.Errorf("persist completed payment: %w", err)
	}

	if f.publisher != nil {
		_ = f.publisher.Publish(ctx, NewChargeCompletedEvent(
			record.ID, record.InvoiceID, record.Provider, record.Amount,
		))
	}

	return record, nil
}

// CheckIdempotency checks if a webhook has already been processed. Returns
// true if the webhook is a duplicate, false otherwise.
func (f *PaymentFacade) CheckIdempotency(ctx context.Context, provider, externalID string, rawBody []byte) (bool, error) {
	webhookLog := NewWebhookLog(provider, externalID, rawBody)
	err := f.repo.CreateWebhookLog(ctx, webhookLog)
	if err != nil {
		if isWebhookDuplicate(err) {
			return true, nil
		}
		return false, fmt.Errorf("create webhook log: %w", err)
	}
	return false, nil
}

// MarkWebhookProcessed updates a webhook log entry as successfully processed.
func (f *PaymentFacade) MarkWebhookProcessed(ctx context.Context, provider, externalID string) error {
	wh, err := f.repo.GetWebhookLog(ctx, provider, externalID)
	if err != nil {
		return fmt.Errorf("get webhook log: %w", err)
	}
	wh.MarkProcessed()
	return f.repo.UpdateWebhookLog(ctx, wh)
}

// MarkWebhookFailed updates a webhook log entry as failed.
func (f *PaymentFacade) MarkWebhookFailed(ctx context.Context, provider, externalID string) error {
	wh, err := f.repo.GetWebhookLog(ctx, provider, externalID)
	if err != nil {
		return fmt.Errorf("get webhook log: %w", err)
	}
	wh.MarkFailed()
	return f.repo.UpdateWebhookLog(ctx, wh)
}

// isWebhookDuplicate checks if the error indicates a duplicate webhook.
func isWebhookDuplicate(err error) bool {
	return errors.Is(err, ErrWebhookDuplicate)
}
