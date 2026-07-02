package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	resellerservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
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

// setBotRequest is the request body for SetResellerBot and UpdateTenantBot.
type setBotRequest struct {
	BotToken   string `json:"bot_token"`
	CabinetURL string `json:"cabinet_url"`
	Enabled    bool   `json:"enabled"`
	BotPlugin  string `json:"bot_plugin"`
}

// getBotResponse is the safe GET response — token is NEVER included.
type getBotResponse struct {
	Enabled     bool   `json:"enabled"`
	CabinetURL  string `json:"cabinet_url"`
	BotUsername string `json:"bot_username"`
	HasToken    bool   `json:"has_token"`
	BotPlugin   string `json:"bot_plugin"`
}

// --- Admin Endpoints ---

// CreateTenant handles POST /api/admin/tenants -- create a new tenant (admin only).
func (h *ResellerHandler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	var req createTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if req.Name == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("name is required"))
		return
	}

	tenant, plainKey, err := h.service.CreateTenant(r.Context(), req.Name, req.Domain, pgutil.StrPtrOrNil(req.OwnerUserID))
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"tenant":  tenantToResponse(tenant),
		"api_key": plainKey,
	})
}

// ListTenants handles GET /api/admin/tenants -- list all tenants (admin only).
func (h *ResellerHandler) ListTenants(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	tenants, err := h.service.ListTenants(r.Context(), limit, offset)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	items := make([]map[string]any, 0, len(tenants))
	for _, t := range tenants {
		items = append(items, tenantToResponse(t))
	}
	writeJSON(w, http.StatusOK, items)
}

// GetTenant handles GET /api/admin/tenants/{tenantID} -- tenant detail (admin only).
func (h *ResellerHandler) GetTenant(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	if tenantID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("tenant ID is required"))
		return
	}

	tenant, err := h.service.GetTenant(r.Context(), tenantID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, tenantToResponse(tenant))
}

// UpdateBranding handles PUT /api/admin/tenants/{tenantID}/branding -- update branding (admin only).
func (h *ResellerHandler) UpdateBranding(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	if tenantID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("tenant ID is required"))
		return
	}

	var req updateBrandingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
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
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, tenantToResponse(tenant))
}

// --- Reseller Self-Service Endpoints ---

