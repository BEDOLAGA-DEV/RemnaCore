package handler

import (
	"encoding/json"
	"net/http"

	billingservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// CheckoutHandler exposes HTTP endpoints for the checkout flow.
type CheckoutHandler struct {
	checkout *billingservice.CheckoutService
}

// NewCheckoutHandler creates a CheckoutHandler backed by the checkout service.
func NewCheckoutHandler(checkout *billingservice.CheckoutService) *CheckoutHandler {
	return &CheckoutHandler{checkout: checkout}
}

// --- Request DTOs ---

type startCheckoutRequest struct {
	PlanID    string   `json:"plan_id"`
	AddonIDs  []string `json:"addon_ids"`
	ReturnURL string   `json:"return_url"`
	CancelURL string   `json:"cancel_url"`
}

// --- Handlers ---

// StartCheckout handles POST /api/checkout — starts the checkout flow.
func (h *CheckoutHandler) StartCheckout(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	var req startCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if req.PlanID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("plan_id is required"))
		return
	}

	result, err := h.checkout.StartCheckout(r.Context(), billingservice.CheckoutRequest{
		UserID:    claims.UserID,
		UserEmail: claims.Email,
		PlanID:    req.PlanID,
		AddonIDs:  req.AddonIDs,
		ReturnURL: req.ReturnURL,
		CancelURL: req.CancelURL,
	})
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}
