package app

import (
	"context"
	"log/slog"

	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/infra"
)

// infraWiring provides infrastructure service lifecycle hooks: health monitor,
// speed test server, and subscription proxy.
var infraWiring = fx.Options(
	// Infrastructure services module
	infra.Module,

	// Lifecycle hooks
	fx.Invoke(startHealthMonitor),
	fx.Invoke(startSpeedTest),
	fx.Invoke(startSubscriptionProxy),
)

// startHealthMonitor runs the node health monitor as a background goroutine.
func startHealthMonitor(lc fx.Lifecycle, hm *infra.HealthMonitor, logger *slog.Logger) {
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
func startSpeedTest(lc fx.Lifecycle, st *infra.SpeedTestServer, cfg *config.Config, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			stCtx, cancel := context.WithCancel(context.Background())
			go func() {
				port := cfg.Infra.SpeedTestPort
				if port == 0 {
					port = infra.SpeedTestPort
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
func startSubscriptionProxy(lc fx.Lifecycle, sp *infra.SubscriptionProxy, cfg *config.Config, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			spCtx, cancel := context.WithCancel(context.Background())
			go func() {
				port := cfg.Infra.SubscriptionProxyPort
				if port == 0 {
					port = infra.SubscriptionProxyPort
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
