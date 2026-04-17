package remnawave

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

// ListRoutingRules returns all geo routing rules.
func (h *Handler) ListRoutingRules(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionGeoRoutingRules)
	if err != nil {
		h.logger.Error("failed to list routing rules", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	rules := make([]GeoRoutingRuleResponse, 0, len(docs))
	for _, doc := range docs {
		var rule GeoRoutingRuleInput
		if json.Unmarshal(doc.Data, &rule) != nil {
			continue
		}
		rules = append(rules, GeoRoutingRuleResponse{
			ID:                  doc.ID,
			GeoRoutingRuleInput: rule,
			CreatedAt:           doc.CreatedAt.Format(time.RFC3339),
			UpdatedAt:           doc.UpdatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, rules)
}

// CreateRoutingRule adds a new geo routing rule.
func (h *Handler) CreateRoutingRule(w http.ResponseWriter, r *http.Request) {
	var input GeoRoutingRuleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := validateGeoRoutingRuleInput(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(err.Error()))
		return
	}

	data, err := json.Marshal(input)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	doc, err := h.collections.InsertDocument(r.Context(), PluginSlug, CollectionGeoRoutingRules, json.RawMessage(data))
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusCreated, GeoRoutingRuleResponse{
		ID:                  doc.ID,
		GeoRoutingRuleInput: input,
		CreatedAt:           doc.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           doc.UpdatedAt.Format(time.RFC3339),
	})
}

// UpdateRoutingRule updates an existing routing rule.
func (h *Handler) UpdateRoutingRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "ruleID")
	if ruleID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("rule ID is required"))
		return
	}

	var input GeoRoutingRuleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := validateGeoRoutingRuleInput(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(err.Error()))
		return
	}

	data, err := json.Marshal(input)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	if err := h.collections.UpdateDocument(r.Context(), PluginSlug, CollectionGeoRoutingRules, ruleID, json.RawMessage(data)); err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// DeleteRoutingRule removes a routing rule.
func (h *Handler) DeleteRoutingRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "ruleID")
	if ruleID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("rule ID is required"))
		return
	}

	if err := h.collections.DeleteDocument(r.Context(), PluginSlug, CollectionGeoRoutingRules, ruleID); err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SimulateRouting determines which panel would be selected for a given country.
func (h *Handler) SimulateRouting(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CountryCode string `json:"country_code"`
		UserID      string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if req.CountryCode == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("country_code is required"))
		return
	}

	// Look up geo routing rules
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionGeoRoutingRules)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	var matchedPanelID string
	var matchedPriority int = -1

	for _, doc := range docs {
		var rule GeoRoutingRuleInput
		if json.Unmarshal(doc.Data, &rule) != nil {
			continue
		}
		if !rule.IsActive {
			continue
		}
		if rule.CountryCode == req.CountryCode {
			if matchedPriority < 0 || rule.Priority < matchedPriority {
				matchedPanelID = rule.PanelID
				matchedPriority = rule.Priority
			}
		}
	}

	if matchedPanelID == "" {
		// Fallback: first active panel
		panelDocs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionPanelConnections)
		if err == nil {
			for _, doc := range panelDocs {
				var p PanelConnectionInput
				if json.Unmarshal(doc.Data, &p) == nil && p.Status == PanelStatusActive {
					matchedPanelID = doc.ID
					break
				}
			}
		}
	}

	result := map[string]any{
		"country_code": req.CountryCode,
		"panel_id":     matchedPanelID,
		"strategy":     "geo",
	}

	if matchedPanelID == "" {
		result["strategy"] = "none"
		result["note"] = fmt.Sprintf("no routing rule for country %s, no active panels", req.CountryCode)
	}

	writeJSON(w, http.StatusOK, result)
}
