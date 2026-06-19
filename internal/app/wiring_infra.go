package app

import (
	"context"
	"log/slog"

	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/infra"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/infra/health"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/infra/proxy"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/infra/speedtest"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// infraWiring provides infrastructure service lifecycle hooks: health monitor,
// speed test server, subscription proxy, and data cleanup scheduler.
var infraWiring = fx.Options(
	// Infrastructure services module
	infra.Module,

	// Data cleanup scheduler for cross-domain infrastructure data
	fx.Provide(func(idempotency *postgres.IdempotencyRepository, bindings *postgres.BindingRepository, runner txmanager.Runner, logger *slog.Logger) *infra.DataCleanupScheduler {
		return infra.NewDataCleanupScheduler(idempotency, bindings, runner, logger)
	}),

	// Lifecycle hooks
	fx.Invoke(startHealthMonitor),
	fx.Invoke(startSpeedTest),
	fx.Invoke(startSubscriptionProxy),
	fx.Invoke(startDataCleanup),
)

// startHealthMonitor runs the node health monitor as a background goroutine.
func startHealthMonitor(lc fx.Lifecycle, hm *health.HealthMonitor, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			hmCtx, cancel := context.WithCancel(context.Background())
			go func() {
				logger.Info("health monitor started")
				hm.Run(hmCtx)
			}()
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logger.Info("health monitor stopping")
					cancel()
					return nil
				},
			})
			return nil
		},
	})
}

// startSpeedTest runs the speed test server on its dedicated port.
func startSpeedTest(lc fx.Lifecycle, st *speedtest.SpeedTestServer, cfg *config.Config, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			stCtx, cancel := context.WithCancel(context.Background())
			go func() {
				port := cfg.Infra.SpeedTestPort
				if port == 0 {
					port = speedtest.SpeedTestPort
				}
				if err := st.Start(stCtx, port); err != nil {
					logger.Error("speed test server error", slog.Any("error", err))
				}
			}()
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logger.Info("speed test server stopping")
					cancel()
					return nil
				},
			})
			return nil
		},
	})
}

// startSubscriptionProxy runs the subscription proxy on its dedicated port.
func startSubscriptionProxy(lc fx.Lifecycle, sp *proxy.SubscriptionProxy, cfg *config.Config, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			spCtx, cancel := context.WithCancel(context.Background())
			go func() {
				port := cfg.Infra.SubscriptionProxyPort
				if port == 0 {
					port = proxy.SubscriptionProxyPort
				}
				if err := sp.Start(spCtx, port); err != nil {
					logger.Error("subscription proxy error", slog.Any("error", err))
				}
			}()
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logger.Info("subscription proxy stopping")
					cancel()
					return nil
				},
			})
			return nil
		},
	})
}

// startDataCleanup spawns the cross-domain data cleanup scheduler as a
// background goroutine managed by the Fx lifecycle. It periodically removes
// old idempotency keys (>7 days) and binding sync log entries (>90 days).
func startDataCleanup(lc fx.Lifecycle, scheduler *infra.DataCleanupScheduler, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			cleanupCtx, cancel := context.WithCancel(context.Background())
			go func() {
				logger.Info("data cleanup scheduler started")
				scheduler.Run(cleanupCtx)
			}()
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logger.Info("data cleanup scheduler stopping")
					cancel()
					return nil
				},
			})
			return nil
		},
	})
}
