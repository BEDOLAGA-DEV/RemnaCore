package app

import (
	"context"
	"log/slog"

	"go.uber.org/fx"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
)

// pluginWiring provides all plugin-domain bindings: repository, storage,
// WASM factory, hook dispatcher, and the startup loader.
var pluginWiring = fx.Options(
	// Plugin domain module
	plugin.Module,

	// Hook dispatcher -> interface binding (domain packages depend on the interface)
	fx.Provide(func(d *plugin.HookDispatcher) hookdispatch.Dispatcher { return d }),

	// Plugin repos -> interface bindings
	fx.Provide(postgres.NewPluginRepository),
	fx.Provide(func(repo *postgres.PluginRepository) plugin.PluginRepository { return repo }),
	fx.Provide(func(pool *pgxpool.Pool, txRunner *postgres.TxManager) plugin.StorageService {
		return postgres.NewPluginStorageRepository(pool, txRunner, plugin.DefaultMaxStorageMB)
	}),

	// WASM runner factory — real Extism/wazero runtime.
	fx.Provide(provideExtismWASMFactory),

	// Load enabled plugins on startup
	fx.Invoke(loadEnabledPlugins),
)

// provideExtismWASMFactory returns a WASMRunnerFactory backed by the Extism Go
// SDK (wazero runtime). Plugins are loaded with WASI support and receive their
// config via the Extism manifest config mechanism.
func provideExtismWASMFactory() plugin.WASMRunnerFactory {
	return plugin.ExtismRunnerFactory()
}

// loadEnabledPlugins bootstraps the plugin runtime by loading every plugin that
// was enabled before the last shutdown.
func loadEnabledPlugins(lc fx.Lifecycle, lm *plugin.LifecycleManager, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := lm.LoadAllEnabled(ctx); err != nil {
				logger.Error("failed to load enabled plugins on startup", slog.Any("error", err))
				// Non-fatal — the platform can still operate without plugins.
			}
			return nil
		},
	})
}
