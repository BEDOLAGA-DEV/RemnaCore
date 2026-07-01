package tariff

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/money"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
)

// --- Promo code DTOs ---

// PromoCodeInput is the request body for creating or updating a promo code.
type PromoCodeInput struct {
	Code         string           `json:"code"`
	DiscountSpec DiscountSpec     `json:"discount_spec"`
	Eligibility  PromoEligibility `json:"eligibility"`
	Usage        PromoUsage       `json:"usage"`
	Validity     PromoValidity    `json:"validity"`
	Stacking     PromoStacking    `json:"stacking"`
	CampaignID   string           `json:"campaign_id"`
	Source       string           `json:"source"`
	CreatedBy    string           `json:"created_by"`
	IsActive     bool             `json:"is_active"`
}

// DiscountSpec defines the discount type and value.
type DiscountSpec struct {
	Type             string `json:"type"`               // "percent_off" | "fixed_off"
	Value            int64  `json:"value"`              // basis points for %, cents for fixed
	MaxDiscountCents int64  `json:"max_discount_cents"` // cap on discount amount
}

// PromoEligibility defines who can use a promo code.
type PromoEligibility struct {
	TariffIDs         []string `json:"tariff_ids,omitempty"`
	TariffIDsExclude  []string `json:"tariff_ids_exclude,omitempty"`
	AudienceSegments  []string `json:"audience_segments,omitempty"`
	Countries         []string `json:"countries,omitempty"`
	CountriesExclude  []string `json:"countries_exclude,omitempty"`
	FirstPurchaseOnly bool     `json:"first_purchase_only"`
	MinPurchaseCents  int64    `json:"min_purchase_cents"`
	MinDurationDays   int      `json:"min_duration_days"`
	RequiresTier      string   `json:"requires_tier"`
	UserIDsWhitelist  []string `json:"user_ids_whitelist,omitempty"`
	UserIDsBlacklist  []string `json:"user_ids_blacklist,omitempty"`
	PromoGroupIDs     []string `json:"promo_group_ids,omitempty"`
	PromoGroupExclude []string `json:"promo_group_exclude,omitempty"`
}

// PromoUsage tracks how many times a promo code has been used.
type PromoUsage struct {
	MaxUsesTotal     int `json:"max_uses_total"`
	MaxUsesPerUser   int `json:"max_uses_per_user"`
	MaxUsesPerTenant int `json:"max_uses_per_tenant"`
	UsesCount        int `json:"uses_count"`
	UniqueUsersCount int `json:"unique_users_count"`
}

// PromoValidity defines when a promo code is valid.
type PromoValidity struct {
	ValidFrom         string `json:"valid_from,omitempty"`
	ValidUntil        string `json:"valid_until,omitempty"`
	ValidOnDaysOfWeek []int  `json:"valid_on_days_of_week,omitempty"`
}

// PromoStacking controls how promo codes interact with other discounts.
type PromoStacking struct {
	StackableWithOthers       bool `json:"stackable_with_others"`
	StackableWithPricingRules bool `json:"stackable_with_pricing_rules"`
	Priority                  int  `json:"priority"`
}

// PromoCodeResponse wraps PromoCodeInput with server-assigned fields.
type PromoCodeResponse struct {
	ID string `json:"id"`
	PromoCodeInput
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// --- Handlers ---

// ListPromoCodes returns all promo codes.
func (h *Handler) ListPromoCodes(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionPromoCodes)
	if err != nil {
		h.logger.Error("failed to list promo codes", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	codes := make([]PromoCodeResponse, 0, len(docs))
	for _, doc := range docs {
		c, convErr := documentToPromoCode(&doc)
		if convErr != nil {
			continue
		}
		codes = append(codes, *c)
	}

	writeJSON(w, http.StatusOK, codes)
}

// GetPromoCode returns a single promo code by ID.
func (h *Handler) GetPromoCode(w http.ResponseWriter, r *http.Request) {
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

	c, err := documentToPromoCode(doc)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, c)
}

// CreatePromoCode creates a new promo code.
func (h *Handler) CreatePromoCode(w http.ResponseWriter, r *http.Request) {
	var input PromoCodeInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := validatePromoCodeInput(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(err.Error()))
		return
	}

	data, err := json.Marshal(input)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	doc, err := h.collections.InsertDocument(r.Context(), PluginSlug, CollectionPromoCodes, json.RawMessage(data))
	if err != nil {
		h.logger.Error("failed to create promo code", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	c, err := documentToPromoCode(doc)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusCreated, c)
}

// UpdatePromoCode updates an existing promo code by ID.
func (h *Handler) UpdatePromoCode(w http.ResponseWriter, r *http.Request) {
	promoCodeID := chi.URLParam(r, "promoCodeID")
	if promoCodeID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("promo code ID is required"))
		return
	}

	var input PromoCodeInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := validatePromoCodeInput(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(err.Error()))
		return
	}

	data, err := json.Marshal(input)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	if err := h.collections.UpdateDocument(r.Context(), PluginSlug, CollectionPromoCodes, promoCodeID, json.RawMessage(data)); err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionPromoCodes, promoCodeID)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	c, err := documentToPromoCode(doc)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, c)
}

// DeletePromoCode removes a promo code by ID.
func (h *Handler) DeletePromoCode(w http.ResponseWriter, r *http.Request) {
	promoCodeID := chi.URLParam(r, "promoCodeID")
	if promoCodeID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("promo code ID is required"))
		return
	}

	if err := h.collections.DeleteDocument(r.Context(), PluginSlug, CollectionPromoCodes, promoCodeID); err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// validatePromoRequest is the request body for POST /validate.
