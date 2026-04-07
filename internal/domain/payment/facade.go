package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tracing"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
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
//
// # Architectural rationale
//
// PaymentFacade intentionally combines domain port, application orchestration,
// and plugin protocol handling in a single type within the domain layer. This is
// a pragmatic trade-off for RemnaCore's plugin-first architecture:
//
//   - hookdispatch.Dispatcher is a shared kernel package (pkg/hookdispatch), not
//     an infrastructure adapter. It defines a pure Go interface with no external
//     dependencies. Domain packages importing it does not create a coupling to
//     infrastructure -- the dependency arrow points from domain to pkg, which is
//     the same direction as stdlib imports.
//
//   - Payment is 100% plugin-driven. There is no built-in provider logic to
//     separate from the dispatch mechanism. Extracting a port interface and
//     adapter would produce a pure-delegation layer (port calls adapter, adapter
//     calls dispatcher) with no additional decoupling benefit since this is a
//     modular monolith compiled as a single binary.
//
//   - The facade validates inputs, dispatches to plugins, interprets results,
//     persists records, and publishes domain events. This is deliberate: all
//     payment invariants are enforced in one place rather than scattered across
//     a thin domain model and a separate application service.
//
// Rationale: in a modular monolith with a single transport layer (chi HTTP),
// the cost of 4-6 additional files per context (port interface, adapter struct,
// adapter constructor, Fx wiring, tests) outweighs the benefit when the adapter
// would be a one-line delegation to the same dispatcher interface.
//
// When to split: if RemnaCore gains a second transport layer (gRPC/ConnectRPC
// gateway), is extracted into separate microservices, or if payment orchestration
// logic diverges from plugin dispatch (e.g., built-in provider fallbacks), then
// extract a PaymentDispatcher port in the domain and move plugin protocol
// handling to an adapter in internal/adapter/plugin/.
type PaymentFacade struct {
	dispatcher hookdispatch.Dispatcher
	repo       PaymentRepository
	publisher  domainevent.Publisher
	txRunner   txmanager.Runner
	logger     *slog.Logger
	clock      clock.Clock
}

// NewPaymentFacade creates a PaymentFacade with the given dependencies.
func NewPaymentFacade(
	dispatcher hookdispatch.Dispatcher,
	repo PaymentRepository,
	publisher domainevent.Publisher,
	txRunner txmanager.Runner,
	logger *slog.Logger,
	clk clock.Clock,
) *PaymentFacade {
	return &PaymentFacade{
		dispatcher: dispatcher,
		repo:       repo,
		publisher:  publisher,
		txRunner:   txRunner,
		logger:     logger,
		clock:      clk,
	}
}

