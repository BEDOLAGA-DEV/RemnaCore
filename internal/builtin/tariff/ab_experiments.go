package tariff

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
)

// --- A/B Experiment DTOs ---

// ABExperimentInput is the request body for creating or updating an A/B experiment.
type ABExperimentInput struct {
	Name               string               `json:"name"`
	TariffID           string               `json:"tariff_id"`
	Status             string               `json:"status"` // "draft" | "running" | "paused" | "completed" | "archived"
	Variants           []ABVariant          `json:"variants"`
	AssignmentStrategy string               `json:"assignment_strategy"` // "hash_user_id" | "hash_tenant_id" | "random"
	StartedAt          string               `json:"started_at,omitempty"`
	EndedAt            string               `json:"ended_at,omitempty"`
	Metrics            map[string]ABMetrics `json:"metrics,omitempty"`
	StatSignificance   float64              `json:"stat_significance"`
	Winner             string               `json:"winner,omitempty"`
	StopRule           string               `json:"stop_rule"`
}

// ABVariant is a single price variant in an experiment.
type ABVariant struct {
	Key        string `json:"key"`
	PriceCents int64  `json:"price_cents"`
	TrafficPct int    `json:"traffic_pct"` // basis points, total must = 10000
}

// ABMetrics tracks conversion data for a single variant.
type ABMetrics struct {
	Exposures    int   `json:"exposures"`
	Conversions  int   `json:"conversions"`
	RevenueCents int64 `json:"revenue_cents"`
}

// ABExperimentResponse wraps ABExperimentInput with server-assigned fields.
type ABExperimentResponse struct {
	ID string `json:"id"`
	ABExperimentInput
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// --- Deterministic Assignment ---

// assignVariant deterministically assigns a user to a variant using FNV hash.
// The same user+experiment always gets the same variant, without storing
// assignments in the database.
func assignVariant(userID, experimentID string, variants []ABVariant) ABVariant {
	h := fnv.New64a()
	_, _ = h.Write([]byte(userID + experimentID))
	bucket := h.Sum64() % 10000
	cumulative := uint64(0)
	for _, v := range variants {
		cumulative += uint64(v.TrafficPct)
		if bucket < cumulative {
			return v
		}
	}
	return variants[len(variants)-1]
}

// --- Handlers ---

// ListABExperiments returns all A/B experiments.
func (h *Handler) ListABExperiments(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionABExperiments)
	if err != nil {
		h.logger.Error("failed to list ab experiments", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	experiments := make([]ABExperimentResponse, 0, len(docs))
	for _, doc := range docs {
		e, convErr := documentToABExperiment(&doc)
		if convErr != nil {
			continue
		}
		experiments = append(experiments, *e)
	}

	writeJSON(w, http.StatusOK, experiments)
}

// CreateABExperiment creates a new A/B experiment.
func (h *Handler) CreateABExperiment(w http.ResponseWriter, r *http.Request) {
	var input ABExperimentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := validateABExperimentInput(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(err.Error()))
		return
	}

	data, err := json.Marshal(input)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	doc, err := h.collections.InsertDocument(r.Context(), PluginSlug, CollectionABExperiments, json.RawMessage(data))
	if err != nil {
		h.logger.Error("failed to create ab experiment", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	e, err := documentToABExperiment(doc)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusCreated, e)
}

// StartABExperiment transitions an experiment from draft/paused to running.
func (h *Handler) StartABExperiment(w http.ResponseWriter, r *http.Request) {
	expID := chi.URLParam(r, "experimentID")
	if expID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("experiment ID is required"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionABExperiments, expID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	var input ABExperimentInput
	if err := json.Unmarshal(doc.Data, &input); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	if input.Status != "draft" && input.Status != "paused" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(
			fmt.Sprintf("cannot start experiment in status %q (must be draft or paused)", input.Status)))
		return
	}

	input.Status = "running"
	if input.StartedAt == "" {
		input.StartedAt = time.Now().Format(time.RFC3339)
	}

	h.updateExperiment(w, r, expID, &input)
}

// PauseABExperiment transitions a running experiment to paused.
func (h *Handler) PauseABExperiment(w http.ResponseWriter, r *http.Request) {
	expID := chi.URLParam(r, "experimentID")
	if expID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("experiment ID is required"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionABExperiments, expID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	var input ABExperimentInput
	if err := json.Unmarshal(doc.Data, &input); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	if input.Status != "running" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(
			fmt.Sprintf("cannot pause experiment in status %q (must be running)", input.Status)))
		return
	}

	input.Status = "paused"
	h.updateExperiment(w, r, expID, &input)
}

