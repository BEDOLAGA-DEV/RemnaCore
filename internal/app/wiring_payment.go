package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// paymentWiring provides all payment-domain bindings: the payment repository
// implementation and typed hook functions that wrap the hookdispatch.Dispatcher.
var paymentWiring = fx.Options(
	// Payment domain service
	fx.Provide(newPaymentFacade),

	// Payment repos -> interface bindings
	fx.Provide(postgres.NewPaymentRepository),
	fx.Provide(func(repo *postgres.PaymentRepository) payment.PaymentRepository { return repo }),
)

// newPaymentFacade creates a PaymentFacade with typed hook functions that wrap
// the hookdispatch.Dispatcher. Each hook function handles JSON marshaling,
// plugin dispatch, and response deserialization so the payment domain never
// touches encoding/json or hookdispatch directly.
func newPaymentFacade(params paymentFacadeParams) *payment.PaymentFacade {
	d := params.Dispatcher
	logger := params.Logger

	createCharge := newCreateChargeHookFn(d, logger)
	verifyWebhook := newVerifyWebhookHookFn(d, logger)
	refundHook := newRefundHookFn(d, logger)

	return payment.NewPaymentFacade(
		createCharge,
		verifyWebhook,
		refundHook,
		params.Repo,
		params.Publisher,
		params.TxRunner,
		params.Logger,
		params.Clock,
	)
}

// paymentFacadeParams groups the Fx dependencies for newPaymentFacade.
type paymentFacadeParams struct {
	fx.In

	Dispatcher hookdispatch.Dispatcher
	Repo       payment.PaymentRepository
	Publisher  domainevent.Publisher
	TxRunner   txmanager.Runner
	Logger     *slog.Logger
	Clock      clock.Clock
}

// newCreateChargeHookFn creates a typed CreateChargeHookFn that marshals the
// request, dispatches via the plugin dispatcher, and unmarshals the result.
func newCreateChargeHookFn(d hookdispatch.Dispatcher, logger *slog.Logger) payment.CreateChargeHookFn {
	return func(ctx context.Context, req payment.CreateChargeRequest) (*payment.CreateChargeResult, error) {
		data, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("marshal create charge request: %w", err)
		}

		output, err := d.DispatchSync(ctx, payment.HookCreateCharge, data)
		if err != nil {
			return nil, err
		}

		var result payment.CreateChargeResult
		if err := json.Unmarshal(output, &result); err != nil {
			return nil, fmt.Errorf("unmarshal create charge result: %w", err)
		}

		return &result, nil
	}
}

// newVerifyWebhookHookFn creates a typed VerifyWebhookHookFn that marshals the
// request, dispatches via the plugin dispatcher, and unmarshals the result.
func newVerifyWebhookHookFn(d hookdispatch.Dispatcher, logger *slog.Logger) payment.VerifyWebhookHookFn {
	return func(ctx context.Context, req payment.VerifyWebhookHookRequest) (*payment.VerifiedPayment, error) {
		data, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("marshal verify webhook request: %w", err)
		}

		output, err := d.DispatchSync(ctx, payment.HookVerifyWebhook, data)
		if err != nil {
			return nil, err
		}

		var verified payment.VerifiedPayment
		if err := json.Unmarshal(output, &verified); err != nil {
			return nil, fmt.Errorf("unmarshal verified payment: %w", err)
		}

		return &verified, nil
	}
}

// newRefundHookFn creates a typed RefundHookFn that marshals the request and
// dispatches via the plugin dispatcher. The refund hook has no meaningful
// response payload.
func newRefundHookFn(d hookdispatch.Dispatcher, logger *slog.Logger) payment.RefundHookFn {
	return func(ctx context.Context, req payment.RefundHookRequest) error {
		data, err := json.Marshal(req)
		if err != nil {
			return fmt.Errorf("marshal refund request: %w", err)
		}

		_, err = d.DispatchSync(ctx, payment.HookRefund, data)
		if err != nil {
			return err
		}

		return nil
	}
}
