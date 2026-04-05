package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	billingservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// BillingHandler exposes HTTP endpoints for plans, subscriptions, and invoices.
type BillingHandler struct {
	service  *billingservice.BillingService
	plans    billing.PlanRepository
	subs     billing.SubscriptionRepository
	invoices billing.InvoiceRepository
}

// NewBillingHandler creates a BillingHandler backed by the billing service and
// read-only repository access for query endpoints.
func NewBillingHandler(
	service *billingservice.BillingService,
	plans billing.PlanRepository,
	subs billing.SubscriptionRepository,
	invoices billing.InvoiceRepository,
) *BillingHandler {
	return &BillingHandler{
		service:  service,
		plans:    plans,
		subs:     subs,
		invoices: invoices,
	}
}

// --- Request DTOs ---

type createSubscriptionRequest struct {
	PlanID   string   `json:"plan_id"`
	AddonIDs []string `json:"addon_ids"`
}

// --- Handlers ---

// GetPlans handles GET /api/plans -- list all active plans.
func (h *BillingHandler) GetPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := h.plans.GetActive(r.Context())
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, plans)
}

// GetPlan handles GET /api/plans/{planID} -- get a single plan by ID.
func (h *BillingHandler) GetPlan(w http.ResponseWriter, r *http.Request) {
	planID := chi.URLParam(r, "planID")
	if planID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("plan ID is required"))
		return
	}

	plan, err := h.plans.GetByID(r.Context(), planID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, plan)
}

// CreateSubscription handles POST /api/subscriptions -- create a new subscription.
func (h *BillingHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	var req createSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if req.PlanID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("plan_id is required"))
		return
	}

	sub, inv, err := h.service.CreateSubscription(r.Context(), billingservice.CreateSubscriptionCmd{
		UserID:   claims.UserID,
		PlanID:   req.PlanID,
		AddonIDs: req.AddonIDs,
	})
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"subscription": sub,
		"invoice":      inv,
	})
}

// GetMySubscriptions handles GET /api/subscriptions -- list user's subscriptions.
func (h *BillingHandler) GetMySubscriptions(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	subs, err := h.subs.GetByUserID(r.Context(), claims.UserID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, subs)
}

// CancelSubscription handles POST /api/subscriptions/{subID}/cancel.
func (h *BillingHandler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
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

	// Verify the subscription belongs to the authenticated user.
	sub, err := h.subs.GetByID(r.Context(), subID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}
	if sub.UserID != claims.UserID {
		writeAPIError(w, apierror.Forbidden.WithDetails("subscription does not belong to you"))
		return
	}

	if err := h.service.CancelSubscription(r.Context(), subID); err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// GetInvoices handles GET /api/invoices -- list user's pending invoices.
func (h *BillingHandler) GetInvoices(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	invoices, err := h.invoices.GetPendingByUserID(r.Context(), claims.UserID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, invoices)
}

// GetSubscription handles GET /api/subscriptions/{subID} -- full subscription detail with bindings.
func (h *BillingHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
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

	sub, err := h.subs.GetByID(r.Context(), subID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}
	if sub.UserID != claims.UserID {
		writeAPIError(w, apierror.Forbidden.WithDetails("subscription does not belong to you"))
		return
	}

	writeJSON(w, http.StatusOK, sub)
}

// AddSubscriptionAddon handles POST /api/subscriptions/{subID}/addons -- add an addon.
func (h *BillingHandler) AddSubscriptionAddon(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		AddonID string `json:"addon_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}
	if req.AddonID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("addon_id is required"))
		return
	}

	// Verify ownership.
	sub, err := h.subs.GetByID(r.Context(), subID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}
	if sub.UserID != claims.UserID {
		writeAPIError(w, apierror.Forbidden.WithDetails("subscription does not belong to you"))
		return
	}

	if err := h.service.AddSubscriptionAddon(r.Context(), subID, req.AddonID); err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "addon_added"})
}

// RemoveSubscriptionAddon handles DELETE /api/subscriptions/{subID}/addons/{addonID} -- remove an addon.
func (h *BillingHandler) RemoveSubscriptionAddon(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	subID := chi.URLParam(r, "subID")
	addonID := chi.URLParam(r, "addonID")
	if subID == "" || addonID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("subscription ID and addon ID are required"))
		return
	}

	// Verify ownership.
	sub, err := h.subs.GetByID(r.Context(), subID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}
	if sub.UserID != claims.UserID {
		writeAPIError(w, apierror.Forbidden.WithDetails("subscription does not belong to you"))
		return
	}

	if err := h.service.RemoveSubscriptionAddon(r.Context(), subID, addonID); err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "addon_removed"})
}

// PayInvoice handles POST /api/invoices/{invoiceID}/pay.
// This is a placeholder; actual payment processing will be added in Phase 4
// via a payment plugin.
func (h *BillingHandler) PayInvoice(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	invoiceID := chi.URLParam(r, "invoiceID")
	if invoiceID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invoice ID is required"))
		return
	}

	// Verify the invoice belongs to the authenticated user.
	inv, err := h.invoices.GetByID(r.Context(), invoiceID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}
	if inv.UserID != claims.UserID {
		writeAPIError(w, apierror.Forbidden.WithDetails("invoice does not belong to you"))
		return
	}

	if err := h.service.PayInvoice(r.Context(), invoiceID); err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "paid"})
}