// concludeRequest is the body for concluding an experiment.
type concludeRequest struct {
	Winner string `json:"winner"`
}

// ConcludeABExperiment marks an experiment as completed with a winner.
func (h *Handler) ConcludeABExperiment(w http.ResponseWriter, r *http.Request) {
	expID := chi.URLParam(r, "experimentID")
	if expID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("experiment ID is required"))
		return
	}

	var req concludeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionABExperiments, expID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	var input ABExperimentInput
	if err := json.Unmarshal(doc.Data, &input); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	if input.Status != "running" && input.Status != "paused" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(
			fmt.Sprintf("cannot conclude experiment in status %q", input.Status)))
		return
	}

	// Validate that winner matches a known variant key.
	if req.Winner != "" {
		validKey := false
		for _, v := range input.Variants {
			if v.Key == req.Winner {
				validKey = true
				break
			}
		}
		if !validKey {
			writeAPIError(w, apierror.ValidationFailed.WithDetails(
				fmt.Sprintf("winner %q is not a valid variant key", req.Winner)))
			return
		}
	}

	input.Status = "completed"
	input.Winner = req.Winner
	input.EndedAt = time.Now().Format(time.RFC3339)

	h.updateExperiment(w, r, expID, &input)
}

// GetABExperimentResults returns experiment data + variant assignment for a test user.
func (h *Handler) GetABExperimentResults(w http.ResponseWriter, r *http.Request) {
	expID := chi.URLParam(r, "experimentID")
	if expID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("experiment ID is required"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionABExperiments, expID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	e, err := documentToABExperiment(doc)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	result := map[string]any{
		"experiment": e,
	}

	userID := r.URL.Query().Get("user_id")
	if userID != "" && len(e.Variants) > 0 {
		assigned := assignVariant(userID, expID, e.Variants)
		result["user_assignment"] = map[string]any{
			"user_id":     userID,
			"variant":     assigned.Key,
			"price_cents": assigned.PriceCents,
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// updateExperiment is a helper that saves the experiment and returns the updated response.
func (h *Handler) updateExperiment(w http.ResponseWriter, r *http.Request, expID string, input *ABExperimentInput) {
	data, err := json.Marshal(input)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	if err := h.collections.UpdateDocument(r.Context(), PluginSlug, CollectionABExperiments, expID, json.RawMessage(data)); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionABExperiments, expID)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	e, err := documentToABExperiment(doc)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, e)
}

// --- Helpers ---

func validateABExperimentInput(input *ABExperimentInput) error {
	if input.Name == "" {
		return fmt.Errorf("name is required")
	}
	if input.TariffID == "" {
		return fmt.Errorf("tariff_id is required")
	}
	if len(input.Variants) < 2 {
		return fmt.Errorf("at least 2 variants required")
	}
	totalPct := 0
	keys := make(map[string]bool)
	for i, v := range input.Variants {
		if v.Key == "" {
			return fmt.Errorf("variants[%d].key is required", i)
		}
		if keys[v.Key] {
			return fmt.Errorf("duplicate variant key: %s", v.Key)
		}
		keys[v.Key] = true
		if v.TrafficPct <= 0 {
			return fmt.Errorf("variants[%d].traffic_pct must be > 0", i)
		}
		totalPct += v.TrafficPct
	}
	if totalPct != 10000 {
		return fmt.Errorf("variant traffic_pct must sum to 10000, got %d", totalPct)
	}
	if input.AssignmentStrategy == "" {
		input.AssignmentStrategy = "hash_user_id"
	}
	validStrategies := map[string]bool{
		"hash_user_id": true, "hash_tenant_id": true, "random": true,
	}
	if !validStrategies[input.AssignmentStrategy] {
		return fmt.Errorf("unsupported assignment_strategy: %s", input.AssignmentStrategy)
	}
	if input.Status == "" {
		input.Status = "draft"
	}
	return nil
}

func documentToABExperiment(doc *pluginstore.Document) (*ABExperimentResponse, error) {
	var e ABExperimentResponse
	if err := json.Unmarshal(doc.Data, &e); err != nil {
		return nil, fmt.Errorf("unmarshal ab experiment: %w", err)
	}
	e.ID = doc.ID
	e.CreatedAt = doc.CreatedAt.Format(time.RFC3339)
	e.UpdatedAt = doc.UpdatedAt.Format(time.RFC3339)
	return &e, nil
}
