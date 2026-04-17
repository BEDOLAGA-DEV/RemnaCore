package remnawave

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	rwclient "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// ListNodePlugins returns all node plugins from a panel.
func (h *Handler) ListNodePlugins(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	plugins, err := client.GetNodePlugins(r.Context())
	if err != nil {
		h.logger.Error("failed to get node plugins", slog.Any("error", err))
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "plugins": plugins})
}

// GetNodePlugin returns a single node plugin.
func (h *Handler) GetNodePlugin(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	pluginUUID := chi.URLParam(r, "pluginUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	np, err := client.GetNodePluginByUUID(r.Context(), pluginUUID)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "plugin": np})
}

// CreateNodePlugin creates a new node plugin.
func (h *Handler) CreateNodePlugin(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req rwclient.CreateNodePluginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	np, err := client.CreateNodePlugin(r.Context(), req)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"panel_id": panelID, "plugin": np})
}

// UpdateNodePlugin updates an existing node plugin.
func (h *Handler) UpdateNodePlugin(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req rwclient.UpdateNodePluginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	np, err := client.UpdateNodePlugin(r.Context(), req)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "plugin": np})
}

// DeleteNodePlugin deletes a node plugin.
func (h *Handler) DeleteNodePlugin(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	pluginUUID := chi.URLParam(r, "pluginUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	if err := client.DeleteNodePlugin(r.Context(), pluginUUID); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ReorderNodePlugins reorders node plugins.
func (h *Handler) ReorderNodePlugins(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req struct {
		UUIDs []string `json:"uuids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := client.ReorderNodePlugins(r.Context(), req.UUIDs); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "reordered"})
}

// CloneNodePlugin clones a node plugin.
func (h *Handler) CloneNodePlugin(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	pluginUUID := chi.URLParam(r, "pluginUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	np, err := client.CloneNodePlugin(r.Context(), pluginUUID)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"panel_id": panelID, "plugin": np})
}
