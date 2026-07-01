package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	identityservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
)

// IAMHandler exposes HTTP endpoints for invitation management, user creation,
// role assignment, and shop provisioning.
type IAMHandler struct {
	svc *identityservice.IdentityAdminService
}

// NewIAMHandler creates an IAMHandler backed by the given IdentityAdminService.
func NewIAMHandler(svc *identityservice.IdentityAdminService) *IAMHandler {
	return &IAMHandler{svc: svc}
}

// --- Request DTOs ---

type createInvitationRequest struct {
	Email          string  `json:"email"`
	RoleKey        string  `json:"role_key"`
	TenantID       *string `json:"tenant_id"`
	CommissionRate *int    `json:"commission_rate"`
}

type acceptInvitationRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type createUserDirectRequest struct {
	Email    string  `json:"email"`
	Password string  `json:"password"`
	RoleKey  string  `json:"role_key"`
	TenantID *string `json:"tenant_id"`
}

type roleBindingRequest struct {
	RoleKey  string  `json:"role_key"`
	TenantID *string `json:"tenant_id"`
}

type createShopRequest struct {
	Name           string  `json:"name"`
	Domain         string  `json:"domain"`
	OwnerUserID    *string `json:"owner_user_id"`
	InviteEmail    *string `json:"invite_email"`
	CommissionRate int     `json:"commission_rate"`
}

// --- Handlers ---

// CreateInvitation handles POST /api/users/invitations.
func (h *IAMHandler) CreateInvitation(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	var req createInvitationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if req.Email == "" || req.RoleKey == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("email and role_key are required"))
		return
	}

	// Resolve effective tenant: body tenant_id takes precedence; fall back to
	// the active tenant from ShopResolver; nil means global scope.
	tenantID := req.TenantID
	if tenantID == nil || *tenantID == "" {
		if active := middleware.ActiveTenantID(r.Context()); active != "" {
			tenantID = &active
		} else {
			tenantID = nil
		}
	}

	inv, err := h.svc.InviteUser(r.Context(), claims.UserID, identityservice.InviteInput{
		Email:          req.Email,
		RoleKey:        req.RoleKey,
		TenantID:       tenantID,
		CommissionRate: req.CommissionRate,
	})
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"invitation_id": inv.ID,
		"email":         inv.Email,
		"role_key":      inv.RoleKey,
		"tenant_id":     inv.TenantID,
		"token":         inv.Token,
		"accept_path":   "/api/auth/accept-invitation",
	})
}

// ListInvitations handles GET /api/users/invitations.
func (h *IAMHandler) ListInvitations(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	invitations, err := h.svc.ListInvitations(r.Context(), claims.UserID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	type invitationItem struct {
		ID        string  `json:"id"`
		Email     string  `json:"email"`
		RoleKey   string  `json:"role_key"`
		TenantID  *string `json:"tenant_id"`
		ExpiresAt string  `json:"expires_at"`
	}

	items := make([]invitationItem, 0, len(invitations))
	for _, inv := range invitations {
		items = append(items, invitationItem{
			ID:        inv.ID,
			Email:     inv.Email,
			RoleKey:   inv.RoleKey,
			TenantID:  inv.TenantID,
			ExpiresAt: inv.ExpiresAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"invitations": items})
}

// RevokeInvitation handles DELETE /api/users/invitations/{id}.
func (h *IAMHandler) RevokeInvitation(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	invitationID := chi.URLParam(r, "id")
	if invitationID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invitation id is required"))
		return
	}

	if err := h.svc.RevokeInvitation(r.Context(), claims.UserID, invitationID); err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AcceptInvitation handles POST /api/auth/accept-invitation (public).
func (h *IAMHandler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	var req acceptInvitationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if req.Token == "" || req.Password == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("token and password are required"))
		return
	}

	result, err := h.svc.AcceptInvitation(r.Context(), req.Token, req.Password, extractIP(r), r.UserAgent())
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefreshToken,
		"user":          userToResponse(result.User),
	})
}

