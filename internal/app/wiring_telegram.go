package app

import (
	"context"
	"log/slog"

	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram"
)

// telegramWiring provides the Telegram bot manager lifecycle: loads all bots
// on OnStart and stops them on OnStop.
var telegramWiring = fx.Options(
	// Telegram bot module
	telegram.Module,

	// Telegram bot manager lifecycle
	fx.Invoke(startBotManager),
)

// startBotManager loads and runs all Telegram bots (platform + per-shop) under
// the Fx lifecycle. A per-shop load error is logged, never fatal to boot.
func startBotManager(lc fx.Lifecycle, mgr *telegram.BotManager, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := mgr.Load(ctx); err != nil {
				logger.Error("telegram bot manager load failed", slog.Any("error", err))
			}
			mgr.Run(context.Background())
			return nil
		},
		OnStop: func(_ context.Context) error {
			logger.Info("telegram bot manager stopping")
			mgr.Stop()
			return nil
		},
	})
}
