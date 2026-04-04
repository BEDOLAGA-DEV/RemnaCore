package app

import (
	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/valkey"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	billingservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/service"
)

// billingWiring provides all billing-domain bindings: repository implementations,
// the domain rate limiter, and the payment gateway ACL adapter.
var billingWiring = fx.Options(
	// Billing domain module
	billingservice.Module,

	// Billing -> Payment ACL: billing.PaymentGateway wraps *payment.PaymentFacade
	// so that the billing domain never imports the payment domain directly.
	fx.Provide(newPaymentGatewayAdapter),

	// Billing repos -> interface bindings
	fx.Provide(postgres.NewPlanRepository),
	fx.Provide(func(repo *postgres.PlanRepository) billing.PlanRepository { return repo }),
	fx.Provide(postgres.NewSubscriptionRepository),
	fx.Provide(func(repo *postgres.SubscriptionRepository) billing.SubscriptionRepository { return repo }),
	fx.Provide(postgres.NewInvoiceRepository),
	fx.Provide(func(repo *postgres.InvoiceRepository) billing.InvoiceRepository { return repo }),
	fx.Provide(postgres.NewFamilyRepository),
	fx.Provide(func(repo *postgres.FamilyRepository) billing.FamilyRepository { return repo }),

	// Domain rate limiter: billing.DomainRateLimiter wraps *valkey.DomainRateLimiter
	fx.Provide(func(r *valkey.DomainRateLimiter) billing.DomainRateLimiter { return r }),
)
