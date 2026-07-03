package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	identityservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

type createCustomRoleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	ScopeKind   string   `json:"scope_kind"`
	TenantID    *string  `json:"tenant_id"`
	Permissions []string `json:"permissions"`
}

type assignCustomRoleRequest struct {
	RoleID   string  `json:"role_id"`
	TenantID *string `json:"tenant_id"`
}

// customRoleView is the API projection of a custom role.
type customRoleView struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	ScopeKind   string   `json:"scope_kind"`
	TenantID    *string  `json:"tenant_id"`
	Permissions []string `json:"permissions"`
}

func toCustomRoleView(r rbac.CustomRole) customRoleView {
	perms := make([]string, len(r.Permissions))
	for i, p := range r.Permissions {
		perms[i] = string(p)
	}
	return customRoleView{
		ID: r.ID, Name: r.Name, Description: r.Description,
		ScopeKind: r.ScopeKind, TenantID: r.TenantID, Permissions: perms,
	}
}

// permissionView is the API projection of a permission catalog entry.
type permissionView struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Scope       string `json:"scope"`
}

// ListPermissions handles GET /api/permissions — the permission catalog the
// custom-role UI offers when building a role's permission set.
func (h *IAMHandler) ListPermissions(w http.ResponseWriter, _ *http.Request) {
	defs := rbac.Catalog()
	views := make([]permissionView, len(defs))
	for i, d := range defs {
		views[i] = permissionView{Key: string(d.Key), Description: d.Description, Scope: string(d.Scope)}
	}
	writeJSON(w, http.StatusOK, map[string]any{"permissions": views})
}

// CreateCustomRole handles POST /api/roles.
func (h *IAMHandler) CreateCustomRole(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	var req createCustomRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}
	if req.Name == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("name is required"))
		return
	}
	if req.ScopeKind != rbac.ScopeGlobal && req.ScopeKind != rbac.ScopeShop {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("scope_kind must be 'global' or 'shop'"))
		return
	}

	perms := make([]rbac.Permission, len(req.Permissions))
	for i, p := range req.Permissions {
		perms[i] = rbac.Permission(p)
	}

	roleID, err := h.svc.CreateCustomRole(r.Context(), claims.UserID, identityservice.CreateCustomRoleInput{
		Name:        req.Name,
		Description: req.Description,
		ScopeKind:   req.ScopeKind,
		TenantID:    strPtrOrNil(req.TenantID),
		Permissions: perms,
	})
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"role_id": roleID})
}

// ListCustomRoles handles GET /api/roles?tenant_id=.
func (h *IAMHandler) ListCustomRoles(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	var tenantID *string
	if q := r.URL.Query().Get("tenant_id"); q != "" {
		tenantID = &q
	}

	roles, err := h.svc.ListCustomRoles(r.Context(), claims.UserID, tenantID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}
	views := make([]customRoleView, len(roles))
	for i, cr := range roles {
		views[i] = toCustomRoleView(cr)
	}
	writeJSON(w, http.StatusOK, map[string]any{"roles": views})
}

// DeleteCustomRole handles DELETE /api/roles/{roleID}.
func (h *IAMHandler) DeleteCustomRole(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}
	roleID := chi.URLParam(r, "roleID")
	if roleID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("roleID is required"))
		return
	}
	if err := h.svc.DeleteCustomRole(r.Context(), claims.UserID, roleID); err != nil {
		writeErrorFromDomain(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AssignCustomRole handles POST /api/users/{userID}/roles/custom.
func (h *IAMHandler) AssignCustomRole(w http.ResponseWriter, r *http.Request) {
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
	var req assignCustomRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}
	if req.RoleID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("role_id is required"))
		return
	}
	if err := h.svc.AssignCustomRole(r.Context(), claims.UserID, targetUserID, req.RoleID, strPtrOrNil(req.TenantID)); err != nil {
		writeErrorFromDomain(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
