package app

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/tariff"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway"
	"go.uber.org/fx"

	// Built-in plugins self-register via init().
	// Add a blank import for each new built-in plugin package here.
	_ "github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/tariff"
)

// builtinRoutesWiring provides the BuiltinRouteRegistry and all built-in
// plugin handlers. Adding a new built-in plugin requires:
//   1. Create internal/builtin/<name>/ with init() calling builtin.RegisterPlugin()
//   2. Add blank import above: _ "github.com/.../internal/builtin/<name>"
//   3. Add fx.Provide(<name>.NewHandler) and route registration below
var builtinRoutesWiring = fx.Options(
	fx.Provide(gateway.NewBuiltinRouteRegistry),
	fx.Provide(tariff.NewHandler),
	fx.Invoke(registerBuiltinPluginRoutes),
)

func registerBuiltinPluginRoutes(registry *gateway.BuiltinRouteRegistry, h *tariff.Handler) {
	tariff.RegisterRoutes(registry, h)
}
