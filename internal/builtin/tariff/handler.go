// Package tariff implements the tariff-manager built-in plugin.
// All tariff logic is self-contained here — the platform only calls
// Plugin() and RegisterRoutes().
package tariff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
)

const (
	// PluginSlug is the canonical slug for the tariff-manager plugin.
	PluginSlug = "tariff-manager"

	// CollectionName is the plugin collection where tariffs are stored.
	CollectionName = "tariffs"
)

// Handler provides HTTP endpoints for tariff CRUD, pricing, promo codes,
// A/B experiments, cohorts, analytics, and Remnawave data lookups.
type Handler struct {
	collections pluginstore.Store
	pluginRepo  plugin.PluginRepository
	planRepo    billing.PlanRepository
	logger      *slog.Logger
}

// NewHandler creates a tariff Handler.
func NewHandler(collections pluginstore.Store, pluginRepo plugin.PluginRepository, planRepo billing.PlanRepository, logger *slog.Logger) *Handler {
	return &Handler{
		collections: collections,
		pluginRepo:  pluginRepo,
		planRepo:    planRepo,
		logger:      logger,
	}
}

// remnawaveClientForPanel builds a client from a specific panel in the
// panel_connections collection. If panelID is empty, uses the first active panel.
func (h *Handler) remnawaveClientForPanel(ctx context.Context, panelID string) *remnawave.Client {
	const rwSlug = "remnawave-provider"
	const rwCollection = "panel_connections"

	docs, err := h.collections.ListDocuments(ctx, rwSlug, rwCollection)
	if err != nil {
		return nil
	}

	for _, doc := range docs {
		var panel struct {
			URL      string `json:"url"`
			APIToken string `json:"api_token"`
			Status   string `json:"status"`
		}
		if json.Unmarshal(doc.Data, &panel) != nil {
			continue
		}
		if panel.Status != "active" {
			continue
		}
		// If specific panel requested, match by ID
		if panelID != "" && doc.ID != panelID {
			continue
		}
		if panel.URL != "" && panel.APIToken != "" {
			return remnawave.NewClient(panel.URL, panel.APIToken)
		}
	}
	return nil
}

// =====================================================================
// Tariff CRUD
// =====================================================================

func (h *Handler) ListTariffs(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionName)
	if err != nil {
		h.logger.Error("failed to list tariffs", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	activeOnly := r.URL.Query().Get("active") == "true"
	audience := r.URL.Query().Get("audience")
	country := r.URL.Query().Get("country")

	tariffs := make([]TariffResponse, 0, len(docs))
	for _, doc := range docs {
		t, convErr := documentToTariff(&doc)
		if convErr != nil {
			continue
		}
		if activeOnly && !t.IsActive {
			continue
		}
		if audience != "" && t.AudienceSegment != "" && t.AudienceSegment != AudienceAll && t.AudienceSegment != audience {
			continue
		}
		_ = country // geo-filtering reserved for tariff.list_visible hook
		tariffs = append(tariffs, *t)
	}

	slices.SortFunc(tariffs, func(a, b TariffResponse) int {
		return a.SortOrder - b.SortOrder
	})

	writeJSON(w, http.StatusOK, tariffs)
}

func (h *Handler) GetTariff(w http.ResponseWriter, r *http.Request) {
	tariffID := chi.URLParam(r, "tariffID")
	if tariffID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("tariff ID is required"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionName, tariffID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	t, err := documentToTariff(doc)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) CreateTariff(w http.ResponseWriter, r *http.Request) {
	input := defaultTariffInput()
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := validateTariffInputV2(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(err.Error()))
		return
	}

	data, err := json.Marshal(input)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	doc, err := h.collections.InsertDocument(r.Context(), PluginSlug, CollectionName, json.RawMessage(data))
	if err != nil {
		h.logger.Error("failed to create tariff", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	t, err := documentToTariff(doc)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	// Sync tariff → billing Plan for cabinet visibility
	h.syncTariffToPlan(r.Context(), doc.ID, &input)

	writeJSON(w, http.StatusCreated, t)
}

func (h *Handler) UpdateTariff(w http.ResponseWriter, r *http.Request) {
	tariffID := chi.URLParam(r, "tariffID")
	if tariffID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("tariff ID is required"))
		return
	}

	input := defaultTariffInput()
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := validateTariffInputV2(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(err.Error()))
		return
	}

	data, err := json.Marshal(input)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	if err := h.collections.UpdateDocument(r.Context(), PluginSlug, CollectionName, tariffID, json.RawMessage(data)); err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionName, tariffID)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	t, err := documentToTariff(doc)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	// Sync tariff → billing Plan
	h.syncTariffToPlan(r.Context(), tariffID, &input)

	writeJSON(w, http.StatusOK, t)
}

