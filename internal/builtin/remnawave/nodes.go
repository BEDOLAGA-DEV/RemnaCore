package remnawave

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	rwclient "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// ListNodes returns all nodes across all active panels.
func (h *Handler) ListNodes(w http.ResponseWriter, r *http.Request) {
	clients, err := h.getAllPanelClients(r.Context())
	if err != nil || len(clients) == 0 {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	type nodeWithPanel struct {
		PanelID string `json:"panel_id"`
		Node    any    `json:"node"`
	}

	allNodes := make([]nodeWithPanel, 0)
	for panelID, client := range clients {
		nodes, err := client.GetNodes(r.Context())
		if err != nil {
			h.logger.Warn("failed to get nodes from panel", slog.String("panel_id", panelID), slog.Any("error", err))
			continue
		}
		for _, n := range nodes {
			allNodes = append(allNodes, nodeWithPanel{PanelID: panelID, Node: n})
		}
	}

	writeJSON(w, http.StatusOK, allNodes)
}

// GetNode returns details of a specific node from a panel.
func (h *Handler) GetNode(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	nodeUUID := chi.URLParam(r, "nodeUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	node, err := client.GetNodeByUUID(r.Context(), nodeUUID)
	if err != nil {
		h.logger.Error("failed to get node", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"panel_id": panelID,
		"node":     node,
	})
}

// RestartNode sends a restart command to a specific node.
func (h *Handler) RestartNode(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	nodeUUID := chi.URLParam(r, "nodeUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	if err := client.RestartNode(r.Context(), nodeUUID); err != nil {
		h.logger.Error("failed to restart node", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"panel_id":  panelID,
		"node_uuid": nodeUUID,
		"action":    "restarted",
	})
}

// DisableNode disables a node on a panel.
func (h *Handler) DisableNode(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	nodeUUID := chi.URLParam(r, "nodeUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	if err := client.DisableNode(r.Context(), nodeUUID); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "disabled"})
}

// EnableNode enables a node on a panel.
func (h *Handler) EnableNode(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	nodeUUID := chi.URLParam(r, "nodeUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	if err := client.EnableNode(r.Context(), nodeUUID); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "enabled"})
}

// GetNodeStats returns bandwidth stats for a node.
func (h *Handler) GetNodeStats(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	nodeUUID := chi.URLParam(r, "nodeUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	bandwidth, err := client.GetNodeUsersBandwidth(r.Context(), nodeUUID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"panel_id":  panelID,
			"node_uuid": nodeUUID,
			"error":     err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"panel_id":  panelID,
		"node_uuid": nodeUUID,
		"bandwidth": bandwidth,
	})
}

// GetNodeHealth returns health check history for a node.
func (h *Handler) GetNodeHealth(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	nodeUUID := chi.URLParam(r, "nodeUUID")

	// Return health log from collection
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionNodeHealthLog)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"entries": []any{}})
		return
	}

	entries := make([]NodeHealthEntry, 0)
	for _, doc := range docs {
		var entry NodeHealthEntry
		if json.Unmarshal(doc.Data, &entry) != nil {
			continue
		}
		if entry.PanelID == panelID && entry.NodeUUID == nodeUUID {
			entries = append(entries, entry)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"panel_id":  panelID,
		"node_uuid": nodeUUID,
		"entries":   entries,
	})
}

// GetNodeUsers returns active users on a specific node.
func (h *Handler) GetNodeUsers(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	nodeUUID := chi.URLParam(r, "nodeUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	bandwidth, err := client.GetNodeUsersBandwidth(r.Context(), nodeUUID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"users": []any{}})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"panel_id":  panelID,
		"node_uuid": nodeUUID,
		"users":     bandwidth,
	})
}

