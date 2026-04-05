package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// MultiSubHandler exposes read-only HTTP endpoints for Remnawave bindings.
// Provisioning happens automatically when billing events fire.
type MultiSubHandler struct {
	bindingRepo multisub.BindingRepository
}

// NewMultiSubHandler creates a MultiSubHandler backed by the binding
// repository.
func NewMultiSubHandler(bindingRepo multisub.BindingRepository) *MultiSubHandler {
	return &MultiSubHandler{bindingRepo: bindingRepo}
}

// GetMyBindings handles GET /api/bindings -- list user's Remnawave bindings
// with shortUUIDs.
func (h *MultiSubHandler) GetMyBindings(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	bindings, err := h.bindingRepo.GetByPlatformUserID(r.Context(), claims.UserID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, bindings)
}

// GetBindingsBySubscription handles GET /api/subscriptions/{subID}/bindings --
// list bindings for a specific subscription.
func (h *MultiSubHandler) GetBindingsBySubscription(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	subID := chi.URLParam(r, "subID")
	if subID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("subscription ID is required"))
		return
	}

	bindings, err := h.bindingRepo.GetBySubscriptionID(r.Context(), subID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	// Verify at least one binding belongs to the authenticated user (or the
	// subscription is empty). This prevents users from inspecting other users'
	// bindings by guessing subscription IDs.
	for _, b := range bindings {
		if b.PlatformUserID != claims.UserID {
			writeAPIError(w, apierror.Forbidden.WithDetails("subscription does not belong to you"))
			return
		}
		break
	}

	writeJSON(w, http.StatusOK, bindings)
}