// DeleteTariff soft-deletes a tariff by setting is_active=false.
// Hard deletes are intentionally not supported — tariffs with purchase
// history are audit objects (see plan §14).
func (h *Handler) DeleteTariff(w http.ResponseWriter, r *http.Request) {
	tariffID := chi.URLParam(r, "tariffID")
	if tariffID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("tariff ID is required"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionName, tariffID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	var input TariffInput
	if err := json.Unmarshal(doc.Data, &input); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	input.IsActive = false

	data, err := json.Marshal(input)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	if err := h.collections.UpdateDocument(r.Context(), PluginSlug, CollectionName, tariffID, json.RawMessage(data)); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// =====================================================================
// Tariff extended endpoints
// =====================================================================

// GetTariffPrice calculates the personalized price for a tariff.
func (h *Handler) GetTariffPrice(w http.ResponseWriter, r *http.Request) {
	tariffID := chi.URLParam(r, "tariffID")
	if tariffID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("tariff ID is required"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionName, tariffID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	var tariff TariffInput
	if err := json.Unmarshal(doc.Data, &tariff); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	q := r.URL.Query()
	currency := q.Get("currency")
	if currency == "" {
		currency = tariff.PriceCurrency
	}

	ctx := &PriceContext{
		TariffID:  tariffID,
		UserID:    q.Get("user_id"),
		Country:   q.Get("country"),
		Currency:  currency,
		Duration:  tariff.DurationDays,
		Quantity:  1,
		PromoCode: q.Get("promo"),
	}

	// Load pricing rule if configured
	var rule *PricingRuleInput
	if tariff.PricingRuleID != "" {
		ruleDoc, ruleErr := h.collections.GetDocument(r.Context(), PluginSlug, CollectionPricingRules, tariff.PricingRuleID)
		if ruleErr == nil {
			var pr PricingRuleInput
			if json.Unmarshal(ruleDoc.Data, &pr) == nil {
				rule = &pr
			}
		}
	}

	// Load promo code if provided
	var promo *PromoCodeInput
	if ctx.PromoCode != "" {
		promo = h.findPromoByCode(r.Context(), ctx.PromoCode)
	}

	pipeline := DefaultPipeline(&tariff, rule, promo)
	if err := pipeline.Calculate(ctx); err != nil {
		h.logger.Error("pricing calculation failed", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, ctx)
}

// findPromoByCode scans promo codes collection for a matching code.
func (h *Handler) findPromoByCode(ctx context.Context, code string) *PromoCodeInput {
	docs, err := h.collections.ListDocuments(ctx, PluginSlug, CollectionPromoCodes)
	if err != nil {
		return nil
	}
	for _, doc := range docs {
		var p PromoCodeInput
		if json.Unmarshal(doc.Data, &p) == nil && strings.EqualFold(p.Code, code) && p.IsActive {
			return &p
		}
	}
	return nil
}

// CloneTariff duplicates a tariff with a new ID.
func (h *Handler) CloneTariff(w http.ResponseWriter, r *http.Request) {
	tariffID := chi.URLParam(r, "tariffID")
	if tariffID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("tariff ID is required"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionName, tariffID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	// Modify the name to indicate it's a clone
	var input TariffInput
	if err := json.Unmarshal(doc.Data, &input); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	input.Name = input.Name + " (copy)"
	input.IsActive = false

	data, err := json.Marshal(input)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	newDoc, err := h.collections.InsertDocument(r.Context(), PluginSlug, CollectionName, json.RawMessage(data))
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	t, err := documentToTariff(newDoc)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusCreated, t)
}

// ArchiveTariff soft-deletes a tariff by setting is_active=false.
func (h *Handler) ArchiveTariff(w http.ResponseWriter, r *http.Request) {
	tariffID := chi.URLParam(r, "tariffID")
	if tariffID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("tariff ID is required"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionName, tariffID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	var input TariffInput
	if err := json.Unmarshal(doc.Data, &input); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	input.IsActive = false

	data, err := json.Marshal(input)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	if err := h.collections.UpdateDocument(r.Context(), PluginSlug, CollectionName, tariffID, json.RawMessage(data)); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"tariff_id":   tariffID,
		"archived_at": time.Now().Format(time.RFC3339),
		"reason":      "admin_action",
	})
}

// GetTariffStats returns basic sales stats for a tariff (placeholder).
func (h *Handler) GetTariffStats(w http.ResponseWriter, r *http.Request) {
	tariffID := chi.URLParam(r, "tariffID")
	if tariffID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("tariff ID is required"))
		return
	}
	if _, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionName, tariffID); err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tariff_id":     tariffID,
		"total_sales":   0,
		"total_revenue": 0,
		"active_subs":   0,
	})
}

