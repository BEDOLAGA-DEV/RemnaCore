package app

import (
	"context"
	"encoding/json"
	"fmt"
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

// newBillingService creates a BillingService with hook dispatch functions
// wired from the Fx container. The hookdispatch.Dispatcher is wrapped in
// SyncHookFn / AsyncHookFn closures so the domain service never imports
// hookdispatch or sdk packages.
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
			billingservice.WithSyncHook(newSyncHookFn(dispatcher, logger)),
			billingservice.WithAsyncHook(newAsyncHookFn(dispatcher, logger)),
		)
	}
	return billingservice.NewBillingService(
		plans, subs, invoices, families, publisher, prorate, trial, txRunner, clk, logger,
		opts...,
	)
}

// newCheckoutService creates a CheckoutService with hook dispatch functions
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
			billingservice.WithCheckoutSyncHook(newSyncHookFn(dispatcher, logger)),
			billingservice.WithCheckoutAsyncHook(newAsyncHookFn(dispatcher, logger)),
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

// newSyncHookFn creates a SyncHookFn that wraps a hookdispatch.Dispatcher.
// It marshals the domain payload to JSON, dispatches via DispatchSyncSafe
// (plugin failures never block), and returns the response payload.
func newSyncHookFn(d hookdispatch.Dispatcher, logger *slog.Logger) billingservice.SyncHookFn {
	return func(ctx context.Context, hookName string, payload any) (json.RawMessage, error) {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal hook payload: %w", err)
		}
		result := d.DispatchSyncSafe(ctx, hookName, data)
		if result.Err != nil {
			logger.Warn("hook dispatch failed, proceeding with defaults",
				slog.String("hook", hookName),
				slog.Any("error", result.Err),
			)
			return nil, nil
		}
		return result.Payload, nil
	}
}

// newAsyncHookFn creates an AsyncHookFn that wraps a hookdispatch.Dispatcher.
// It marshals the domain payload to JSON and dispatches asynchronously
// (fire-and-forget). Marshal errors are logged but never propagated.
func newAsyncHookFn(d hookdispatch.Dispatcher, logger *slog.Logger) billingservice.AsyncHookFn {
	return func(ctx context.Context, hookName string, payload any) {
		data, err := json.Marshal(payload)
		if err != nil {
			logger.Warn("failed to marshal async hook payload",
				slog.String("hook", hookName),
				slog.Any("error", err),
			)
			return
		}
		d.DispatchAsync(ctx, hookName, data)
	}
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
