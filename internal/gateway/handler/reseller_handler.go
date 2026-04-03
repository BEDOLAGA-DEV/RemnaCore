package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
)

// ResellerHandler exposes HTTP endpoints for reseller and white-label tenant
// management.
type ResellerHandler struct {
	service *reseller.ResellerService
}

// NewResellerHandler creates a ResellerHandler backed by the given reseller
// service.
func NewResellerHandler(service *reseller.ResellerService) *ResellerHandler {
	return &ResellerHandler{service: service}
}

// --- Request DTOs ---

type createTenantRequest struct {
	Name        string `json:"name"`
	Domain      string `json:"domain"`
	OwnerUserID string `json:"owner_user_id"`
}

type updateBrandingRequest struct {
	Logo         string `json:"logo"`
	PrimaryColor string `json:"primary_color"`
	AppName      string `json:"app_name"`
	SupportEmail string `json:"support_email"`
	SupportURL   string `json:"support_url"`
}

// --- Admin Endpoints ---

// CreateTenant handles POST /api/admin/tenants -- create a new tenant (admin only).
func (h *ResellerHandler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	var req createTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.OwnerUserID == "" {
		writeError(w, http.StatusBadRequest, "name and owner_user_id are required")
		return
	}

	tenant, plainKey, err := h.service.CreateTenant(r.Context(), req.Name, req.Domain, req.OwnerUserID)
	if err != nil {
		status, message := mapServiceError(err)
		writeError(w, status, message)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"tenant":  tenantToResponse(tenant),
		"api_key": plainKey,
	})
}

// ListTenants handles GET /api/admin/tenants -- list all tenants (admin only).
func (h *ResellerHandler) ListTenants(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	tenants, err := h.service.ListTenants(r.Context(), limit, offset)
	if err != nil {
		status, message := mapServiceError(err)
		writeError(w, status, message)
		return
	}

	items := make([]map[string]interface{}, 0, len(tenants))
	for _, t := range tenants {
		items = append(items, tenantToResponse(t))
	}
	writeJSON(w, http.StatusOK, items)
}

// GetTenant handles GET /api/admin/tenants/{tenantID} -- tenant detail (admin only).
func (h *ResellerHandler) GetTenant(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant ID is required")
		return
	}

	tenant, err := h.service.GetTenant(r.Context(), tenantID)
	if err != nil {
		status, message := mapServiceError(err)
		writeError(w, status, message)
		return
	}

	writeJSON(w, http.StatusOK, tenantToResponse(tenant))
}

// UpdateBranding handles PUT /api/admin/tenants/{tenantID}/branding -- update branding (admin only).
func (h *ResellerHandler) UpdateBranding(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant ID is required")
		return
	}

	var req updateBrandingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	branding := reseller.BrandingConfig{
		Logo:         req.Logo,
		PrimaryColor: req.PrimaryColor,
		AppName:      req.AppName,
		SupportEmail: req.SupportEmail,
		SupportURL:   req.SupportURL,
	}

	tenant, err := h.service.UpdateBranding(r.Context(), tenantID, branding)
	if err != nil {
		status, message := mapServiceError(err)
		writeError(w, status, message)
		return
	}

	writeJSON(w, http.StatusOK, tenantToResponse(tenant))
}

// --- Reseller Self-Service Endpoints ---

// Dashboard handles GET /api/reseller/dashboard -- reseller's own dashboard.
func (h *ResellerHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.GetTenant(r.Context())
	if tenant == nil {
		writeError(w, http.StatusForbidden, "tenant context required")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenant_id":   tenant.ID,
		"tenant_name": tenant.Name,
		"branding":    tenant.BrandingConfig,
	})
}

// Commissions handles GET /api/reseller/commissions -- reseller's commissions.
func (h *ResellerHandler) Commissions(w http.ResponseWriter, r *http.Request) {
	// For now, commissions endpoint returns a placeholder.
	// Full implementation requires resolving the reseller account from the
	// claims user ID and tenant context.
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"commissions": []interface{}{},
	})
}

// Customers handles GET /api/reseller/customers -- reseller's customers scoped by tenant.
func (h *ResellerHandler) Customers(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.GetTenant(r.Context())
	if tenant == nil {
		writeError(w, http.StatusForbidden, "tenant context required")
		return
	}

	// Scoped customer list -- placeholder for Phase 6 iteration.
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenant_id": tenant.ID,
		"customers": []interface{}{},
	})
}

// tenantToResponse converts a Tenant to a JSON-friendly map.
func tenantToResponse(t *reseller.Tenant) map[string]interface{} {
	return map[string]interface{}{
		"id":              t.ID,
		"name":            t.Name,
		"domain":          t.Domain,
		"owner_user_id":   t.OwnerUserID,
		"branding_config": t.BrandingConfig,
		"is_active":       t.IsActive,
		"created_at":      t.CreatedAt,
		"updated_at":      t.UpdatedAt,
	}
}