// GetTariffVersionHistory returns change history for a tariff (placeholder).
func (h *Handler) GetTariffVersionHistory(w http.ResponseWriter, r *http.Request) {
	tariffID := chi.URLParam(r, "tariffID")
	if tariffID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("tariff ID is required"))
		return
	}
	if _, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionName, tariffID); err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tariff_id": tariffID,
		"versions":  []any{},
	})
}

// ListTariffCatalog returns tariffs grouped and sorted for storefront display.
func (h *Handler) ListTariffCatalog(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionName)
	if err != nil {
		h.logger.Error("failed to list tariffs for catalog", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	tariffs := make([]TariffResponse, 0, len(docs))
	for _, doc := range docs {
		t, convErr := documentToTariff(&doc)
		if convErr != nil {
			continue
		}
		if !t.IsActive {
			continue
		}
		if !t.VisibleInPublic {
			continue
		}
		tariffs = append(tariffs, *t)
	}

	slices.SortFunc(tariffs, func(a, b TariffResponse) int {
		return a.SortOrder - b.SortOrder
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"tariffs": tariffs,
		"count":   len(tariffs),
	})
}

// ExportTariffs exports all tariffs as JSON.
func (h *Handler) ExportTariffs(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionName)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	tariffs := make([]TariffResponse, 0, len(docs))
	for _, doc := range docs {
		t, convErr := documentToTariff(&doc)
		if convErr != nil {
			continue
		}
		tariffs = append(tariffs, *t)
	}

	w.Header().Set("Content-Disposition", "attachment; filename=tariffs_export.json")
	writeJSON(w, http.StatusOK, tariffs)
}

// ImportTariffs imports tariffs from a JSON array.
func (h *Handler) ImportTariffs(w http.ResponseWriter, r *http.Request) {
	var inputs []TariffInput
	if err := json.NewDecoder(r.Body).Decode(&inputs); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body: expected JSON array of tariffs"))
		return
	}

	const maxImportSize = 100
	if len(inputs) > maxImportSize {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(
			fmt.Sprintf("import limited to %d tariffs per request, got %d", maxImportSize, len(inputs))))
		return
	}

	imported := 0
	errs := make([]string, 0)
	for i, input := range inputs {
		if err := validateTariffInputV2(&input); err != nil {
			errs = append(errs, fmt.Sprintf("index %d: %s", i, err.Error()))
			continue
		}

		data, err := json.Marshal(input)
		if err != nil {
			errs = append(errs, fmt.Sprintf("index %d: marshal error", i))
			continue
		}

		if _, err := h.collections.InsertDocument(r.Context(), PluginSlug, CollectionName, json.RawMessage(data)); err != nil {
			errs = append(errs, fmt.Sprintf("index %d: insert error", i))
			continue
		}
		imported++
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"imported": imported,
		"errors":   errs,
		"total":    len(inputs),
	})
}

// SyncAllTariffs forces a sync of all existing tariffs to billing Plans.
func (h *Handler) SyncAllTariffs(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionName)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	synced := 0
	for _, doc := range docs {
		var input TariffInput
		if json.Unmarshal(doc.Data, &input) != nil {
			continue
		}
		h.syncTariffToPlan(r.Context(), doc.ID, &input)
		synced++
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"synced": synced,
		"total":  len(docs),
	})
}

