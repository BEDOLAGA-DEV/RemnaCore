package tariff

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
)

// --- Pricing Rule DTOs ---

// PricingRuleInput is the request body for creating or updating a pricing rule.
type PricingRuleInput struct {
	Name            string                `json:"name"`
	BasePrices      []BasePrice           `json:"base_prices"`
	Modifiers       []PricingRuleModifier `json:"modifiers"`
	StackingPolicy  string                `json:"stacking_policy"` // "additive" | "multiplicative" | "best_only"
	PriceFloorCents int64                 `json:"price_floor_cents"`
	Rounding        string                `json:"rounding"` // "none" | "charm_99" | "charm_95"
}

// BasePrice is a single base price entry in a specific currency.
type BasePrice struct {
	Currency    string `json:"currency"`
	AmountCents int64  `json:"amount_cents"`
}

// PricingRuleResponse wraps PricingRuleInput with server-assigned fields.
type PricingRuleResponse struct {
	ID string `json:"id"`
	PricingRuleInput
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// --- Handlers ---

// ListPricingRules returns all pricing rules.
func (h *Handler) ListPricingRules(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionPricingRules)
	if err != nil {
		h.logger.Error("failed to list pricing rules", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	rules := make([]PricingRuleResponse, 0, len(docs))
	for _, doc := range docs {
		pr, convErr := documentToPricingRule(&doc)
		if convErr != nil {
			continue
		}
		rules = append(rules, *pr)
	}

	writeJSON(w, http.StatusOK, rules)
}

// GetPricingRule returns a single pricing rule by ID.
func (h *Handler) GetPricingRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "pricingRuleID")
	if ruleID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("pricing rule ID is required"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionPricingRules, ruleID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	pr, err := documentToPricingRule(doc)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, pr)
}

// CreatePricingRule creates a new pricing rule.
func (h *Handler) CreatePricingRule(w http.ResponseWriter, r *http.Request) {
	var input PricingRuleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := validatePricingRuleInput(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(err.Error()))
		return
	}

	data, err := json.Marshal(input)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	doc, err := h.collections.InsertDocument(r.Context(), PluginSlug, CollectionPricingRules, json.RawMessage(data))
	if err != nil {
		h.logger.Error("failed to create pricing rule", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	pr, err := documentToPricingRule(doc)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusCreated, pr)
}

// UpdatePricingRule updates an existing pricing rule by ID.
func (h *Handler) UpdatePricingRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "pricingRuleID")
	if ruleID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("pricing rule ID is required"))
		return
	}

	var input PricingRuleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := validatePricingRuleInput(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(err.Error()))
		return
	}

	data, err := json.Marshal(input)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	if err := h.collections.UpdateDocument(r.Context(), PluginSlug, CollectionPricingRules, ruleID, json.RawMessage(data)); err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionPricingRules, ruleID)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	pr, err := documentToPricingRule(doc)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, pr)
}

// DeletePricingRule removes a pricing rule by ID.
func (h *Handler) DeletePricingRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "pricingRuleID")
	if ruleID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("pricing rule ID is required"))
		return
	}

	if err := h.collections.DeleteDocument(r.Context(), PluginSlug, CollectionPricingRules, ruleID); err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// simulateRequest is the input for dry-run pricing simulation.
type simulateRequest struct {
	Currency       string `json:"currency"`
	Country        string `json:"country"`
	DurationDays   int    `json:"duration_days"`
	Quantity       int    `json:"quantity"`
	BasePriceCents int64  `json:"base_price_cents"`
}

// SimulatePricingRule performs a dry-run pricing calculation using the rule's modifiers.
func (h *Handler) SimulatePricingRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "pricingRuleID")
	if ruleID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("pricing rule ID is required"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionPricingRules, ruleID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	var rule PricingRuleInput
	if err := json.Unmarshal(doc.Data, &rule); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	var req simulateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if req.BasePriceCents <= 0 {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("base_price_cents must be > 0"))
		return
	}

	// Sort modifiers by priority.
	mods := make([]PricingRuleModifier, len(rule.Modifiers))
	copy(mods, rule.Modifiers)
	sort.Slice(mods, func(i, j int) bool {
		return mods[i].Priority < mods[j].Priority
	})

	ctx := &PriceContext{
		Currency:  req.Currency,
		Country:   req.Country,
		Duration:  req.DurationDays,
		Quantity:  req.Quantity,
		BasePrice: req.BasePriceCents,
	}

	// Run simulation through the modifiers.
	pipeline := NewPricingPipeline(
		&geoAdjustStage{modifiers: mods},
		&bulkDiscountStage{modifiers: mods},
		&priceFloorStage{floorCents: rule.PriceFloorCents},
	)

	if err := pipeline.Calculate(ctx); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"base_price":  req.BasePriceCents,
		"final_price": ctx.FinalPrice,
		"modifiers":   ctx.Modifiers,
		"breakdown":   ctx.Breakdown,
	})
}

// --- Helpers ---

func validatePricingRuleInput(input *PricingRuleInput) error {
	if input.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(input.BasePrices) == 0 {
		return fmt.Errorf("at least one base_price is required")
	}
	for i, bp := range input.BasePrices {
		if len(bp.Currency) != 3 {
			return fmt.Errorf("base_prices[%d].currency must be 3 characters", i)
		}
		if bp.AmountCents < 0 {
			return fmt.Errorf("base_prices[%d].amount_cents must be >= 0", i)
		}
	}
	validStacking := map[string]bool{
		"additive": true, "multiplicative": true, "best_only": true,
	}
	if input.StackingPolicy != "" && !validStacking[input.StackingPolicy] {
		return fmt.Errorf("unsupported stacking_policy: %s", input.StackingPolicy)
	}
	if input.StackingPolicy == "" {
		input.StackingPolicy = "multiplicative"
	}
	if input.PriceFloorCents < 0 {
		return fmt.Errorf("price_floor_cents must be >= 0")
	}
	return nil
}

func documentToPricingRule(doc *pluginstore.Document) (*PricingRuleResponse, error) {
	var r PricingRuleResponse
	if err := json.Unmarshal(doc.Data, &r); err != nil {
		return nil, fmt.Errorf("unmarshal pricing rule: %w", err)
	}
	r.ID = doc.ID
	r.CreatedAt = doc.CreatedAt.Format(time.RFC3339)
	r.UpdatedAt = doc.UpdatedAt.Format(time.RFC3339)
	return &r, nil
}
