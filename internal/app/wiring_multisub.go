package app

import (
	"context"
	"log/slog"

	"go.uber.org/fx"

	natsadapter "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/nats"
	pluginadapter "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	multisubservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookfn"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// multisubWiring provides all multisub-domain bindings: binding repository,
// saga repository, Remnawave gateway, event handler, lookup adapters, VPN
// provider (optional, plugin-backed), and lifecycle hooks for periodic sync,
// binding reconciler, and saga cleanup.
var multisubWiring = fx.Options(
	// MultiSub domain services
	fx.Provide(multisubservice.NewBindingCalculator),
	fx.Provide(multisubservice.NewProvisioningSaga),
	fx.Provide(multisubservice.NewDeprovisioningSaga),
	fx.Provide(multisubservice.NewSyncSaga),
	fx.Provide(multisubservice.NewSyncService),
	fx.Provide(multisubservice.NewBindingLifecycleService),
	fx.Provide(newMultiSubOrchestrator),
	fx.Provide(multisubservice.NewBindingReconciler),
	fx.Provide(multisubservice.NewSagaCleanupService),

	// VPN provider: plugin-backed adapter when feature flag is enabled, nil
	// otherwise (existing RemnawaveGateway is used directly by the sagas).
	fx.Provide(provideVPNProvider),

	// MultiSub repos -> interface bindings
	fx.Provide(postgres.NewBindingRepository),
	fx.Provide(func(repo *postgres.BindingRepository) multisub.BindingRepository { return repo }),
	fx.Provide(func(repo *postgres.BindingRepository) multisub.BindingReader { return repo }),

	// Saga repository -> interface binding
	fx.Provide(postgres.NewSagaRepository),
	fx.Provide(func(repo *postgres.SagaRepository) multisub.SagaRepository { return repo }),

	// Remnawave gateway -> interface binding
	fx.Provide(func(client *remnawave.ResilientClient, clk clock.Clock, cfg *config.Config) *remnawave.GatewayAdapter {
		return remnawave.NewGatewayAdapter(client, clk, cfg.Remnawave.DefaultInternalSquads)
	}),
	fx.Provide(func(adapter *remnawave.GatewayAdapter) multisub.RemnawaveGateway { return adapter }),

	// MultiSub orchestrator -> billing event handler interface
	fx.Provide(func(o *multisubservice.MultiSubOrchestrator) natsadapter.SubscriptionEventHandler {
		return o
	}),

	// Billing event consumer dependencies — lookup adapter satisfies multisub
	// domain ports (PlanProvider + SubscriptionProvider).
	fx.Provide(natsadapter.NewBillingSubscriptionLookup),
	fx.Provide(func(l *natsadapter.BillingSubscriptionLookup) multisub.PlanProvider { return l }),
	fx.Provide(func(l *natsadapter.BillingSubscriptionLookup) multisub.SubscriptionProvider { return l }),

	// Lifecycle hooks
	fx.Invoke(startSyncService),
	fx.Invoke(startBindingReconciler),
	fx.Invoke(startSagaCleanup),
)

// newMultiSubOrchestrator creates a MultiSubOrchestrator with typed hook
// functions wired from the Fx container via functional options. The
// hookdispatch.Dispatcher is wrapped in closures so the multisub domain never
// imports hookdispatch or encoding/json for hook dispatch.
func newMultiSubOrchestrator(
	provisioning *multisubservice.ProvisioningSaga,
	deprovisioning *multisubservice.DeprovisioningSaga,
	syncService *multisubservice.SyncService,
	lifecycle *multisubservice.BindingLifecycleService,
	bindings multisub.BindingRepository,
	publisher domainevent.Publisher,
	txRunner txmanager.Runner,
	clk clock.Clock,
	logger *slog.Logger,
	dispatcher hookdispatch.Dispatcher,
	cfg *config.Config,
) *multisubservice.MultiSubOrchestrator {
	var opts []multisubservice.OrchestratorOption
	if cfg.FeatureFlags.HooksSubscriptionEnabled && dispatcher != nil {
		opts = append(opts,
			multisubservice.WithLimitingHook(hookfn.NewSyncSafe[multisubservice.SubLimitingPayload, multisubservice.SubLimitingResponse](dispatcher, multisubservice.HookSubLimiting, logger)),
			multisubservice.WithSyncHook(hookfn.NewSyncRaw(dispatcher, logger)),
			multisubservice.WithAsyncHook(hookfn.NewAsync(dispatcher, logger)),
		)
	}
	opts = append(opts, multisubservice.WithHooksEnabled(cfg.FeatureFlags.HooksSubscriptionEnabled))

	return multisubservice.NewMultiSubOrchestrator(
		provisioning, deprovisioning, syncService, lifecycle, bindings, publisher, txRunner, clk, logger,
		opts...,
	)
}

// provideVPNProvider creates a plugin-backed VPN provider when the shared VPN
// executor is available (non-nil). When nil (HooksVPNProviderEnabled is false),
// nil is returned — existing code paths using RemnawaveGateway directly remain
// unchanged. The executor is created once in provideVPNExecutor (wiring_plugin.go)
// and shared with the plugin HostFunctions so a single circuit breaker protects
// the VPN backend.
func provideVPNProvider(
	executor sdk.VPNHTTPExecutor,
	dispatcher hookdispatch.Dispatcher,
	logger *slog.Logger,
) multisub.VPNProvider {
	if executor == nil {
		return nil
	}
	return pluginadapter.NewPluginVPNProvider(dispatcher, executor, logger)
}

// startSyncService spawns the periodic Remnawave binding sync as a background
// goroutine managed by the Fx lifecycle.
func startSyncService(lc fx.Lifecycle, syncService *multisubservice.SyncService, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Create a long-lived context that is cancelled on shutdown.
			syncCtx, cancel := context.WithCancel(context.Background())
			go func() {
				logger.Info("periodic sync service started")
				syncService.RunPeriodicSync(syncCtx)
			}()
			// Store cancel for OnStop via closure.
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logger.Info("periodic sync service stopping")
					cancel()
					return nil
				},
			})
			return nil
		},
	})
}

// startBindingReconciler spawns the orphaned Remnawave user reconciler as a
// background goroutine managed by the Fx lifecycle. It periodically cleans up
// ghost Remnawave users left behind by failed saga compensations.
func startBindingReconciler(lc fx.Lifecycle, reconciler *multisubservice.BindingReconciler, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			recCtx, cancel := context.WithCancel(context.Background())
			go func() {
				logger.Info("binding reconciler started")
				reconciler.Run(recCtx)
			}()
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logger.Info("binding reconciler stopping")
					cancel()
					return nil
				},
			})
			return nil
		},
	})
}

// startSagaCleanup reports stale running sagas on startup and spawns the
// periodic saga cleanup service as a background goroutine.
func startSagaCleanup(lc fx.Lifecycle, cleanup *multisubservice.SagaCleanupService, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Report stale sagas synchronously on startup before accepting traffic.
			cleanup.ReportStaleOnStartup(ctx)

			cleanupCtx, cancel := context.WithCancel(context.Background())
			go func() {
				logger.Info("saga cleanup service started")
				cleanup.Run(cleanupCtx)
			}()
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logger.Info("saga cleanup service stopping")
					cancel()
					return nil
				},
			})
			return nil
		},
	})
}