// =====================================================================
// Analytics endpoints (placeholders)
// =====================================================================

func (h *Handler) GetMRRByTariff(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"mrr_by_tariff": []any{},
		"total_mrr":     0,
	})
}

func (h *Handler) GetConversionFunnel(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"stages": []map[string]any{
			{"name": "viewed", "count": 0},
			{"name": "price_checked", "count": 0},
			{"name": "checkout_started", "count": 0},
			{"name": "purchased", "count": 0},
		},
	})
}

// =====================================================================
// Reseller endpoints (placeholders)
// =====================================================================

func (h *Handler) GetResellerCatalog(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionName)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	tariffs := make([]TariffResponse, 0)
	for _, doc := range docs {
		t, convErr := documentToTariff(&doc)
		if convErr != nil {
			continue
		}
		if !t.IsActive || !t.VisibleInResellerPortal {
			continue
		}
		tariffs = append(tariffs, *t)
	}

	writeJSON(w, http.StatusOK, tariffs)
}

func (h *Handler) CustomizeReseller(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "not_implemented",
	})
}

// GetPromoCodeStats returns usage statistics for a promo code.
func (h *Handler) GetPromoCodeStats(w http.ResponseWriter, r *http.Request) {
	promoCodeID := chi.URLParam(r, "promoCodeID")
	if promoCodeID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("promo code ID is required"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionPromoCodes, promoCodeID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	var code PromoCodeInput
	if err := json.Unmarshal(doc.Data, &code); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"promo_code_id":    promoCodeID,
		"code":             code.Code,
		"uses_count":       code.Usage.UsesCount,
		"unique_users":     code.Usage.UniqueUsersCount,
		"max_uses_total":   code.Usage.MaxUsesTotal,
		"is_active":        code.IsActive,
		"remaining_uses":   max(0, code.Usage.MaxUsesTotal-code.Usage.UsesCount),
	})
}

// =====================================================================
// Remnawave lookups (unchanged)
// =====================================================================

