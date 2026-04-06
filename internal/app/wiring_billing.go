package app

import (
	"log/slog"

	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/valkey"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	billingservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// billingWiring provides all billing-domain bindings: repository implementations,
// the domain rate limiter, and the payment gateway ACL adapter.
var billingWiring = fx.Options(
	// Billing domain services
	fx.Provide(billingservice.NewProrateCalculator),
	fx.Provide(func(cfg *config.Config, clk clock.Clock) *billingservice.TrialManager {
		return billingservice.NewTrialManagerWithClock(cfg.Billing.TrialDays, clk)
	}),
	fx.Provide(newBillingService),
	fx.Provide(newCheckoutService),

	// PricingModifier: billing.PricingModifier wraps hookdispatch.Dispatcher
	// to handle WASM plugin wire protocol for pricing calculations.
	fx.Provide(func(d hookdispatch.Dispatcher) billing.PricingModifier {
		return newPluginPricingModifier(d)
	}),

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

// newBillingService creates a BillingService with the hook dispatcher and
// feature flag wired from the Fx container via functional options.
func newBillingService(
	plans billing.PlanRepository,
	subs billing.SubscriptionRepository,
	invoices billing.InvoiceRepository,
	families billing.FamilyRepository,
	publisher domainevent.Publisher,
	prorate *billingservice.ProrateCalculator,
	trial *billingservice.TrialManager,
	txRunner txmanager.Runner,
	clk clock.Clock,
	logger *slog.Logger,
	dispatcher hookdispatch.Dispatcher,
	cfg *config.Config,
) *billingservice.BillingService {
	return billingservice.NewBillingService(
		plans, subs, invoices, families, publisher, prorate, trial, txRunner, clk, logger,
		billingservice.WithDispatcher(dispatcher),
		billingservice.WithHooksEnabled(cfg.FeatureFlags.HooksSubscriptionEnabled),
	)
}

// newCheckoutService creates a CheckoutService with the hook dispatcher and
// feature flag wired from the Fx container.
func newCheckoutService(
	billingSvc *billingservice.BillingService,
	paymentGateway billing.PaymentGateway,
	pricing billing.PricingModifier,
	publisher domainevent.Publisher,
	logger *slog.Logger,
	rateLimiter billing.DomainRateLimiter,
	clk clock.Clock,
	dispatcher hookdispatch.Dispatcher,
	cfg *config.Config,
) *billingservice.CheckoutService {
	return billingservice.NewCheckoutService(
		billingSvc,
		paymentGateway,
		pricing,
		publisher,
		logger,
		rateLimiter,
		clk,
		billingservice.WithCheckoutDispatcher(dispatcher),
		billingservice.WithCheckoutHooksEnabled(cfg.FeatureFlags.HooksSubscriptionEnabled),
	)
}
