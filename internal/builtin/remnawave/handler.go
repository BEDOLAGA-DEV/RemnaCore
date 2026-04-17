// Package remnawave implements the remnawave-provider built-in plugin handlers.
// This plugin exposes Remnawave VPN panel management through the RemnaCore admin UI,
// supporting multi-panel deployments, node monitoring, user management, and analytics.
package remnawave

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	rwclient "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
)

// Handler provides HTTP endpoints for the remnawave-provider built-in plugin.
type Handler struct {
	collections pluginstore.Store
	pluginRepo  plugin.PluginRepository
	logger      *slog.Logger
}

// NewHandler creates a remnawave-provider Handler.
func NewHandler(collections pluginstore.Store, pluginRepo plugin.PluginRepository, logger *slog.Logger) *Handler {
	return &Handler{
		collections: collections,
		pluginRepo:  pluginRepo,
		logger:      logger,
	}
}

// buildClientForPanel creates a Remnawave client from a panel connection document.
func (h *Handler) buildClientForPanel(ctx context.Context, panelID string) (*rwclient.Client, error) {
	doc, err := h.collections.GetDocument(ctx, PluginSlug, CollectionPanelConnections, panelID)
	if err != nil {
		return nil, fmt.Errorf("panel %s not found: %w", panelID, err)
	}

	var panel PanelConnectionInput
	if err := json.Unmarshal(doc.Data, &panel); err != nil {
		return nil, fmt.Errorf("unmarshal panel config: %w", err)
	}

	if panel.Status == PanelStatusDisabled {
		return nil, fmt.Errorf("panel %s is disabled", panelID)
	}

	return rwclient.NewClient(panel.URL, panel.APIToken), nil
}

// buildDefaultClient creates a client from the legacy plugin config or the first active panel.
func (h *Handler) buildDefaultClient(ctx context.Context) (*rwclient.Client, error) {
	// Try panel_connections first
	docs, err := h.collections.ListDocuments(ctx, PluginSlug, CollectionPanelConnections)
	if err == nil && len(docs) > 0 {
		for _, doc := range docs {
			var panel PanelConnectionInput
			if json.Unmarshal(doc.Data, &panel) == nil && panel.Status == PanelStatusActive {
				return rwclient.NewClient(panel.URL, panel.APIToken), nil
			}
		}
	}

	// Fallback: legacy plugin config
	p, err := h.pluginRepo.GetBySlug(ctx, plugin.BuiltInSlugRemnawaveProvider)
	if err != nil {
		return nil, fmt.Errorf("plugin not configured: %w", err)
	}

	url := p.Config[plugin.RemnawaveConfigKeyURL]
	token := p.Config[plugin.RemnawaveConfigKeyAPIToken]
	if url == "" || token == "" {
		return nil, fmt.Errorf("panel URL or API token not configured")
	}

	return rwclient.NewClient(url, token), nil
}

// getAllPanelClients returns clients for all active panels.
func (h *Handler) getAllPanelClients(ctx context.Context) (map[string]*rwclient.Client, error) {
	docs, err := h.collections.ListDocuments(ctx, PluginSlug, CollectionPanelConnections)
	if err != nil {
		return nil, err
	}

	clients := make(map[string]*rwclient.Client)
	for _, doc := range docs {
		var panel PanelConnectionInput
		if json.Unmarshal(doc.Data, &panel) != nil {
			continue
		}
		if panel.Status != PanelStatusActive && panel.Status != PanelStatusReadonly {
			continue
		}
		clients[doc.ID] = rwclient.NewClient(panel.URL, panel.APIToken)
	}

	// Fallback: legacy config as "default" panel
	if len(clients) == 0 {
		client, err := h.buildDefaultClient(ctx)
		if err == nil {
			clients["default"] = client
		}
	}

	return clients, nil
}

// TestConnection verifies connectivity to a specific panel or the default panel.
func (h *Handler) TestConnection(w http.ResponseWriter, r *http.Request) {
	client, err := h.buildDefaultClient(r.Context())
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

// --- Helpers ---

// maskToken returns a masked API token for display (last 4 chars visible).
func maskToken(token string) string {
	if len(token) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(token)-4) + token[len(token)-4:]
}

// nowRFC3339 returns the current time formatted as RFC3339.
func nowRFC3339() string {
	return time.Now().Format(time.RFC3339)
}
