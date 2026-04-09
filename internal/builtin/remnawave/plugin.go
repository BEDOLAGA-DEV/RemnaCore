package remnawave

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
)

// PluginSlug is the canonical slug for the remnawave-provider plugin.
// It must match plugin.BuiltInSlugRemnawaveProvider.
const PluginSlug = plugin.BuiltInSlugRemnawaveProvider

// RegisterRoutes maps the plugin's manifest route functions to native Go handlers.
func RegisterRoutes(registry *gateway.BuiltinRouteRegistry, h *Handler) {
	registry.Register(PluginSlug, "test_connection", h.TestConnection)
}
