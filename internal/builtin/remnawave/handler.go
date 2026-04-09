// Package remnawave implements the remnawave-provider built-in plugin handlers.
// The plugin definition itself lives in internal/plugin/builtin.go — this
// package only provides the native Go HTTP handlers for its routes.
package remnawave

import (
	"log/slog"
	"net/http"

	rwclient "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
)

// Handler provides HTTP endpoints for the remnawave-provider built-in plugin.
type Handler struct {
	client *rwclient.Client
	logger *slog.Logger
}

// NewHandler creates a remnawave-provider Handler.
func NewHandler(client *rwclient.Client, logger *slog.Logger) *Handler {
	return &Handler{client: client, logger: logger}
}

// TestConnection verifies connectivity to the configured Remnawave panel by
// calling the nodes endpoint. Returns a JSON response indicating success or
// failure without exposing internal details beyond the error message.
func (h *Handler) TestConnection(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.client.GetNodes(r.Context())
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
