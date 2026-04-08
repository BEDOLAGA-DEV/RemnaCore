package tariff

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
)

func init() {
	builtin.RegisterPlugin(Plugin())
}

// Plugin returns the built-in plugin definition for the tariff-manager.
func Plugin() plugin.BuiltInPluginDef {
	return plugin.BuiltInPluginDef{
		Slug:        PluginSlug,
		Name:        "Tariff Manager",
		Version:     "1.0.0",
		Description: "Tariff management with flexible conditions. Create subscription plans with traffic limits, device limits, squad assignments, and custom pricing rules.",
		Author:      "RemnaCore",
		ConfigFields: map[string]plugin.ManifestConfigField{
			"default_currency": {
				Type:     "select",
				Label:    "Default Currency",
				Required: false,
				Default:  "USD",
				Options:  []string{"USD", "EUR", "RUB", "GBP", "TRY", "UAH", "KZT"},
			},
		},
		Pages: []plugin.ManifestPage{
			{
				Path:  "tariffs",
				Title: "Tariffs",
				Icon:  "CreditCard",
				Menu:  plugin.PageMenuAdmin,
			},
		},
		Routes: []plugin.ManifestRoute{
			{Method: "GET", Path: "/api/tariffs", Function: "list_tariffs", Public: true},
			{Method: "GET", Path: "/api/tariffs/{tariffID}", Function: "get_tariff", Public: true},
			{Method: "POST", Path: "/api/tariffs", Function: "create_tariff", Public: false},
			{Method: "PUT", Path: "/api/tariffs/{tariffID}", Function: "update_tariff", Public: false},
			{Method: "DELETE", Path: "/api/tariffs/{tariffID}", Function: "delete_tariff", Public: false},
			{Method: "GET", Path: "/api/tariffs/squads", Function: "list_squads", Public: false},
			{Method: "GET", Path: "/api/tariffs/nodes", Function: "list_nodes", Public: false},
		},
	}
}

// RegisterRoutes maps the plugin's manifest route functions to native Go handlers.
func RegisterRoutes(registry *gateway.BuiltinRouteRegistry, h *Handler) {
	registry.Register(PluginSlug, "list_tariffs", h.ListTariffs)
	registry.Register(PluginSlug, "get_tariff", h.GetTariff)
	registry.Register(PluginSlug, "create_tariff", h.CreateTariff)
	registry.Register(PluginSlug, "update_tariff", h.UpdateTariff)
	registry.Register(PluginSlug, "delete_tariff", h.DeleteTariff)
	registry.Register(PluginSlug, "list_squads", h.ListSquads)
	registry.Register(PluginSlug, "list_nodes", h.ListNodes)
}