type validatePromoRequest struct {
	Code     string `json:"code"`
	TariffID string `json:"tariff_id"`
	UserID   string `json:"user_id"`
}

// ValidatePromoCode checks whether a promo code is valid without applying it.
func (h *Handler) ValidatePromoCode(w http.ResponseWriter, r *http.Request) {
	var req validatePromoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if req.Code == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("code is required"))
		return
	}

	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionPromoCodes)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	for _, doc := range docs {
		var code PromoCodeInput
		if json.Unmarshal(doc.Data, &code) != nil {
			continue
		}
		if !strings.EqualFold(code.Code, req.Code) {
			continue
		}

		// Found the code — check all validity conditions.
		// Return uniform "invalid code" for all failure modes to prevent oracle attacks.
		invalid := func() {
			writeJSON(w, http.StatusOK, map[string]any{"valid": false, "reason": "invalid code"})
		}

		if !code.IsActive {
			invalid()
			return
		}
		if code.Usage.MaxUsesTotal > 0 && code.Usage.UsesCount >= code.Usage.MaxUsesTotal {
			invalid()
			return
		}

		now := time.Now()
		if code.Validity.ValidFrom != "" {
			from, err := time.Parse(time.RFC3339, code.Validity.ValidFrom)
			if err == nil && now.Before(from) {
				invalid()
				return
			}
		}
		if code.Validity.ValidUntil != "" {
			until, err := time.Parse(time.RFC3339, code.Validity.ValidUntil)
			if err == nil && now.After(until) {
				invalid()
				return
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"valid":         true,
			"discount_spec": code.DiscountSpec,
		})
		return
	}

	// Same response for not-found — prevents code enumeration.
	writeJSON(w, http.StatusOK, map[string]any{"valid": false, "reason": "invalid code"})
}

// bulkGenerateRequest is the request body for POST /bulk-generate.
type bulkGenerateRequest struct {
	Count    int            `json:"count"`
	Prefix   string         `json:"prefix"`
	Template PromoCodeInput `json:"template"`
}

// maxPromoBatchCount is the maximum number of promo codes generatable in one
// bulk request.
const maxPromoBatchCount = 1000

// BulkGeneratePromoCodes generates N unique promo codes with a shared template.
func (h *Handler) BulkGeneratePromoCodes(w http.ResponseWriter, r *http.Request) {
	var req bulkGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if req.Count <= 0 || req.Count > maxPromoBatchCount {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(fmt.Sprintf("count must be between 1 and %d", maxPromoBatchCount)))
		return
	}
	if req.Prefix == "" {
		req.Prefix = "PROMO"
	}

	created := make([]PromoCodeResponse, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		code := req.Template
		code.Code = generatePromoCode(req.Prefix)

		if err := validatePromoCodeInput(&code); err != nil {
			writeAPIError(w, apierror.ValidationFailed.WithDetails(err.Error()))
			return
		}

		data, err := json.Marshal(code)
		if err != nil {
			writeAPIError(w, apierror.Internal)
			return
		}

		doc, err := h.collections.InsertDocument(r.Context(), PluginSlug, CollectionPromoCodes, json.RawMessage(data))
		if err != nil {
			h.logger.Error("failed to insert bulk promo code", slog.Any("error", err), slog.Int("index", i))
			continue
		}

		c, err := documentToPromoCode(doc)
		if err != nil {
			continue
		}
		created = append(created, *c)
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"created": len(created),
		"codes":   created,
	})
}

// --- Helpers ---

func validatePromoCodeInput(input *PromoCodeInput) error {
	if input.Code == "" {
		return fmt.Errorf("code is required")
	}
	if input.DiscountSpec.Type == "" {
		return fmt.Errorf("discount_spec.type is required")
	}
	validTypes := map[string]bool{"percent_off": true, "fixed_off": true}
	if !validTypes[input.DiscountSpec.Type] {
		return fmt.Errorf("discount_spec.type must be percent_off or fixed_off")
	}
	if input.DiscountSpec.Value <= 0 {
		return fmt.Errorf("discount_spec.value must be > 0")
	}
	if input.DiscountSpec.Type == "percent_off" && input.DiscountSpec.Value > money.PercentBase {
		return fmt.Errorf("discount_spec.value for percent_off must be <= %d (100%%)", money.PercentBase)
	}
	return nil
}

func documentToPromoCode(doc *pluginstore.Document) (*PromoCodeResponse, error) {
	var c PromoCodeResponse
	if err := json.Unmarshal(doc.Data, &c); err != nil {
		return nil, fmt.Errorf("unmarshal promo code: %w", err)
	}
	c.ID = doc.ID
	c.CreatedAt = doc.CreatedAt.Format(time.RFC3339)
	c.UpdatedAt = doc.UpdatedAt.Format(time.RFC3339)
	return &c, nil
}

// generatePromoCode creates a code like "PREFIX-A1B2C3D4E5F6A7B8" (64-bit entropy).
func generatePromoCode(prefix string) string {
	b := make([]byte, 8) // 64 bits of entropy
	_, _ = rand.Read(b)
	suffix := strings.ToUpper(hex.EncodeToString(b))
	// Format as XXXX-XXXX-XXXX-XXXX for readability
	return prefix + "-" + suffix[:4] + "-" + suffix[4:8] + "-" + suffix[8:12] + "-" + suffix[12:]
}
