package app

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
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
	fx.Provide(remnawave.NewGatewayAdapter),
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
	clk clock.Clock,
	logger *slog.Logger,
	dispatcher hookdispatch.Dispatcher,
	cfg *config.Config,
) *multisubservice.MultiSubOrchestrator {
	var opts []multisubservice.OrchestratorOption
	if cfg.FeatureFlags.HooksSubscriptionEnabled && dispatcher != nil {
		opts = append(opts,
			multisubservice.WithLimitingHook(newLimitingHookFn(dispatcher, logger)),
			multisubservice.WithSyncHook(newMultiSubSyncHookFn(dispatcher, logger)),
			multisubservice.WithAsyncHook(newMultiSubAsyncHookFn(dispatcher, logger)),
		)
	}
	opts = append(opts, multisubservice.WithHooksEnabled(cfg.FeatureFlags.HooksSubscriptionEnabled))

	return multisubservice.NewMultiSubOrchestrator(
		provisioning, deprovisioning, syncService, lifecycle, bindings, publisher, clk, logger,
		opts...,
	)
}

// newLimitingHookFn creates a typed LimitingHookFn that marshals the payload,
// dispatches via DispatchSyncSafe, and unmarshals the response.
func newLimitingHookFn(d hookdispatch.Dispatcher, logger *slog.Logger) multisubservice.LimitingHookFn {
	return func(ctx context.Context, payload multisubservice.SubLimitingPayload) (*multisubservice.SubLimitingResponse, error) {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal limiting hook payload: %w", err)
		}

		result := d.DispatchSyncSafe(ctx, multisubservice.HookSubLimiting, data)
		if result.Err != nil {
			logger.Warn("limiting hook dispatch failed",
				slog.String("hook", multisubservice.HookSubLimiting),
				slog.Any("error", result.Err),
			)
			return nil, nil
		}
		if result.Payload == nil {
			return nil, nil
		}

		var resp multisubservice.SubLimitingResponse
		if err := json.Unmarshal(result.Payload, &resp); err != nil {
			logger.Warn("failed to unmarshal limiting hook response",
				slog.Any("error", err),
			)
			return nil, nil
		}

		return &resp, nil
	}
}

// newMultiSubSyncHookFn creates a SyncHookFn that wraps a hookdispatch.Dispatcher.
// It marshals the domain payload to JSON, dispatches via DispatchSyncSafe, and
// returns the raw response bytes.
func newMultiSubSyncHookFn(d hookdispatch.Dispatcher, logger *slog.Logger) multisubservice.SyncHookFn {
	return func(ctx context.Context, hookName string, payload any) ([]byte, error) {
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

// newMultiSubAsyncHookFn creates an AsyncHookFn that wraps a hookdispatch.Dispatcher.
// It marshals the domain payload to JSON and dispatches asynchronously
// (fire-and-forget). Marshal errors are logged but never propagated.
func newMultiSubAsyncHookFn(d hookdispatch.Dispatcher, logger *slog.Logger) multisubservice.AsyncHookFn {
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