// Commissions handles GET /api/reseller/commissions -- the active shop's
// commissions. The reseller account is resolved server-side from the JWT user
// and the active shop (never from request input); rows are tenant-scoped via
// RLS under RunInTx in the service.
func (h *ResellerHandler) Commissions(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.ActiveTenantID(r.Context())
	if tenantID == "" {
		writeAPIError(w, apierror.Forbidden.WithDetails("active shop required (X-Shop-Id)"))
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Forbidden.WithDetails("authentication required"))
		return
	}

	commissions, err := h.service.ListCommissions(r.Context(), claims.UserID, tenantID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	items := make([]map[string]any, 0, len(commissions))
	for _, c := range commissions {
		items = append(items, map[string]any{
			"id":         c.ID,
			"sale_id":    c.SaleID,
			"amount":     c.Amount.Amount,
			"currency":   string(c.Amount.Currency),
			"status":     string(c.Status),
			"created_at": c.CreatedAt,
			"paid_at":    c.PaidAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tenant_id":   tenantID,
		"commissions": items,
	})
}

// Customers handles GET /api/reseller/customers -- the active shop's customers
// (identity.platform_users tenant-scoped to the active shop).
func (h *ResellerHandler) Customers(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.ActiveTenantID(r.Context())
	if tenantID == "" {
		writeAPIError(w, apierror.Forbidden.WithDetails("active shop required (X-Shop-Id)"))
		return
	}
	limit, offset := parsePagination(r)

	customers, err := h.service.ListCustomers(r.Context(), tenantID, limit, offset)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	items := make([]map[string]any, 0, len(customers))
	for _, c := range customers {
		items = append(items, map[string]any{
			"user_id":           c.UserID,
			"email":             c.Email,
			"display_name":      c.DisplayName,
			"active_subs_count": c.ActiveSubsCount,
			"created_at":        c.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tenant_id": tenantID,
		"customers": items,
	})
}

// Dashboard handles GET /api/reseller/dashboard -- the active shop's dashboard:
// branding plus purpose-built tenant-scoped aggregates (active customers,
// active subscriptions, pending commission). NOT the platform /api/admin/stats.
func (h *ResellerHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.ActiveTenantID(r.Context())
	if tenantID == "" {
		writeAPIError(w, apierror.Forbidden.WithDetails("active shop required (X-Shop-Id)"))
		return
	}

	tenant, err := h.service.GetTenant(r.Context(), tenantID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}
	summary, err := h.service.DashboardSummary(r.Context(), tenantID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"tenant_id":   tenant.ID,
		"tenant_name": tenant.Name,
		"branding":    tenant.BrandingConfig,
		"summary": map[string]any{
			"active_customers":     summary.ActiveCustomers,
			"active_subscriptions": summary.ActiveSubscriptions,
			"pending_commission":   summary.PendingCommission,
			"currency":             summary.Currency,
		},
	})
}

// SetResellerBot handles PUT /api/reseller/bot -- upsert the active shop's
// Telegram bot configuration. An empty bot_token means "keep the stored token";
// format validation and webhook-secret generation are delegated to the service.
func (h *ResellerHandler) SetResellerBot(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.ActiveTenantID(r.Context())
	if tenantID == "" {
		writeAPIError(w, apierror.Forbidden.WithDetails("active shop required (X-Shop-Id)"))
		return
	}

	var req setBotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if err := h.service.SetShopBot(r.Context(), tenantID, resellerservice.SetShopBotInput{
		BotToken:   req.BotToken,
		CabinetURL: req.CabinetURL,
		Enabled:    req.Enabled,
		BotPlugin:  req.BotPlugin,
	}); err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetResellerBot handles GET /api/reseller/bot -- the active shop's bot config.
// The token is NEVER included in the response; callers receive only enabled,
// cabinet_url, bot_username, and has_token (a boolean indicating whether a
// token is stored). A missing config is returned as an all-false empty config.
func (h *ResellerHandler) GetResellerBot(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.ActiveTenantID(r.Context())
	if tenantID == "" {
		writeAPIError(w, apierror.Forbidden.WithDetails("active shop required (X-Shop-Id)"))
		return
	}

	h.respondBotConfig(w, r, tenantID)
}

// GetTenantBot handles GET /api/admin/tenants/{tenantID}/bot -- admin-level
// read of any tenant's Telegram bot configuration (token never returned; only
// has_token). Twin of GetResellerBot with an explicit tenant path param.
func (h *ResellerHandler) GetTenantBot(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	if tenantID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("tenant ID is required"))
		return
	}

	h.respondBotConfig(w, r, tenantID)
}

// respondBotConfig writes the redacted bot configuration for tenantID: an empty
// getBotResponse when none is configured, the mapped view otherwise. Shared by
// the reseller- and admin-facing GET handlers.
func (h *ResellerHandler) respondBotConfig(w http.ResponseWriter, r *http.Request, tenantID string) {
	cfg, err := h.service.GetShopBot(r.Context(), tenantID)
	if err != nil {
		if errors.Is(err, reseller.ErrShopBotNotFound) {
			writeJSON(w, http.StatusOK, getBotResponse{})
			return
		}
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, getBotResponse{
		Enabled:     cfg.Enabled,
		CabinetURL:  cfg.CabinetURL,
		BotUsername: cfg.BotUsername,
		HasToken:    cfg.Token.Expose() != "",
		BotPlugin:   cfg.BotPluginSlug,
	})
}

// UpdateTenantBot handles PUT /api/admin/tenants/{tenantID}/bot -- admin-level
// upsert of any tenant's Telegram bot configuration. An empty bot_token means
// "keep the stored token"; format validation is delegated to the service.
func (h *ResellerHandler) UpdateTenantBot(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	if tenantID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("tenant ID is required"))
		return
	}

	var req setBotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if err := h.service.SetShopBot(r.Context(), tenantID, resellerservice.SetShopBotInput{
		BotToken:   req.BotToken,
		CabinetURL: req.CabinetURL,
		Enabled:    req.Enabled,
		BotPlugin:  req.BotPlugin,
	}); err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// tenantToResponse converts a Tenant to a JSON-friendly map.
func tenantToResponse(t *reseller.Tenant) map[string]any {
	return map[string]any{
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
