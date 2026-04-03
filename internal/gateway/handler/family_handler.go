package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	billingservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
)

// FamilyHandler exposes HTTP endpoints for family group management.
type FamilyHandler struct {
	service  *billingservice.BillingService
	families billing.FamilyRepository
}

// NewFamilyHandler creates a FamilyHandler backed by the billing service and
// family repository.
func NewFamilyHandler(
	service *billingservice.BillingService,
	families billing.FamilyRepository,
) *FamilyHandler {
	return &FamilyHandler{
		service:  service,
		families: families,
	}
}

// --- Request DTOs ---

type createFamilyRequest struct {
	SubscriptionID string `json:"subscription_id"`
}

type addFamilyMemberRequest struct {
	SubscriptionID string `json:"subscription_id"`
	MemberUserID   string `json:"member_user_id"`
	Nickname       string `json:"nickname"`
}

// --- Handlers ---

// CreateFamily handles POST /api/family -- create a family group.
func (h *FamilyHandler) CreateFamily(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req createFamilyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.SubscriptionID == "" {
		writeError(w, http.StatusBadRequest, "subscription_id is required")
		return
	}

	// AddFamilyMember with the owner as member triggers group creation if not
	// yet created. Alternatively, we can just return the existing group.
	fg, err := h.families.GetByOwnerID(r.Context(), claims.UserID)
	if err != nil {
		// No group yet — caller should use AddFamilyMember which auto-creates.
		writeError(w, http.StatusNotFound, "family group not found, add a member to create one")
		return
	}

	writeJSON(w, http.StatusOK, fg)
}

// GetMyFamily handles GET /api/family -- get the current user's family group.
func (h *FamilyHandler) GetMyFamily(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	fg, err := h.families.GetByOwnerID(r.Context(), claims.UserID)
	if err != nil {
		status, message := mapServiceError(err)
		writeError(w, status, message)
		return
	}

	writeJSON(w, http.StatusOK, fg)
}

// AddMember handles POST /api/family/members -- add a member to the family.
func (h *FamilyHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req addFamilyMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.SubscriptionID == "" || req.MemberUserID == "" {
		writeError(w, http.StatusBadRequest, "subscription_id and member_user_id are required")
		return
	}

	if err := h.service.AddFamilyMember(r.Context(), req.SubscriptionID, req.MemberUserID, req.Nickname); err != nil {
		status, message := mapServiceError(err)
		writeError(w, status, message)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "member_added"})
}

// RemoveMember handles DELETE /api/family/members/{userID} -- remove a member.
func (h *FamilyHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	memberUserID := chi.URLParam(r, "userID")
	if memberUserID == "" {
		writeError(w, http.StatusBadRequest, "user ID is required")
		return
	}

	// Get the subscription ID from query param for authorization check.
	subID := r.URL.Query().Get("subscription_id")
	if subID == "" {
		writeError(w, http.StatusBadRequest, "subscription_id query parameter is required")
		return
	}

	if err := h.service.RemoveFamilyMember(r.Context(), subID, memberUserID); err != nil {
		status, message := mapServiceError(err)
		writeError(w, status, message)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "member_removed"})
}
