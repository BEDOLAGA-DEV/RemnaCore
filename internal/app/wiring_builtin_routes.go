package app

import (
	balancebuiltin "github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/balance"
	checkoutbuiltin "github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/checkout"
	rwbuiltin "github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/tariff"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway"
	"go.uber.org/fx"

	// Built-in plugins self-register via init().
	// Add a blank import for each new built-in plugin package here.
	_ "github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/balance"
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
