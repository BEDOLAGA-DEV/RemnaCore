package remnawave

import (
	"log/slog"
	"net/http"

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

// GetSubscriptionByUUID returns a subscription by UUID.
func (h *Handler) GetSubscriptionByUUID(w http.ResponseWriter, r *http.Request) {
	panelID := chi.URLParam(r, "panelID")
	subUUID := chi.URLParam(r, "subUUID")

	client, err := h.buildClientForPanel(r.Context(), panelID)
	if err != nil {
		writeAPIError(w, apierror.NotFound)
		return
	}

	sub, err := client.GetSubscriptionByUUID(r.Context(), subUUID)
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