func (h *Handler) ListInternalSquads(w http.ResponseWriter, r *http.Request) {
	panelID := r.URL.Query().Get("panel_id")
	client := h.remnawaveClientForPanel(r.Context(), panelID)
	if client == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	squads, err := client.GetInternalSquads(r.Context())
	if err != nil {
		h.logger.Error("failed to list internal squads", slog.Any("error", err))
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	if squads == nil {
		squads = []remnawave.RemnawaveSquad{}
	}
	writeJSON(w, http.StatusOK, squads)
}

func (h *Handler) ListExternalSquads(w http.ResponseWriter, r *http.Request) {
	panelID := r.URL.Query().Get("panel_id")
	client := h.remnawaveClientForPanel(r.Context(), panelID)
	if client == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	squads, err := client.GetExternalSquads(r.Context())
	if err != nil {
		h.logger.Error("failed to list external squads", slog.Any("error", err))
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	if squads == nil {
		squads = []remnawave.RemnawaveSquad{}
	}
	writeJSON(w, http.StatusOK, squads)
}

func (h *Handler) ListNodes(w http.ResponseWriter, r *http.Request) {
	panelID := r.URL.Query().Get("panel_id")
	client := h.remnawaveClientForPanel(r.Context(), panelID)
	if client == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	nodes, err := client.GetNodes(r.Context())
	if err != nil {
		h.logger.Error("failed to list nodes from remnawave", slog.Any("error", err))
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	if nodes == nil {
		nodes = []remnawave.RemnawaveNode{}
	}
	writeJSON(w, http.StatusOK, nodes)
}

// ListPanelsForTariff returns panel connections for tariff form dropdown.
func (h *Handler) ListPanelsForTariff(w http.ResponseWriter, r *http.Request) {
	const rwSlug = "remnawave-provider"
	const rwCollection = "panel_connections"

	docs, err := h.collections.ListDocuments(r.Context(), rwSlug, rwCollection)
	if err != nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	type panelOption struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	panels := make([]panelOption, 0)
	for _, doc := range docs {
		var p struct {
			Slug   string `json:"slug"`
			URL    string `json:"url"`
			Status string `json:"status"`
		}
		if json.Unmarshal(doc.Data, &p) != nil || p.Status != "active" {
			continue
		}
		label := p.Slug
		if label == "" {
			label = p.URL
		}
		panels = append(panels, panelOption{ID: doc.ID, Name: label})
	}

	writeJSON(w, http.StatusOK, panels)
}

// =====================================================================
// Helpers
// =====================================================================

func documentToTariff(doc *pluginstore.Document) (*TariffResponse, error) {
	var t TariffResponse
	if err := json.Unmarshal(doc.Data, &t); err != nil {
		return nil, err
	}
	t.ID = doc.ID
	t.CreatedAt = doc.CreatedAt.Format(time.RFC3339)
	t.UpdatedAt = doc.UpdatedAt.Format(time.RFC3339)

	// Nil-safe slice defaults for backward compat
	if t.InternalSquadUUIDs == nil {
		t.InternalSquadUUIDs = []string{}
	}
	if t.ExternalSquadUUIDs == nil {
		t.ExternalSquadUUIDs = []string{}
	}
	if t.Features == nil {
		t.Features = []string{}
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}

	// Default billing model for old documents
	if t.BillingModel == "" {
		t.BillingModel = string(BillingModelFixed)
	}
	if t.AudienceSegment == "" {
		t.AudienceSegment = AudienceAll
	}

	return &t, nil
}

// syncTariffToPlan creates or updates a billing Plan from a tariff document.
// This ensures tariffs created in the plugin are visible in the cabinet
// via the standard usePlans() hook.
func (h *Handler) syncTariffToPlan(ctx context.Context, docID string, input *TariffInput) {
	if h.planRepo == nil {
		return
	}

	// Map duration_days to billing interval
	interval := durationToInterval(input.DurationDays)

	// Map price
	currency := vo.Currency(strings.ToLower(input.PriceCurrency))
	if currency == "" {
		currency = "usd"
	}
	price := vo.NewMoney(input.PriceAmount, currency)

	// Map tier from audience/billing model
	tier := aggregate.TierBasic
	if input.AudienceSegment == AudienceB2B {
		tier = aggregate.TierPremium
	}

	// Convert traffic GB to bytes
	var trafficBytes int64
	if input.TrafficLimitGB > 0 {
		trafficBytes = int64(input.TrafficLimitGB * 1024 * 1024 * 1024)
	}

	// Default country to allow all
	countries := []string{"ALL"}

	now := time.Now()

	// Try update first (existing plan)
	existing, err := h.planRepo.GetByID(ctx, docID)
	if err == nil && existing != nil {
		existing.Name = input.Name
		existing.Description = input.Description
		existing.BasePrice = price
		existing.Interval = interval
		existing.TrafficLimitBytes = trafficBytes
		existing.DeviceLimit = input.DeviceLimit
		existing.IsActive = input.IsActive
		existing.UpdatedAt = now
		if updateErr := h.planRepo.Update(ctx, existing); updateErr != nil {
			h.logger.Warn("failed to update billing plan from tariff",
				slog.String("tariff_id", docID), slog.Any("error", updateErr))
		}
		return
	}

	// Create new plan with tariff document ID as plan ID
	plan, err := aggregate.NewPlan(
		input.Name,
		input.Description,
		price,
		interval,
		trafficBytes,
		input.DeviceLimit,
		countries,
		[]string{}, // protocols
		tier,
		1,     // max remnawave bindings
		false, // family
		0,     // family members
		nil,   // no addons
		now,
	)
	if err != nil {
		h.logger.Warn("failed to build billing plan from tariff",
			slog.String("tariff_id", docID), slog.Any("error", err))
		return
	}

	// Override auto-generated ID with tariff document ID for 1:1 mapping
	plan.ID = docID

	if createErr := h.planRepo.Create(ctx, plan); createErr != nil {
		h.logger.Warn("failed to create billing plan from tariff",
			slog.String("tariff_id", docID), slog.Any("error", createErr))
	} else {
		h.logger.Info("synced tariff to billing plan", slog.String("id", docID), slog.String("name", input.Name))
	}
}

// durationToInterval maps duration days to a billing interval.
func durationToInterval(days int) vo.BillingInterval {
	switch {
	case days <= 31:
		return vo.IntervalMonth
	case days <= 92:
		return vo.IntervalQuarter
	default:
		return vo.IntervalYear
	}
}
