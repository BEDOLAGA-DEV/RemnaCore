package app

import (
	"context"
	"log/slog"

	"go.uber.org/fx"

	billingadapter "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/valkey"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	billingservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookfn"
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
	fx.Provide(newFamilyService),
	fx.Provide(newAddonService),
	fx.Provide(billingservice.NewSubscriptionScheduler),

	// Lifecycle hooks
	fx.Invoke(startSubscriptionScheduler),

	// PricingModifier: billing.PricingModifier wraps hookdispatch.Dispatcher
	// to handle WASM plugin wire protocol for pricing calculations.
	fx.Provide(func(d hookdispatch.Dispatcher) billing.PricingModifier {
		return billingadapter.NewPluginPricingModifier(d)
	}),

	// Billing -> Payment ACL: billing.PaymentGateway wraps *payment.PaymentFacade
	// so that the billing domain never imports the payment domain directly.
	fx.Provide(billingadapter.NewPaymentGatewayAdapter),

	// Billing repos -> interface bindings (full Repository for services,
	// Reader for handlers/adapters that only need read access).
	fx.Provide(postgres.NewPlanRepository),
	fx.Provide(func(repo *postgres.PlanRepository) billing.PlanRepository { return repo }),
	fx.Provide(func(repo *postgres.PlanRepository) billing.PlanReader { return repo }),
	fx.Provide(postgres.NewSubscriptionRepository),
	fx.Provide(func(repo *postgres.SubscriptionRepository) billing.SubscriptionRepository { return repo }),
	fx.Provide(func(repo *postgres.SubscriptionRepository) billing.SubscriptionReader { return repo }),
	fx.Provide(postgres.NewInvoiceRepository),
	fx.Provide(func(repo *postgres.InvoiceRepository) billing.InvoiceRepository { return repo }),
	fx.Provide(func(repo *postgres.InvoiceRepository) billing.InvoiceReader { return repo }),
	fx.Provide(postgres.NewFamilyRepository),
	fx.Provide(func(repo *postgres.FamilyRepository) billing.FamilyRepository { return repo }),
	fx.Provide(func(repo *postgres.FamilyRepository) billing.FamilyReader { return repo }),

	// Domain rate limiter: billing.DomainRateLimiter wraps *valkey.DomainRateLimiter
	fx.Provide(func(r *valkey.DomainRateLimiter) billing.DomainRateLimiter { return r }),
)

// newBillingService creates a BillingService with typed hook functions wired
// from the Fx container. Each typed hook function wraps the
// hookdispatch.Dispatcher, handling JSON marshal/unmarshal so the domain
// service never imports encoding/json or hookdispatch.
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
	var opts []billingservice.BillingServiceOption
	if cfg.FeatureFlags.HooksSubscriptionEnabled && dispatcher != nil {
		opts = append(opts,
			billingservice.WithCancelHook(hookfn.NewSyncSafe[billingservice.CancellingPayload, billingservice.CancellingResponse](dispatcher, billingservice.HookSubCancelling, logger)),
			billingservice.WithRenewHook(hookfn.NewSyncSafe[billingservice.RenewingPayload, billingservice.RenewingResponse](dispatcher, billingservice.HookSubRenewing, logger)),
			billingservice.WithUpgradeHook(hookfn.NewSyncSafe[billingservice.UpgradingPayload, billingservice.UpgradingResponse](dispatcher, billingservice.HookSubUpgrading, logger)),
			billingservice.WithAsyncHook(hookfn.NewAsync(dispatcher, logger)),
		)
	}
	return billingservice.NewBillingService(
		billingservice.BillingDeps{
			Plans:     plans,
			Subs:      subs,
			Invoices:  invoices,
			Families:  families,
			Publisher: publisher,
			Prorate:   prorate,
			Trial:     trial,
			TxRunner:  txRunner,
			Clock:     clk,
			Logger:    logger,
		},
		opts...,
	)
}

// newCheckoutService creates a CheckoutService with typed hook functions
// wired from the Fx container.
func newCheckoutService(
	billingSvc *billingservice.BillingService,
	invoiceReader billing.InvoiceReader,
	subReader billing.SubscriptionReader,
	paymentGateway billing.PaymentGateway,
	pricing billing.PricingModifier,
	publisher domainevent.Publisher,
	logger *slog.Logger,
	rateLimiter billing.DomainRateLimiter,
	clk clock.Clock,
	dispatcher hookdispatch.Dispatcher,
	cfg *config.Config,
) *billingservice.CheckoutService {
	var opts []billingservice.CheckoutServiceOption
	if cfg.FeatureFlags.HooksSubscriptionEnabled && dispatcher != nil {
		opts = append(opts,
			billingservice.WithCheckoutValidatingHook(hookfn.NewSyncSafe[billingservice.CheckoutValidatingPayload, billingservice.CheckoutValidatingResponse](dispatcher, billingservice.HookCheckoutValidating, logger)),
			billingservice.WithSubCreatingHook(hookfn.NewSyncSafe[billingservice.SubCreatingPayload, billingservice.SubCreatingResponse](dispatcher, billingservice.HookSubscriptionCreating, logger)),
			billingservice.WithCheckoutAsyncHook(hookfn.NewAsync(dispatcher, logger)),
		)
	}
	return billingservice.NewCheckoutService(
		billingSvc,
		invoiceReader,
		subReader,
		paymentGateway,
		pricing,
		publisher,
		logger,
		rateLimiter,
		clk,
		opts...,
	)
}

// newFamilyService creates a FamilyService wired from the Fx container.
func newFamilyService(
	familyRepo billing.FamilyRepository,
	subReader billing.SubscriptionReader,
	planReader billing.PlanReader,
	publisher domainevent.Publisher,
	txRunner txmanager.Runner,
	clk clock.Clock,
	logger *slog.Logger,
) *billingservice.FamilyService {
	return billingservice.NewFamilyService(familyRepo, subReader, planReader, publisher, txRunner, clk, logger)
}

// newAddonService creates an AddonService wired from the Fx container.
func newAddonService(
	subRepo billing.SubscriptionRepository,
	planReader billing.PlanReader,
	publisher domainevent.Publisher,
	txRunner txmanager.Runner,
	clk clock.Clock,
	logger *slog.Logger,
) *billingservice.AddonService {
	return billingservice.NewAddonService(subRepo, planReader, publisher, txRunner, clk, logger)
}

// startSubscriptionScheduler spawns the subscription scheduler as a background
// goroutine managed by the Fx lifecycle. It periodically checks for active
// subscriptions whose billing period has elapsed and triggers renewal or
// expiration.
func startSubscriptionScheduler(lc fx.Lifecycle, scheduler *billingservice.SubscriptionScheduler, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			schedCtx, cancel := context.WithCancel(context.Background())
			go func() {
				logger.Info("subscription scheduler started")
				scheduler.Run(schedCtx)
			}()
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logger.Info("subscription scheduler stopping")
					cancel()
					return nil
				},
			})
			return nil
		},
	})
}
