package remnawave

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	rwclient "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// ListInternalSquads returns internal squads from all active panels.
func (h *Handler) ListInternalSquads(w http.ResponseWriter, r *http.Request) {
	clients, err := h.getAllPanelClients(r.Context())
	if err != nil || len(clients) == 0 {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	type squadWithPanel struct {
		PanelID string `json:"panel_id"`
		Squad   any    `json:"squad"`
	}

	all := make([]squadWithPanel, 0)
	for panelID, client := range clients {
		squads, err := client.GetInternalSquads(r.Context())
		if err != nil {
			h.logger.Warn("failed to get internal squads", slog.String("panel_id", panelID), slog.Any("error", err))
			continue
		}
		for _, s := range squads {
			all = append(all, squadWithPanel{PanelID: panelID, Squad: s})
		}
	}

	writeJSON(w, http.StatusOK, all)
}

// ListExternalSquads returns external squads from all active panels.
func (h *Handler) ListExternalSquads(w http.ResponseWriter, r *http.Request) {
	clients, err := h.getAllPanelClients(r.Context())
	if err != nil || len(clients) == 0 {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	type squadWithPanel struct {
		PanelID string `json:"panel_id"`
		Squad   any    `json:"squad"`
	}

	all := make([]squadWithPanel, 0)
	for panelID, client := range clients {
		squads, err := client.GetExternalSquads(r.Context())
		if err != nil {
			h.logger.Warn("failed to get external squads", slog.String("panel_id", panelID), slog.Any("error", err))
			continue
		}
		for _, s := range squads {
			all = append(all, squadWithPanel{PanelID: panelID, Squad: s})
		}
	}

	writeJSON(w, http.StatusOK, all)
}

// CreateSquad creates a new squad on a panel (internal type by default).
func (h *Handler) CreateSquad(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req rwclient.CreateSquadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	squad, err := client.CreateInternalSquad(r.Context(), req)
	if err != nil {
		h.logger.Error("failed to create squad", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"panel_id": panelID,
		"squad":    squad,
	})
}

// UpdateSquad updates a squad on a panel.
func (h *Handler) UpdateSquad(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	squadUUID := chi.URLParam(r, "squadUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req rwclient.UpdateSquadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}
	req.UUID = squadUUID

	squad, err := client.UpdateInternalSquad(r.Context(), req)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"panel_id": panelID,
		"squad":    squad,
	})
}

// DeleteSquad deletes a squad from a panel.
func (h *Handler) DeleteSquad(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	squadUUID := chi.URLParam(r, "squadUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	if err := client.DeleteInternalSquad(r.Context(), squadUUID); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AddNodesToSquad adds nodes to a squad.
func (h *Handler) AddNodesToSquad(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	squadUUID := chi.URLParam(r, "squadUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req struct {
		UserIDs []int64 `json:"user_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := client.AddUsersToInternalSquad(r.Context(), squadUUID, req.UserIDs); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "nodes_added"})
}

// RemoveNodesFromSquad removes nodes from a squad.
func (h *Handler) RemoveNodesFromSquad(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	squadUUID := chi.URLParam(r, "squadUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req struct {
		UserIDs []int64 `json:"user_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := client.RemoveUsersFromInternalSquad(r.Context(), squadUUID, req.UserIDs); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "nodes_removed"})
}

// ReorderSquad reorders nodes within a squad.
func (h *Handler) ReorderSquad(w http.ResponseWriter, r *http.Request) {
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

	if err := client.ReorderInternalSquads(r.Context(), req.UUIDs); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "reordered"})
}

// =====================================================================
// External squad operations
// =====================================================================

// CreateExternalSquad creates a new external squad on a panel.
func (h *Handler) CreateExternalSquad(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}
	var req rwclient.CreateSquadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}
	squad, err := client.CreateExternalSquad(r.Context(), req)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"panel_id": panelID, "squad": squad})
}

// UpdateExternalSquad updates an external squad.
func (h *Handler) UpdateExternalSquad(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	squadUUID := chi.URLParam(r, "squadUUID")
	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}
	var req rwclient.UpdateSquadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}
	req.UUID = squadUUID
	squad, err := client.UpdateExternalSquad(r.Context(), req)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "squad": squad})
}

// DeleteExternalSquad deletes an external squad.
func (h *Handler) DeleteExternalSquad(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	squadUUID := chi.URLParam(r, "squadUUID")
	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}
	if err := client.DeleteExternalSquad(r.Context(), squadUUID); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetSquadByUUID returns a single internal or external squad by UUID.
func (h *Handler) GetSquadByUUID(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	squadUUID := chi.URLParam(r, "squadUUID")
	squadType := r.URL.Query().Get("type") // "internal" or "external"
	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}
	if squadType == "external" {
		squad, err := client.GetExternalSquadByUUID(r.Context(), squadUUID)
		if err != nil {
			writeAPIError(w, apierror.Internal)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "type": "external", "squad": squad})
		return
	}
	squad, err := client.GetInternalSquadByUUID(r.Context(), squadUUID)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "type": "internal", "squad": squad})
}

// GetSquadNodes returns accessible nodes for a squad.
func (h *Handler) GetSquadNodes(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	squadUUID := chi.URLParam(r, "squadUUID")
	squadType := r.URL.Query().Get("type")
	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}
	if squadType == "external" {
		nodes, err := client.GetExternalSquadNodes(r.Context(), squadUUID)
		if err != nil {
			writeAPIError(w, apierror.Internal)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "nodes": nodes})
		return
	}
	nodes, err := client.GetInternalSquadNodes(r.Context(), squadUUID)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "nodes": nodes})
}

// AddUsersToExternalSquad adds users to an external squad.
func (h *Handler) AddUsersToExternalSquad(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	squadUUID := chi.URLParam(r, "squadUUID")
	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}
	var req struct {
		UserIDs []int64 `json:"user_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}
	if err := client.AddUsersToExternalSquad(r.Context(), squadUUID, req.UserIDs); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"action": "users_added"})
}

// RemoveUsersFromExternalSquad removes users from an external squad.
func (h *Handler) RemoveUsersFromExternalSquad(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	squadUUID := chi.URLParam(r, "squadUUID")
	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}
	var req struct {
		UserIDs []int64 `json:"user_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}
	if err := client.RemoveUsersFromExternalSquad(r.Context(), squadUUID, req.UserIDs); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"action": "users_removed"})
}

// ReorderExternalSquads reorders external squads on a panel.
func (h *Handler) ReorderExternalSquads(w http.ResponseWriter, r *http.Request) {
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
	if err := client.ReorderExternalSquads(r.Context(), req.UUIDs); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"action": "reordered"})
}
