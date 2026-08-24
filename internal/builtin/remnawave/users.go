package remnawave

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	rwclient "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// SearchUsers searches users across all panels by query params.
func (h *Handler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	clients, err := h.getAllPanelClients(r.Context())
	if err != nil || len(clients) == 0 {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	query := r.URL.Query().Get("q")
	type userWithPanel struct {
		PanelID string `json:"panel_id"`
		User    any    `json:"user"`
	}

	allUsers := make([]userWithPanel, 0)
	for panelID, client := range clients {
		resp, err := client.GetAllUsers(r.Context(), rwclient.GetAllUsersParams{Size: 200})
		if err != nil {
			h.logger.Warn("failed to get users from panel", slog.String("panel_id", panelID), slog.Any("error", err))
			continue
		}
		for _, u := range resp.Users {
			// Client-side filter by query if provided
			if query != "" {
				match := strings.Contains(strings.ToLower(u.Username), strings.ToLower(query)) ||
					strings.Contains(strings.ToLower(u.ShortUUID), strings.ToLower(query))
				if !match {
					continue
				}
			}
			allUsers = append(allUsers, userWithPanel{PanelID: panelID, User: u})
		}
	}

	writeJSON(w, http.StatusOK, allUsers)
}

// GetUser returns details of a specific user from a panel.
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	userUUID := chi.URLParam(r, "userUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	user, err := client.GetUserByUUID(r.Context(), userUUID)
	if err != nil {
		h.logger.Error("failed to get user", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	// Get HWID devices
	hwid, _ := client.GetUserHWIDDevices(r.Context(), userUUID)

	writeJSON(w, http.StatusOK, map[string]any{
		"panel_id": panelID,
		"user":     user,
		"hwid":     hwid,
	})
}

// routeUserID reads the numeric Remnawave user id from the request path.
// Remnawave 3 addresses users by id; the route parameter keeps its historical
// name so existing links and the admin UI stay valid. It writes the validation
// error itself and reports whether the caller may continue.
func routeUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "userUUID"), 10, 64)
	if err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("user id must be numeric"))
		return 0, false
	}
	return id, true
}

// UpdateUser updates a user's settings on a panel.
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	userID, ok := routeUserID(w, r)
	if !ok {
		return
	}

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	var req rwclient.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}
	req.ID = userID

	user, err := client.UpdateUser(r.Context(), req)
	if err != nil {
		h.logger.Error("failed to update user", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"panel_id": panelID,
		"user":     user,
	})
}

// ResetUserTraffic resets traffic counters for a user.
func (h *Handler) ResetUserTraffic(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	userUUID := chi.URLParam(r, "userUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	if err := client.ResetUserTraffic(r.Context(), userUUID); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "traffic_reset"})
}

// RevokeUserSubscription revokes a user's subscription (generates new sub URL).
func (h *Handler) RevokeUserSubscription(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	userUUID := chi.URLParam(r, "userUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	if err := client.RevokeUser(r.Context(), userUUID); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "subscription_revoked"})
}

// DisconnectUser drops all active connections for a user.
func (h *Handler) DisconnectUser(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	userID, ok := routeUserID(w, r)
	if !ok {
		return
	}

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	if err := client.DropConnections(r.Context(), rwclient.NewDropUserConnections(userID)); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "disconnected"})
}

// GetUserSessions returns live sessions for a user (via IP fetch).
func (h *Handler) GetUserSessions(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	userID, ok := routeUserID(w, r)
	if !ok {
		return
	}

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	job, err := client.FetchUserIPs(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"sessions": []any{}, "error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"panel_id": panelID,
		"user_id":  userID,
		"job_id":   job.JobID,
	})
}

// GetUserHWID returns HWID devices for a user.
func (h *Handler) GetUserHWID(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	userUUID := chi.URLParam(r, "userUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	devices, err := client.GetUserHWIDDevices(r.Context(), userUUID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"devices": []any{}})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"panel_id":  panelID,
		"user_uuid": userUUID,
		"devices":   devices,
	})
}

// DeleteUserHWID removes a specific HWID device from a user.
func (h *Handler) DeleteUserHWID(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	userUUID := chi.URLParam(r, "userUUID")
	hwidID := chi.URLParam(r, "hwidID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	if err := client.DeleteHWIDDevice(r.Context(), rwclient.DeleteHWIDDeviceRequest{
		Hwid:     hwidID,
		UserUUID: userUUID,
	}); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetUserSubscriptionURL returns the subscription URL for a user.
func (h *Handler) GetUserSubscriptionURL(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	userUUID := chi.URLParam(r, "userUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	user, err := client.GetUserByUUID(r.Context(), userUUID)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	// Get subscription page URL from plugin config
	p, _ := h.pluginRepo.GetBySlug(r.Context(), PluginSlug)
	basePageURL := ""
	preferredClient := "auto"
	if p != nil {
		basePageURL = p.Config["subscription_page_url"]
		if c := p.Config["subscription_preferred_client"]; c != "" {
			preferredClient = c
		}
	}

	subURL := BuildSubscriptionURL(user.SubscriptionURL, basePageURL, preferredClient)
	writeJSON(w, http.StatusOK, subURL)
}

// EnableUser enables a user on a panel.
func (h *Handler) EnableUser(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	userUUID := chi.URLParam(r, "userUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	if err := client.EnableUser(r.Context(), userUUID); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "enabled"})
}

// DisableUser disables a user on a panel.
func (h *Handler) DisableUser(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	userUUID := chi.URLParam(r, "userUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	if err := client.DisableUser(r.Context(), userUUID); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "disabled"})
}

// DeleteUser deletes a user from a panel.
func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	userUUID := chi.URLParam(r, "userUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	if err := client.DeleteUser(r.Context(), userUUID); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetUserTags returns all user tags from a panel.
func (h *Handler) GetUserTags(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	tags, err := client.GetUserTags(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"tags": []string{}})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "tags": tags})
}

// GetUserBandwidth returns bandwidth stats for a specific user.
func (h *Handler) GetUserBandwidth(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	userUUID := chi.URLParam(r, "userUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	stats, err := client.GetUserBandwidthStats(r.Context(), userUUID)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "bandwidth": stats})
}

// GetIPFetchResult polls for the result of an async IP fetch job.
func (h *Handler) GetIPFetchResult(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	jobID := chi.URLParam(r, "jobID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	result, err := client.GetFetchIPsResult(r.Context(), jobID)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "result": result})
}

// ListAllHWIDDevices returns all HWID devices across a panel.
func (h *Handler) ListAllHWIDDevices(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}
	devices, err := client.GetAllHWIDDevices(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"devices": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "devices": devices})
}

// CreateHWIDDevice registers a new HWID device for a user.
func (h *Handler) CreateHWIDDevice(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}
	var req rwclient.CreateHWIDDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}
	// 2.8.0 returns the user's full {total, devices[]} list, not a single device.
	result, err := client.CreateHWIDDevice(r.Context(), req)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"panel_id": panelID, "devices": result})
}

// DeleteAllUserHWID wipes all HWID devices for a user.
func (h *Handler) DeleteAllUserHWID(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	userUUID := chi.URLParam(r, "userUUID")
	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}
	if err := client.DeleteAllHWIDDevices(r.Context(), userUUID); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"action": "all_hwid_deleted"})
}
