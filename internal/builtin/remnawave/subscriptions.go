package remnawave

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// ListSubscriptions returns all subscriptions from a panel.
func (h *Handler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	subs, err := client.GetAllSubscriptions(r.Context())
	if err != nil {
		h.logger.Error("failed to get subscriptions", slog.Any("error", err))
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "subscriptions": subs})
}

// GetSubscriptionByUUID returns a subscription by its owner's user id.
// Remnawave 3 removed lookup by subscription UUID; the route parameter keeps
// its name so existing links stay valid.
func (h *Handler) GetSubscriptionByUUID(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	userID, convErr := strconv.ParseInt(chi.URLParam(r, "subUUID"), 10, 64)
	if convErr != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("subscription lookup expects a numeric user id"))
		return
	}

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	sub, err := client.GetSubscriptionByUserID(r.Context(), userID)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "subscription": sub})
}

// GetSubscriptionByShortUUID returns a subscription by short UUID.
func (h *Handler) GetSubscriptionByShortUUID(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	shortUUID := chi.URLParam(r, "shortUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	sub, err := client.GetSubscriptionByShortUUID(r.Context(), shortUUID)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "subscription": sub})
}

// GetSubscriptionByUsername returns a subscription by username.
func (h *Handler) GetSubscriptionByUsername(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	username := chi.URLParam(r, "username")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	sub, err := client.GetSubscriptionByUsername(r.Context(), username)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"panel_id": panelID, "subscription": sub})
}

// ListSubscriptionPageConfigs returns subscription page configs from all panels.
func (h *Handler) ListSubscriptionPageConfigs(w http.ResponseWriter, r *http.Request) {
	clients, err := h.getAllPanelClients(r.Context())
	if err != nil || len(clients) == 0 {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	type configWithPanel struct {
		PanelID string `json:"panel_id"`
		Config  any    `json:"config"`
	}

	all := make([]configWithPanel, 0)
	for panelID, client := range clients {
		configs, err := client.GetSubscriptionPageConfigs(r.Context())
		if err != nil {
			continue
		}
		for _, c := range configs {
			all = append(all, configWithPanel{PanelID: panelID, Config: c})
		}
	}

	writeJSON(w, http.StatusOK, all)
}
