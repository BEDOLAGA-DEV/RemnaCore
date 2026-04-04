package app

import (
	"context"
	"log/slog"

	"go.uber.org/fx"

	natsadapter "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/nats"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	multisubservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/service"
)

// multisubWiring provides all multisub-domain bindings: binding repository,
// Remnawave gateway, event handler, lookup adapters, and lifecycle hooks for
// the periodic sync and binding reconciler.
var multisubWiring = fx.Options(
	// MultiSub domain module
	multisubservice.Module,

	// MultiSub repos -> interface bindings
	fx.Provide(postgres.NewBindingRepository),
	fx.Provide(func(repo *postgres.BindingRepository) multisub.BindingRepository { return repo }),

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
)

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
