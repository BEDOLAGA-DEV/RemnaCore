package remnawave

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	rwclient "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// ListConfigProfiles returns all config profiles from a panel.
func (h *Handler) ListConfigProfiles(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	profiles, err := client.GetConfigProfiles(r.Context())
	if err != nil {
		h.logger.Error("failed to get config profiles", slog.Any("error", err))
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "profiles": profiles})
}

// GetConfigProfile returns a single config profile.
func (h *Handler) GetConfigProfile(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	profileUUID := chi.URLParam(r, "profileUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	profile, err := client.GetConfigProfileByUUID(r.Context(), profileUUID)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "profile": profile})
}

// CreateConfigProfile creates a new config profile.
func (h *Handler) CreateConfigProfile(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req rwclient.CreateConfigProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	profile, err := client.CreateConfigProfile(r.Context(), req)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"panel_id": panelID, "profile": profile})
}

// UpdateConfigProfile updates an existing config profile.
func (h *Handler) UpdateConfigProfile(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req rwclient.UpdateConfigProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	profile, err := client.UpdateConfigProfile(r.Context(), req)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "profile": profile})
}

// DeleteConfigProfile deletes a config profile.
func (h *Handler) DeleteConfigProfile(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	profileUUID := chi.URLParam(r, "profileUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	if err := client.DeleteConfigProfile(r.Context(), profileUUID); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetConfigProfileInbounds returns inbounds for a config profile.
func (h *Handler) GetConfigProfileInbounds(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	profileUUID := chi.URLParam(r, "profileUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	inbounds, err := client.GetConfigProfileInbounds(r.Context(), profileUUID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"inbounds": []any{}})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "inbounds": inbounds})
}

// GetAllInbounds returns all inbounds from a panel.
func (h *Handler) GetAllInbounds(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	inbounds, err := client.GetAllInbounds(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"inbounds": []any{}})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "inbounds": inbounds})
}

// ReorderConfigProfiles reorders config profiles on a panel.
func (h *Handler) ReorderConfigProfiles(w http.ResponseWriter, r *http.Request) {
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

	if err := client.ReorderConfigProfiles(r.Context(), req.UUIDs); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "reordered"})
}
