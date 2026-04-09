// Package remnawave implements the remnawave-provider built-in plugin handlers.
package remnawave

import (
	"log/slog"
	"net/http"

	rwclient "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
)

// Handler provides HTTP endpoints for the remnawave-provider built-in plugin.
type Handler struct {
	pluginRepo plugin.PluginRepository
	logger     *slog.Logger
}

// NewHandler creates a remnawave-provider Handler.
func NewHandler(pluginRepo plugin.PluginRepository, logger *slog.Logger) *Handler {
	return &Handler{pluginRepo: pluginRepo, logger: logger}
}

// buildClient creates a temporary Remnawave client from the plugin config
// stored in the database. This ensures the client always uses the latest
// config values set via the admin UI.
func (h *Handler) buildClient(r *http.Request) (*rwclient.Client, error) {
	p, err := h.pluginRepo.GetBySlug(r.Context(), plugin.BuiltInSlugRemnawaveProvider)
	if err != nil {
		return nil, err
	}

	url := p.Config[plugin.RemnawaveConfigKeyURL]
	token := p.Config[plugin.RemnawaveConfigKeyAPIToken]

	return rwclient.NewClient(url, token), nil
}

// TestConnection verifies connectivity to the configured Remnawave panel by
// calling the nodes endpoint.
func (h *Handler) TestConnection(w http.ResponseWriter, r *http.Request) {
	client, err := h.buildClient(r)
	if err != nil {
		h.logger.Warn("failed to build remnawave client", slog.Any("error", err))
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   "Plugin not found or not configured",
		})
		return
	}

	nodes, err := client.GetNodes(r.Context())
	if err != nil {
		h.logger.Warn("remnawave connection test failed", slog.Any("error", err))
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"node_count": len(nodes),
		"message":    "Connection successful",
	})
}
