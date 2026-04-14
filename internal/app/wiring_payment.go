package app

import (
	"log/slog"

	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookfn"
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

	return payment.NewPaymentFacade(
		hookfn.NewSync[payment.CreateChargeRequest, payment.CreateChargeResult](d, payment.HookCreateCharge),
		hookfn.NewSync[payment.VerifyWebhookHookRequest, payment.VerifiedPayment](d, payment.HookVerifyWebhook),
		hookfn.NewSyncFireAndForget[payment.RefundHookRequest](d, payment.HookRefund),
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

