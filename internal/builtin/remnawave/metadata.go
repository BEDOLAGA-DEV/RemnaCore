package remnawave

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// GetUserMetadata returns metadata for a user.
func (h *Handler) GetUserMetadata(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	userUUID := chi.URLParam(r, "userUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	meta, err := client.GetUserMetadata(r.Context(), userUUID)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "user_uuid": userUUID, "metadata": meta})
}

// UpsertUserMetadata sets metadata for a user.
func (h *Handler) UpsertUserMetadata(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	userUUID := chi.URLParam(r, "userUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var meta json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&meta); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid JSON body"))
		return
	}

	if err := client.UpsertUserMetadata(r.Context(), userUUID, meta); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "metadata_updated"})
}

// GetNodeMetadata returns metadata for a node.
func (h *Handler) GetNodeMetadata(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	nodeUUID := chi.URLParam(r, "nodeUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	meta, err := client.GetNodeMetadata(r.Context(), nodeUUID)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "node_uuid": nodeUUID, "metadata": meta})
}

// UpsertNodeMetadata sets metadata for a node.
func (h *Handler) UpsertNodeMetadata(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	nodeUUID := chi.URLParam(r, "nodeUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var meta json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&meta); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid JSON body"))
		return
	}

	if err := client.UpsertNodeMetadata(r.Context(), nodeUUID, meta); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "metadata_updated"})
}
