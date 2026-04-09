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
				Path:       "tariffs",
				Title:      "Tariffs",
				Icon:       "CreditCard",
				Menu:       plugin.PageMenuAdmin,
				Collection: "tariffs",
				Fields: []plugin.ManifestPageField{
					{Key: "name", Label: "Name", Type: "text", Required: true},
					{Key: "description", Label: "Description", Type: "textarea"},
					{Key: "price_amount", Label: "Price (cents)", Type: "number", Required: true, Default: "0"},
					{Key: "price_currency", Label: "Currency", Type: "select", Required: true, Default: "USD", Options: []string{"USD", "EUR", "RUB", "GBP", "TRY", "UAH", "KZT"}},
					{Key: "duration_days", Label: "Duration (days)", Type: "number", Required: true, Default: "30"},
					{Key: "traffic_limit_gb", Label: "Traffic (GB, 0=unlimited)", Type: "number", Default: "0"},
					{Key: "device_limit", Label: "Devices (0=unlimited)", Type: "number", Default: "0"},
					{Key: "max_purchases_per_user", Label: "Max per user (0=unlimited)", Type: "number", Default: "0"},
					{Key: "internal_squad_uuids", Label: "Internal Squads", Type: "multiselect", OptionsURL: "/api/tariffs/internal-squads", OptionsValueKey: "uuid", OptionsLabelKey: "name"},
					{Key: "external_squad_uuids", Label: "External Squads", Type: "multiselect", OptionsURL: "/api/tariffs/external-squads", OptionsValueKey: "uuid", OptionsLabelKey: "name"},
					{Key: "features", Label: "Features (one per line)", Type: "textarea"},
					{Key: "is_active", Label: "Active", Type: "boolean", Default: "true"},
					{Key: "sort_order", Label: "Sort order", Type: "number", Default: "0"},
				},
			},
		},
		Routes: []plugin.ManifestRoute{
			// Static paths MUST come before parameterized paths.
			{Method: "GET", Path: "/api/tariffs/internal-squads", Function: "list_internal_squads", Public: false},
			{Method: "GET", Path: "/api/tariffs/external-squads", Function: "list_external_squads", Public: false},
			{Method: "GET", Path: "/api/tariffs/nodes", Function: "list_nodes", Public: false},
			{Method: "GET", Path: "/api/tariffs", Function: "list_tariffs", Public: true},
			{Method: "GET", Path: "/api/tariffs/{tariffID}", Function: "get_tariff", Public: true},
			{Method: "POST", Path: "/api/tariffs", Function: "create_tariff", Public: false},
			{Method: "PUT", Path: "/api/tariffs/{tariffID}", Function: "update_tariff", Public: false},
			{Method: "DELETE", Path: "/api/tariffs/{tariffID}", Function: "delete_tariff", Public: false},
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
	registry.Register(PluginSlug, "list_internal_squads", h.ListInternalSquads)
	registry.Register(PluginSlug, "list_external_squads", h.ListExternalSquads)
	registry.Register(PluginSlug, "list_nodes", h.ListNodes)
}
