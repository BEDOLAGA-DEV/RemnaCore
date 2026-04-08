package app

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/handler"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"go.uber.org/fx"
)

// builtinRoutesWiring provides the BuiltinRouteRegistry and registers all
// native Go handlers for built-in plugin routes.
var builtinRoutesWiring = fx.Options(
	fx.Provide(gateway.NewBuiltinRouteRegistry),
	fx.Invoke(registerTariffRoutes),
)

// registerTariffRoutes maps tariff-manager manifest route functions to the
// native TariffHandler methods.
func registerTariffRoutes(registry *gateway.BuiltinRouteRegistry, h *handler.TariffHandler) {
	slug := plugin.BuiltInSlugTariffManager
	registry.Register(slug, "list_tariffs", h.ListTariffs)
	registry.Register(slug, "get_tariff", h.GetTariff)
	registry.Register(slug, "create_tariff", h.CreateTariff)
	registry.Register(slug, "update_tariff", h.UpdateTariff)
	registry.Register(slug, "delete_tariff", h.DeleteTariff)
	registry.Register(slug, "list_squads", h.ListSquads)
	registry.Register(slug, "list_nodes", h.ListNodes)
}