// CreateUserDirect handles POST /api/users.
func (h *IAMHandler) CreateUserDirect(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	var req createUserDirectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if req.Email == "" || req.Password == "" || req.RoleKey == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("email, password, and role_key are required"))
		return
	}

	tenantID := strPtrOrNil(req.TenantID)

	user, err := h.svc.CreateUserDirect(r.Context(), claims.UserID, identityservice.CreateUserInput{
		Email:    req.Email,
		Password: req.Password,
		RoleKey:  req.RoleKey,
		TenantID: tenantID,
	})
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"user_id": user.ID,
		"email":   user.Email,
	})
}

// AssignRole handles POST /api/users/{userID}/roles.
func (h *IAMHandler) AssignRole(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	targetUserID := chi.URLParam(r, "userID")
	if targetUserID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("userID is required"))
		return
	}

	var req roleBindingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if req.RoleKey == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("role_key is required"))
		return
	}

	tenantID := strPtrOrNil(req.TenantID)

	if err := h.svc.AssignRole(r.Context(), claims.UserID, targetUserID, req.RoleKey, tenantID); err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RevokeRole handles DELETE /api/users/{userID}/roles.
func (h *IAMHandler) RevokeRole(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	targetUserID := chi.URLParam(r, "userID")
	if targetUserID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("userID is required"))
		return
	}

	var req roleBindingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if req.RoleKey == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("role_key is required"))
		return
	}

	tenantID := strPtrOrNil(req.TenantID)

	if err := h.svc.RevokeRole(r.Context(), claims.UserID, targetUserID, req.RoleKey, tenantID); err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListUserRoles handles GET /api/users/{userID}/roles.
func (h *IAMHandler) ListUserRoles(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	targetUserID := chi.URLParam(r, "userID")
	if targetUserID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("userID is required"))
		return
	}

	bindings, err := h.svc.ListUserRoles(r.Context(), claims.UserID, targetUserID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	type roleItem struct {
		RoleID    string  `json:"role_id"`
		RoleKey   string  `json:"role_key"`
		ScopeKind string  `json:"scope_kind"`
		TenantID  *string `json:"tenant_id"`
	}

	items := make([]roleItem, 0, len(bindings))
	for _, b := range bindings {
		items = append(items, roleItem{
			RoleID:    b.RoleID,
			RoleKey:   b.RoleKey,
			ScopeKind: b.ScopeKind,
			TenantID:  b.TenantID,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"roles": items})
}

// CreateShop handles POST /api/admin/shops.
func (h *IAMHandler) CreateShop(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	var req createShopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if req.Name == "" || req.Domain == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("name and domain are required"))
		return
	}

	ownerUserID := pgutil.DerefStr(req.OwnerUserID)
	inviteEmail := pgutil.DerefStr(req.InviteEmail)

	result, err := h.svc.CreateShop(r.Context(), claims.UserID, identityservice.CreateShopInput{
		Name:   req.Name,
		Domain: req.Domain,
		Owner: identityservice.OwnerSpec{
			ExistingUserID: ownerUserID,
			InviteEmail:    inviteEmail,
		},
		CommissionRate: req.CommissionRate,
	})
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	resp := map[string]any{
		"tenant_id": result.TenantID,
		"api_key":   result.APIKey,
	}
	if result.OwnerUserID != nil {
		resp["owner_user_id"] = *result.OwnerUserID
	}
	if result.InvitationToken != "" {
		resp["invitation_token"] = result.InvitationToken
	}

	writeJSON(w, http.StatusCreated, resp)
}

// --- Helpers ---

// strPtrOrNil returns nil if p is nil or points to an empty string, otherwise
// returns p unchanged. This normalises optional tenant_id JSON fields.
func strPtrOrNil(p *string) *string {
	if p == nil || *p == "" {
		return nil
	}
	return p
}
