package app

import (
	"context"
	"log/slog"

	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/settings"
)

// settingsWiring provides the runtime settings override infrastructure:
// repository, service, and a startup hook that loads persisted overrides
// onto the shared *config.Config before other modules read it.
var settingsWiring = fx.Options(
	fx.Provide(postgres.NewSettingsRepository),
	fx.Provide(func(repo *postgres.SettingsRepository) settings.OverrideRepository { return repo }),
	fx.Provide(settings.NewService),
	fx.Invoke(loadSettingsOverrides),
)

// loadSettingsOverrides reads persisted runtime overrides from the database
// and applies them to the shared config on application start. This runs
// early in the startup sequence (before domain services read config values)
// because settingsWiring is declared before other domain wiring modules.
func loadSettingsOverrides(lc fx.Lifecycle, svc *settings.Service, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := svc.LoadOverrides(ctx); err != nil {
				logger.Error("failed to load settings overrides from database", slog.Any("error", err))
				// Non-fatal — the platform can still operate with env-var defaults.
			}
			return nil
		},
	})
}
