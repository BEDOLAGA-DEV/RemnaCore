package remnawave

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	rwclient "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// ListHosts returns all hosts across all active panels.
func (h *Handler) ListHosts(w http.ResponseWriter, r *http.Request) {
	clients, err := h.getAllPanelClients(r.Context())
	if err != nil || len(clients) == 0 {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	type hostWithPanel struct {
		PanelID string `json:"panel_id"`
		Host    any    `json:"host"`
	}

	all := make([]hostWithPanel, 0)
	for panelID, client := range clients {
		hosts, err := client.GetAllHosts(r.Context())
		if err != nil {
			h.logger.Warn("failed to get hosts", slog.String("panel_id", panelID), slog.Any("error", err))
			continue
		}
		for _, host := range hosts {
			all = append(all, hostWithPanel{PanelID: panelID, Host: host})
		}
	}

	writeJSON(w, http.StatusOK, all)
}

// GetHost returns a specific host from a panel.
func (h *Handler) GetHost(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	hostUUID := chi.URLParam(r, "hostUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	host, err := client.GetHostByUUID(r.Context(), hostUUID)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "host": host})
}

// CreateHost creates a new host on a panel.
func (h *Handler) CreateHost(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req rwclient.CreateHostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	host, err := client.CreateHost(r.Context(), req)
	if err != nil {
		h.logger.Error("failed to create host", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"panel_id": panelID, "host": host})
}

// UpdateHost updates a host on a panel.
func (h *Handler) UpdateHost(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req rwclient.UpdateHostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	host, err := client.UpdateHost(r.Context(), req)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "host": host})
}

// DeleteHost deletes a host from a panel.
func (h *Handler) DeleteHost(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	hostUUID := chi.URLParam(r, "hostUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	if err := client.DeleteHost(r.Context(), hostUUID); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ReorderHosts reorders hosts on a panel.
func (h *Handler) ReorderHosts(w http.ResponseWriter, r *http.Request) {
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

	if err := client.ReorderHosts(r.Context(), req.UUIDs); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "reordered"})
}

// GetHostTags returns all host tags from a panel.
func (h *Handler) GetHostTags(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	tags, err := client.GetHostTags(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"tags": []string{}})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "tags": tags})
}

// BulkHostAction performs bulk enable/disable/delete on hosts.
func (h *Handler) BulkHostAction(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req struct {
		Action string   `json:"action"` // "enable" | "disable" | "delete"
		UUIDs  []string `json:"uuids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if len(req.UUIDs) == 0 {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("uuids is required"))
		return
	}
	const maxBulkSize = 100
	if len(req.UUIDs) > maxBulkSize {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("maximum 100 UUIDs per request"))
		return
	}

	var actionErr error
	switch req.Action {
	case "enable":
		actionErr = client.BulkEnableHosts(r.Context(), req.UUIDs)
	case "disable":
		actionErr = client.BulkDisableHosts(r.Context(), req.UUIDs)
	case "delete":
		actionErr = client.BulkDeleteHosts(r.Context(), req.UUIDs)
	default:
		writeAPIError(w, apierror.ValidationFailed.WithDetails("action must be enable, disable, or delete"))
		return
	}

	if actionErr != nil {
		h.logger.Error("bulk host action failed", slog.Any("error", actionErr))
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"panel_id": panelID,
		"action":   req.Action,
		"count":    len(req.UUIDs),
	})
}
