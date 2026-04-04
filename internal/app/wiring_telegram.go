package app

import (
	"context"
	"log/slog"

	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram"
)

// telegramWiring provides the Telegram bot lifecycle: starts polling on OnStart
// and cancels on OnStop.
var telegramWiring = fx.Options(
	// Telegram bot module
	telegram.Module,

	// Telegram bot lifecycle
	fx.Invoke(startTelegramBot),
)

// startTelegramBot starts the Telegram bot as a background goroutine managed
// by the Fx lifecycle.
func startTelegramBot(lc fx.Lifecycle, bot *telegram.Bot, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			botCtx, cancel := context.WithCancel(context.Background())
			go func() {
				if err := bot.Start(botCtx); err != nil {
					logger.Error("telegram bot error", slog.Any("error", err))
				}
			}()
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logger.Info("telegram bot stopping")
					cancel()
					return nil
				},
			})
			return nil
		},
	})
}