// BulkNodeAction performs a bulk action on multiple nodes of a panel.
func (h *Handler) BulkNodeAction(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req struct {
		Action string   `json:"action"` // "restart" | "enable" | "disable"
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
		writeAPIError(w, apierror.ValidationFailed.WithDetails(fmt.Sprintf("maximum %d UUIDs per request", maxBulkSize)))
		return
	}

	results := make([]map[string]any, 0, len(req.UUIDs))
	for _, uuid := range req.UUIDs {
		var actionErr error
		switch req.Action {
		case "restart":
			actionErr = client.RestartNode(r.Context(), uuid)
		case "enable":
			actionErr = client.EnableNode(r.Context(), uuid)
		case "disable":
			actionErr = client.DisableNode(r.Context(), uuid)
		default:
			actionErr = fmt.Errorf("unsupported action: %s", req.Action)
		}

		status := "ok"
		errMsg := ""
		if actionErr != nil {
			status = "error"
			errMsg = actionErr.Error()
		}
		results = append(results, map[string]any{
			"uuid":   uuid,
			"status": status,
			"error":  errMsg,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"panel_id": panelID,
		"action":   req.Action,
		"results":  results,
	})
}

// CreateNode creates a new node on a panel.
func (h *Handler) CreateNode(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req rwclient.CreateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	node, err := client.CreateNode(r.Context(), req)
	if err != nil {
		h.logger.Error("failed to create node", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"panel_id": panelID, "node": node})
}

// UpdateNode updates a node on a panel.
func (h *Handler) UpdateNode(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req rwclient.UpdateNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	node, err := client.UpdateNode(r.Context(), req)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "node": node})
}

// DeleteNode deletes a node from a panel.
func (h *Handler) DeleteNode(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	nodeUUID := chi.URLParam(r, "nodeUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	if err := client.DeleteNode(r.Context(), nodeUUID); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ResetNodeTraffic resets traffic for a node.
func (h *Handler) ResetNodeTraffic(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	nodeUUID := chi.URLParam(r, "nodeUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	if err := client.ResetNodeTraffic(r.Context(), nodeUUID); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "traffic_reset"})
}

// RestartAllNodes restarts all nodes on a panel.
func (h *Handler) RestartAllNodes(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	if err := client.RestartAllNodes(r.Context()); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "all_restarted"})
}

// ReorderNodes reorders nodes on a panel.
func (h *Handler) ReorderNodes(w http.ResponseWriter, r *http.Request) {
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

	if err := client.ReorderNodes(r.Context(), req.UUIDs); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "reordered"})
}

// GetNodeTags returns all node tags from a panel.
func (h *Handler) GetNodeTags(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	tags, err := client.GetNodeTags(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"tags": []string{}})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "tags": tags})
}

// GetSystemInfo returns system info from a panel (metadata, stats, recap).
func (h *Handler) GetSystemInfo(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	result := map[string]any{"panel_id": panelID}

	if meta, err := client.GetSystemMetadata(r.Context()); err == nil {
		result["metadata"] = meta
	}
	if stats, err := client.GetSystemStats(r.Context()); err == nil {
		result["stats"] = stats
	}
	if recap, err := client.GetStatsRecap(r.Context()); err == nil {
		result["recap"] = recap
	}
	if nodesStats, err := client.GetNodesSystemStats(r.Context()); err == nil {
		result["nodes_stats"] = nodesStats
	}
	if metrics, err := client.GetNodesMetrics(r.Context()); err == nil {
		result["node_metrics"] = metrics
	}

	writeJSON(w, http.StatusOK, result)
}

// FetchNodeUsersIPs starts an async IP fetch for all users on a node.
func (h *Handler) FetchNodeUsersIPs(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	nodeUUID := chi.URLParam(r, "nodeUUID")
	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}
	job, err := client.FetchNodeUsersIPs(r.Context(), nodeUUID)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "job": job})
}

// GetFetchNodeUsersIPsResult polls the result of a node-level IP fetch job.
func (h *Handler) GetFetchNodeUsersIPsResult(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	jobID := chi.URLParam(r, "jobID")
	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}
	result, err := client.GetFetchNodeUsersIPsResult(r.Context(), jobID)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "result": result})
}