// CreateCharge dispatches a payment creation request to the registered payment
// plugin and persists the resulting payment record.
func (f *PaymentFacade) CreateCharge(ctx context.Context, req CreateChargeRequest) (*CreateChargeResult, error) {
	ctx, span := tracing.StartSpan(ctx, "payment.create_charge")
	defer span.End()

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

	record := NewPaymentRecord(req.InvoiceID, result.Provider, result.ExternalID, req.Amount, req.Currency, f.clock.Now())
	if err := f.repo.CreatePayment(ctx, record); err != nil {
		return nil, fmt.Errorf("persist payment record: %w", err)
	}

	// Aggregate already recorded its own creation event in NewPaymentRecord.
	if err := f.publishAggregateEvents(ctx, record); err != nil {
		f.logger.Warn("failed to publish aggregate events",
			slog.String("payment_id", record.ID),
			slog.Any("error", err),
		)
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

	payload, err := json.Marshal(VerifyWebhookHookRequest{
		Provider: provider,
		Headers:  headers,
		Body:     body,
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
		webhookEvent := NewWebhookReceivedEvent(provider, verified.ExternalID, verified.Status, f.clock.Now())
		if err := f.publisher.Publish(ctx, webhookEvent); err != nil {
			f.logger.Warn("failed to publish event",
				slog.String("event_type", string(webhookEvent.Type)),
				slog.Any("error", err),
			)
		}
	}

	f.logger.Info("webhook verified",
		slog.String("provider", provider),
		slog.String("external_id", verified.ExternalID),
		slog.String("status", verified.Status),
	)

	return &verified, nil
}

// Refund dispatches a refund request to the registered payment plugin. The
// external plugin call happens outside the transaction (to avoid holding row
// locks during I/O), then the record is re-read with FOR UPDATE inside a
// transaction before mutation and persistence.
func (f *PaymentFacade) Refund(ctx context.Context, paymentID string, amount int64, reason string) error {
	// Read without lock to get provider/externalID for the external call.
	record, err := f.repo.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("get payment for refund: %w", err)
	}

	payload, err := json.Marshal(RefundHookRequest{
		Provider:   record.Provider,
		ExternalID: record.ExternalID,
		Amount:     amount,
		Reason:     reason,
	})
	if err != nil {
		return fmt.Errorf("marshal refund request: %w", err)
	}

	_, err = f.dispatcher.DispatchSync(ctx, HookRefund, payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRefundFailed, err)
	}

	// Re-read with FOR UPDATE inside tx to prevent TOCTOU races.
	err = f.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		locked, err := f.repo.GetPaymentByIDForUpdate(txCtx, paymentID)
		if err != nil {
			return fmt.Errorf("get payment for refund (locked): %w", err)
		}

		if err := locked.MarkRefunded(amount, f.clock.Now()); err != nil {
			return fmt.Errorf("mark payment refunded: %w", err)
		}

		if err := f.repo.UpdatePayment(txCtx, locked); err != nil {
			return fmt.Errorf("persist refunded payment: %w", err)
		}

		return f.publishAggregateEvents(txCtx, locked)
	})
	if err != nil {
		return err
	}

	f.logger.Info("payment refunded",
		slog.String("payment_id", paymentID),
		slog.String("provider", record.Provider),
		slog.Int64("amount", amount),
	)

	return nil
}

// CompletePayment marks a payment record as completed after webhook
// confirmation. The read (with FOR UPDATE lock), mutation, update, and event
// publish are all inside a single transaction to prevent TOCTOU races.
func (f *PaymentFacade) CompletePayment(ctx context.Context, provider, externalID string) (*PaymentRecord, error) {
	var record *PaymentRecord
	err := f.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		record, err = f.repo.GetPaymentByExternalIDForUpdate(txCtx, provider, externalID)
		if err != nil {
			return fmt.Errorf("get payment by external id: %w", err)
		}

		if err := record.MarkCompleted(f.clock.Now()); err != nil {
			return fmt.Errorf("mark payment completed: %w", err)
		}

		if err := f.repo.UpdatePayment(txCtx, record); err != nil {
			return fmt.Errorf("persist completed payment: %w", err)
		}

		return f.publishAggregateEvents(txCtx, record)
	})
	if err != nil {
		return nil, err
	}

	return record, nil
}

// CheckIdempotency checks if a webhook has already been processed. Returns
// true if the webhook is a duplicate, false otherwise.
func (f *PaymentFacade) CheckIdempotency(ctx context.Context, provider, externalID string, rawBody []byte) (bool, error) {
	webhookLog := NewWebhookLog(provider, externalID, rawBody, f.clock.Now())
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
	wh.MarkProcessed(f.clock.Now())
	return f.repo.UpdateWebhookLog(ctx, wh)
}

// MarkWebhookFailed updates a webhook log entry as failed.
func (f *PaymentFacade) MarkWebhookFailed(ctx context.Context, provider, externalID string) error {
	wh, err := f.repo.GetWebhookLog(ctx, provider, externalID)
	if err != nil {
		return fmt.Errorf("get webhook log: %w", err)
	}
	wh.MarkFailed(f.clock.Now())
	return f.repo.UpdateWebhookLog(ctx, wh)
}

// isWebhookDuplicate checks if the error indicates a duplicate webhook.
func isWebhookDuplicate(err error) bool {
	return errors.Is(err, ErrWebhookDuplicate)
}

// publishAggregateEvents flushes all pending events from the aggregate and
// publishes them through the publisher. This nil-safe variant is specific to
// PaymentFacade because its publisher may be nil in tests or minimal
// deployments.
func (f *PaymentFacade) publishAggregateEvents(ctx context.Context, src domainevent.EventSource) error {
	if f.publisher == nil {
		return nil
	}
	return domainevent.PublishAll(ctx, f.publisher, src)
}
