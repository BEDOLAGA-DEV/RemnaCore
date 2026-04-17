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

const maxBulkUUIDs = 1000

func validateUUIDList(uuids []string, w http.ResponseWriter) bool {
	if len(uuids) == 0 {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("uuids is required"))
		return false
	}
	if len(uuids) > maxBulkUUIDs {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(fmt.Sprintf("maximum %d UUIDs per request", maxBulkUUIDs)))
		return false
	}
	return true
}

// BulkDeleteUsersByStatus deletes all users with a given status.
func (h *Handler) BulkDeleteUsersByStatus(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	allowedStatuses := map[string]bool{"disabled": true, "expired": true, "limited": true}
	if !allowedStatuses[req.Status] {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("status must be disabled, expired, or limited"))
		return
	}

	if err := client.BulkDeleteUsersByStatus(r.Context(), req.Status); err != nil {
		h.logger.Error("bulk delete by status failed", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "deleted_by_status", "status": req.Status})
}

// BulkUpdateUsers updates multiple users at once.
func (h *Handler) BulkUpdateUsers(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req rwclient.BulkUpdateUsersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := client.BulkUpdateUsers(r.Context(), req); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "bulk_updated"})
}

// BulkResetTraffic resets traffic for multiple users.
func (h *Handler) BulkResetTraffic(w http.ResponseWriter, r *http.Request) {
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

	if !validateUUIDList(req.UUIDs, w) {
		return
	}

	if err := client.BulkResetTraffic(r.Context(), req.UUIDs); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "traffic_reset", "count": len(req.UUIDs)})
}

// BulkRevokeSubscription revokes subscriptions for multiple users.
func (h *Handler) BulkRevokeSubscription(w http.ResponseWriter, r *http.Request) {
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

	if !validateUUIDList(req.UUIDs, w) {
		return
	}

	if err := client.BulkRevokeSubscription(r.Context(), req.UUIDs); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "subscriptions_revoked", "count": len(req.UUIDs)})
}

// BulkDeleteUsers deletes multiple users by UUIDs.
func (h *Handler) BulkDeleteUsers(w http.ResponseWriter, r *http.Request) {
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

	if !validateUUIDList(req.UUIDs, w) {
		return
	}

	if err := client.BulkDeleteUsers(r.Context(), req.UUIDs); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "users_deleted", "count": len(req.UUIDs)})
}

// BulkUpdateSquads updates squad assignments for multiple users.
func (h *Handler) BulkUpdateSquads(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req rwclient.BulkUpdateSquadsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := client.BulkUpdateSquads(r.Context(), req); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "squads_updated"})
}

// BulkExtendExpiration extends expiration for multiple users.
func (h *Handler) BulkExtendExpiration(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req rwclient.BulkExtendExpirationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := client.BulkExtendExpiration(r.Context(), req); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "expiration_extended"})
}

// BulkUpdateAllUsers updates all users on a panel.
func (h *Handler) BulkUpdateAllUsers(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req rwclient.BulkUpdateAllRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := client.BulkUpdateAllUsers(r.Context(), req); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "all_users_updated"})
}

// BulkResetAllTraffic resets traffic for all users on a panel.
func (h *Handler) BulkResetAllTraffic(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	if err := client.BulkResetAllTraffic(r.Context()); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "all_traffic_reset"})
}

// BulkExtendAllExpiration extends expiration for all users on a panel.
func (h *Handler) BulkExtendAllExpiration(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req rwclient.BulkExtendExpirationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := client.BulkExtendAllExpiration(r.Context(), req); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "all_expiration_extended"})
}
