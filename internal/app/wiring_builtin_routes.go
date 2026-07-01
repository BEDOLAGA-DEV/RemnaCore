package app

import (
	"log/slog"

	balancebuiltin "github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/balance"
	checkoutbuiltin "github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/checkout"
	rwbuiltin "github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/tariff"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
	"go.uber.org/fx"

	// Built-in plugins self-register via init().
	// Add a blank import for each new built-in plugin package here.
	_ "github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/balance"
	_ "github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/cabinetbot"
	_ "github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/checkout"
	_ "github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/remnawave"
	_ "github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/tariff"
)

// builtinRoutesWiring provides the BuiltinRouteRegistry and all built-in
// plugin handlers. Adding a new built-in plugin requires:
//  1. Create internal/builtin/<name>/ with init() calling builtin.RegisterPlugin()
//  2. Add blank import above: _ "github.com/.../internal/builtin/<name>"
//  3. Add fx.Provide(<name>.NewHandler) and route registration below
var builtinRoutesWiring = fx.Options(
	fx.Provide(gateway.NewBuiltinRouteRegistry),
	fx.Provide(tariff.NewHandler),
	fx.Provide(rwbuiltin.NewHandler),
	fx.Provide(balancebuiltin.NewHandler),
	fx.Provide(checkoutbuiltin.NewHandler),
	fx.Invoke(registerBuiltinPluginRoutes),
	// Domain-reader adapters for the bot-plugin op-catalog (BP3).
	// BalanceService is a separate value from the balance Handler; it is
	// provided here because its dependency (pluginstore.Store) is in this graph.
	fx.Provide(balancebuiltin.NewBalanceService),
	fx.Provide(balancebuiltin.NewBalanceReaderAdapter),
	// TariffReaderAdapter's constructor returns the concrete type; wrap it so fx
	// sees the bothost.TariffReader interface type for type-directed injection.
	fx.Provide(func(h *tariff.Handler, logger *slog.Logger) bothost.TariffReader {
		return tariff.NewTariffReaderAdapter(h, logger)
	}),
)

func registerBuiltinPluginRoutes(
	registry *gateway.BuiltinRouteRegistry,
	tariffH *tariff.Handler,
	rwH *rwbuiltin.Handler,
	balanceH *balancebuiltin.Handler,
	checkoutH *checkoutbuiltin.Handler,
) {
	tariff.RegisterRoutes(registry, tariffH)
	rwbuiltin.RegisterRoutes(registry, rwH)
	balancebuiltin.RegisterRoutes(registry, balanceH)
	checkoutbuiltin.RegisterRoutes(registry, checkoutH)
}
