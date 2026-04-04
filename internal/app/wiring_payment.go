package app

import (
	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
)

// paymentWiring provides all payment-domain bindings: the payment repository
// implementation.
var paymentWiring = fx.Options(
	// Payment domain service
	fx.Provide(payment.NewPaymentFacade),

	// Payment repos -> interface bindings
	fx.Provide(postgres.NewPaymentRepository),
	fx.Provide(func(repo *postgres.PaymentRepository) payment.PaymentRepository { return repo }),
)
