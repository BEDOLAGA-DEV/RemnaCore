package tariff

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
)

// --- Cohort DTOs ---

// CohortInput is the request body for creating or updating a cohort.
type CohortInput struct {
	Name            string          `json:"name"`
	Selector        CohortSelector  `json:"selector"`
	PricingOverride PricingOverride `json:"pricing_override"`
	UserCount       int             `json:"user_count"`
}

// CohortSelector defines how users are selected into a cohort.
type CohortSelector struct {
	Type  string       `json:"type"` // "event_based" | "attribute_based" | "manual"
	Rules []CohortRule `json:"rules"`
}

// CohortRule is a single selection rule within a CohortSelector.
type CohortRule struct {
	Event      string `json:"event,omitempty"` // e.g. "SubCancelled"
	WithinDays int    `json:"within_days,omitempty"`
	UserAttr   string `json:"user_attr,omitempty"` // e.g. "lifetime_months"
	Gte        int    `json:"gte,omitempty"`
	Lte        int    `json:"lte,omitempty"`
	Eq         string `json:"eq,omitempty"`
}

// PricingOverride links a cohort to a specific pricing rule or auto-applied
// promo code.
type PricingOverride struct {
	PricingRuleID string `json:"pricing_rule_id,omitempty"`
	PromoCodeAuto string `json:"promo_code_auto,omitempty"` // auto-apply promo
}

// CohortResponse wraps CohortInput with server-assigned identity and
// timestamps.
type CohortResponse struct {
	ID string `json:"id"`
	CohortInput
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// --- Cohort Handlers ---

// ListCohorts returns all cohorts in the CollectionCohorts collection.
func (h *Handler) ListCohorts(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionCohorts)
	if err != nil {
		h.logger.Error("failed to list cohorts", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	cohorts := make([]CohortResponse, 0, len(docs))
	for _, doc := range docs {
		c, convErr := documentToCohort(&doc)
		if convErr != nil {
			continue
		}
		cohorts = append(cohorts, *c)
	}

	writeJSON(w, http.StatusOK, cohorts)
}

// GetCohort returns a single cohort by its ID.
func (h *Handler) GetCohort(w http.ResponseWriter, r *http.Request) {
	cohortID := chi.URLParam(r, "cohortID")
	if cohortID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("cohort ID is required"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionCohorts, cohortID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	c, err := documentToCohort(doc)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, c)
}

// CreateCohort decodes, validates, and inserts a new cohort.
func (h *Handler) CreateCohort(w http.ResponseWriter, r *http.Request) {
	var input CohortInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := validateCohortInput(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(err.Error()))
		return
	}

	data, err := json.Marshal(input)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	doc, err := h.collections.InsertDocument(r.Context(), PluginSlug, CollectionCohorts, json.RawMessage(data))
	if err != nil {
		h.logger.Error("failed to create cohort", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	c, err := documentToCohort(doc)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusCreated, c)
}

// UpdateCohort updates an existing cohort by ID.
func (h *Handler) UpdateCohort(w http.ResponseWriter, r *http.Request) {
	cohortID := chi.URLParam(r, "cohortID")
	if cohortID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("cohort ID is required"))
		return
	}

	var input CohortInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := validateCohortInput(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(err.Error()))
		return
	}

	data, err := json.Marshal(input)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	if err := h.collections.UpdateDocument(r.Context(), PluginSlug, CollectionCohorts, cohortID, json.RawMessage(data)); err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionCohorts, cohortID)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	c, err := documentToCohort(doc)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, c)
}

// DeleteCohort removes a cohort by ID.
func (h *Handler) DeleteCohort(w http.ResponseWriter, r *http.Request) {
	cohortID := chi.URLParam(r, "cohortID")
	if cohortID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("cohort ID is required"))
		return
	}

	if err := h.collections.DeleteDocument(r.Context(), PluginSlug, CollectionCohorts, cohortID); err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetCohortUsers returns a placeholder response for the cohort's user
// membership. Actual cohort membership is recalculated asynchronously.
func (h *Handler) GetCohortUsers(w http.ResponseWriter, r *http.Request) {
	cohortID := chi.URLParam(r, "cohortID")
	if cohortID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("cohort ID is required"))
		return
	}

	// Verify the cohort exists.
	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionCohorts, cohortID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	c, err := documentToCohort(doc)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"cohort_id":  c.ID,
		"user_count": c.UserCount,
		"note":       "cohort membership is recalculated asynchronously",
	})
}

// --- Cohort Helpers ---

func validateCohortInput(input *CohortInput) error {
	if input.Name == "" {
		return fmt.Errorf("name is required")
	}
	validTypes := map[string]bool{
		"event_based": true, "attribute_based": true, "manual": true,
	}
	if input.Selector.Type == "" {
		return fmt.Errorf("selector.type is required")
	}
	if !validTypes[input.Selector.Type] {
		return fmt.Errorf("unsupported selector.type: %s", input.Selector.Type)
	}
	if input.Selector.Type != "manual" && len(input.Selector.Rules) == 0 {
		return fmt.Errorf("selector.rules required for type %s", input.Selector.Type)
	}
	return nil
}

func documentToCohort(doc *pluginstore.Document) (*CohortResponse, error) {
	var c CohortResponse
	if err := json.Unmarshal(doc.Data, &c); err != nil {
		return nil, fmt.Errorf("unmarshal cohort: %w", err)
	}
	c.ID = doc.ID
	c.CreatedAt = doc.CreatedAt.Format(time.RFC3339)
	c.UpdatedAt = doc.UpdatedAt.Format(time.RFC3339)
	return &c, nil
}
