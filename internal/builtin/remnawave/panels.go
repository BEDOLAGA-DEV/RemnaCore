package remnawave

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
)

// ListPanels returns all panel connections with health status.
func (h *Handler) ListPanels(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionPanelConnections)
	if err != nil {
		h.logger.Error("failed to list panels", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	panels := make([]PanelConnectionResponse, 0, len(docs))
	for _, doc := range docs {
		var p PanelConnectionInput
		if json.Unmarshal(doc.Data, &p) != nil {
			continue
		}
		panels = append(panels, PanelConnectionResponse{
			ID:               doc.ID,
			PanelConnectionInput: PanelConnectionInput{
				Slug:          p.Slug,
				URL:           p.URL,
				APIToken:      maskToken(p.APIToken),
				Region:        p.Region,
				Priority:      p.Priority,
				Status:        p.Status,
				TLSSkipVerify: p.TLSSkipVerify,
			},
			CreatedAt: doc.CreatedAt.Format(time.RFC3339),
			UpdatedAt: doc.UpdatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, panels)
}

// CreatePanel adds a new panel connection.
func (h *Handler) CreatePanel(w http.ResponseWriter, r *http.Request) {
	var input PanelConnectionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := validatePanelConnectionInput(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(err.Error()))
		return
	}

	data, err := json.Marshal(input)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	doc, err := h.collections.InsertDocument(r.Context(), PluginSlug, CollectionPanelConnections, json.RawMessage(data))
	if err != nil {
		h.logger.Error("failed to create panel", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":        doc.ID,
		"slug":      input.Slug,
		"url":       input.URL,
		"status":    input.Status,
		"created_at": doc.CreatedAt.Format(time.RFC3339),
	})
}

// UpdatePanel updates an existing panel connection.
func (h *Handler) UpdatePanel(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	if panelID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("panel ID is required"))
		return
	}

	var input PanelConnectionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := validatePanelConnectionInput(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(err.Error()))
		return
	}

	data, err := json.Marshal(input)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	if err := h.collections.UpdateDocument(r.Context(), PluginSlug, CollectionPanelConnections, panelID, json.RawMessage(data)); err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// DeletePanel removes a panel connection (soft: sets status to disabled).
func (h *Handler) DeletePanel(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	if panelID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("panel ID is required"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionPanelConnections, panelID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	var input PanelConnectionInput
	if err := json.Unmarshal(doc.Data, &input); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	input.Status = PanelStatusDisabled

	data, err := json.Marshal(input)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	if err := h.collections.UpdateDocument(r.Context(), PluginSlug, CollectionPanelConnections, panelID, json.RawMessage(data)); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TestPanel tests connectivity to a specific panel.
func (h *Handler) TestPanel(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	if panelID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("panel ID is required"))
		return
	}

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	nodes, err := client.GetNodes(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	result := map[string]any{
		"success":    true,
		"panel_id":   panelID,
		"node_count": len(nodes),
	}

	// Fetch panel version from system metadata
	if meta, err := client.GetSystemMetadata(r.Context()); err == nil && meta != nil {
		result["version"] = meta.Version
	}

	writeJSON(w, http.StatusOK, result)
}

// SetPanelMaintenance puts a panel into maintenance mode.
func (h *Handler) SetPanelMaintenance(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	if panelID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("panel ID is required"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionPanelConnections, panelID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	var input PanelConnectionInput
	if err := json.Unmarshal(doc.Data, &input); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	input.Status = PanelStatusMaintenance

	data, err := json.Marshal(input)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	if err := h.collections.UpdateDocument(r.Context(), PluginSlug, CollectionPanelConnections, panelID, json.RawMessage(data)); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"panel_id": panelID,
		"status":   PanelStatusMaintenance,
	})
}

// GetPanelStats returns live stats for a specific panel.
func (h *Handler) GetPanelStats(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	if panelID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("panel ID is required"))
		return
	}

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	stats, err := client.GetSystemStats(r.Context())
	if err != nil {
		h.logger.Error("failed to get panel stats", slog.Any("error", err), slog.String("panel_id", panelID))
		writeJSON(w, http.StatusOK, map[string]any{
			"panel_id": panelID,
			"error":    err.Error(),
		})
		return
	}

	health, _ := client.GetSystemHealth(r.Context())

	result := map[string]any{
		"panel_id": panelID,
		"stats":    stats,
	}
	if health != nil {
		result["health"] = health
	}

	writeJSON(w, http.StatusOK, result)
}

// migrateFromLegacyConfig creates a default panel from legacy plugin config if needed.
func (h *Handler) migrateFromLegacyConfig(ctx context.Context) {
	docs, _ := h.collections.ListDocuments(ctx, PluginSlug, CollectionPanelConnections)
	if len(docs) > 0 {
		return // already have panels
	}

	p, err := h.pluginRepo.GetBySlug(ctx, PluginSlug)
	if err != nil {
		return
	}

	url := p.Config["url"]
	token := p.Config["api_token"]
	if url == "" || token == "" {
		return
	}

	input := PanelConnectionInput{
		Slug:     "default",
		URL:      url,
		APIToken: token,
		Status:   PanelStatusActive,
	}

	data, _ := json.Marshal(input)
	if _, err := h.collections.InsertDocument(ctx, PluginSlug, CollectionPanelConnections, json.RawMessage(data)); err != nil {
		h.logger.Warn("failed to migrate legacy config to panel_connections", slog.Any("error", err))
		return
	}

	h.logger.Info("migrated legacy remnawave config to panel_connections")
}

