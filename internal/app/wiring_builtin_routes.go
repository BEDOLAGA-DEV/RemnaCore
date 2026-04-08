package app

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/tariff"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway"
	"go.uber.org/fx"
)

// builtinRoutesWiring provides the BuiltinRouteRegistry and registers all
// built-in plugin handlers. Each plugin package owns its own registration.
var builtinRoutesWiring = fx.Options(
	fx.Provide(gateway.NewBuiltinRouteRegistry),
	fx.Provide(tariff.NewHandler),
	fx.Invoke(registerBuiltinPluginRoutes),
)

// registerBuiltinPluginRoutes delegates route registration to each built-in
// plugin package. The platform doesn't know about tariff internals.
func registerBuiltinPluginRoutes(registry *gateway.BuiltinRouteRegistry, h *tariff.Handler) {
	tariff.RegisterRoutes(registry, h)
}
