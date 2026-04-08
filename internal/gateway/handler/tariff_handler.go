package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
)

// tariffPluginSlug is the built-in plugin slug that owns tariff data.
const tariffPluginSlug = "tariff-manager"

// tariffCollection is the collection name within the tariff-manager plugin
// where tariff documents are stored.
const tariffCollection = "tariffs"

// priceCurrencyLen is the required length for ISO 4217 currency codes.
const priceCurrencyLen = 3

// TariffHandler provides dedicated CRUD endpoints for tariff management.
// Tariffs are stored as documents in the tariff-manager plugin's collection
// but exposed through a first-class API with domain-level validation.
type TariffHandler struct {
	collections pluginstore.Store
	logger      *slog.Logger
}

// NewTariffHandler creates a TariffHandler backed by the given collection
// store.
func NewTariffHandler(collections pluginstore.Store, logger *slog.Logger) *TariffHandler {
	return &TariffHandler{
		collections: collections,
		logger:      logger,
	}
}

// --- Request / Response DTOs ---

// TariffInput is the request body for creating or updating a tariff.
type TariffInput struct {
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	PriceAmount         int64    `json:"price_amount"`
	PriceCurrency       string   `json:"price_currency"`
	DurationDays        int      `json:"duration_days"`
	TrafficLimitGB      float64  `json:"traffic_limit_gb"`
	DeviceLimit         int      `json:"device_limit"`
	MaxPurchasesPerUser int      `json:"max_purchases_per_user"`
	InternalSquadUUIDs  []string `json:"internal_squad_uuids"`
	ExternalSquadUUIDs  []string `json:"external_squad_uuids"`
	Features            []string `json:"features"`
	IsActive            bool     `json:"is_active"`
	SortOrder           int      `json:"sort_order"`
}

// TariffResponse is the API representation of a tariff document.
type TariffResponse struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	PriceAmount         int64    `json:"price_amount"`
	PriceCurrency       string   `json:"price_currency"`
	DurationDays        int      `json:"duration_days"`
	TrafficLimitGB      float64  `json:"traffic_limit_gb"`
	DeviceLimit         int      `json:"device_limit"`
	MaxPurchasesPerUser int      `json:"max_purchases_per_user"`
	InternalSquadUUIDs  []string `json:"internal_squad_uuids"`
	ExternalSquadUUIDs  []string `json:"external_squad_uuids"`
	Features            []string `json:"features"`
	IsActive            bool     `json:"is_active"`
	SortOrder           int      `json:"sort_order"`
	CreatedAt           string   `json:"created_at"`
	UpdatedAt           string   `json:"updated_at"`
}

// --- Handlers ---

// ListTariffs handles GET /api/tariffs — list tariff documents.
// Accepts optional query param "active=true" to filter to active tariffs only.
// Results are sorted by sort_order ascending.
func (h *TariffHandler) ListTariffs(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), tariffPluginSlug, tariffCollection)
	if err != nil {
		h.logger.Error("failed to list tariffs", slog.Any("error", err))
		writeJSON(w, http.StatusOK, []TariffResponse{})
		return
	}

	activeOnly := r.URL.Query().Get("active") == "true"

	tariffs := make([]TariffResponse, 0, len(docs))
	for _, doc := range docs {
		t, convErr := documentToTariff(&doc)
		if convErr != nil {
			h.logger.Warn("skipping malformed tariff document",
				slog.String("doc_id", doc.ID),
				slog.Any("error", convErr),
			)
			continue
		}
		if activeOnly && !t.IsActive {
			continue
		}
		tariffs = append(tariffs, *t)
	}

	slices.SortFunc(tariffs, func(a, b TariffResponse) int {
		return a.SortOrder - b.SortOrder
	})

	writeJSON(w, http.StatusOK, tariffs)
}

// GetTariff handles GET /api/tariffs/{tariffID} — get a single tariff by ID.
func (h *TariffHandler) GetTariff(w http.ResponseWriter, r *http.Request) {
	tariffID := chi.URLParam(r, "tariffID")
	if tariffID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("tariff ID is required"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), tariffPluginSlug, tariffCollection, tariffID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get tariff")
		return
	}

	t, err := documentToTariff(doc)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse tariff document")
		return
	}

	writeJSON(w, http.StatusOK, t)
}

// CreateTariff handles POST /api/tariffs — create a new tariff (admin only).
func (h *TariffHandler) CreateTariff(w http.ResponseWriter, r *http.Request) {
	var input TariffInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeValidationError(w, err)
		return
	}

	if err := validateTariffInput(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(err.Error()))
		return
	}

	data, err := json.Marshal(input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal tariff")
		return
	}

	doc, err := h.collections.InsertDocument(r.Context(), tariffPluginSlug, tariffCollection, json.RawMessage(data))
	if err != nil {
		h.logger.Error("failed to create tariff", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to create tariff")
		return
	}

	t, err := documentToTariff(doc)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse created tariff")
		return
	}

	writeJSON(w, http.StatusCreated, t)
}

// UpdateTariff handles PUT /api/tariffs/{tariffID} — update a tariff (admin only).
func (h *TariffHandler) UpdateTariff(w http.ResponseWriter, r *http.Request) {
	tariffID := chi.URLParam(r, "tariffID")
	if tariffID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("tariff ID is required"))
		return
	}

	var input TariffInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeValidationError(w, err)
		return
	}

	if err := validateTariffInput(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(err.Error()))
		return
	}

	data, err := json.Marshal(input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal tariff")
		return
	}

	if err := h.collections.UpdateDocument(r.Context(), tariffPluginSlug, tariffCollection, tariffID, json.RawMessage(data)); err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		h.logger.Error("failed to update tariff",
			slog.String("tariff_id", tariffID),
			slog.Any("error", err),
		)
		writeError(w, http.StatusInternalServerError, "failed to update tariff")
		return
	}

	// Re-fetch the updated document to return the full response with timestamps.
	doc, err := h.collections.GetDocument(r.Context(), tariffPluginSlug, tariffCollection, tariffID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve updated tariff")
		return
	}

	t, err := documentToTariff(doc)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse updated tariff")
		return
	}

	writeJSON(w, http.StatusOK, t)
}

// DeleteTariff handles DELETE /api/tariffs/{tariffID} — delete a tariff (admin only).
func (h *TariffHandler) DeleteTariff(w http.ResponseWriter, r *http.Request) {
	tariffID := chi.URLParam(r, "tariffID")
	if tariffID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("tariff ID is required"))
		return
	}

	if err := h.collections.DeleteDocument(r.Context(), tariffPluginSlug, tariffCollection, tariffID); err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		h.logger.Error("failed to delete tariff",
			slog.String("tariff_id", tariffID),
			slog.Any("error", err),
		)
		writeError(w, http.StatusInternalServerError, "failed to delete tariff")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Helpers ---

// validateTariffInput checks required fields and sets nil slice defaults to
// empty slices for consistent JSON serialisation.
func validateTariffInput(input *TariffInput) error {
	if input.Name == "" {
		return fmt.Errorf("name is required")
	}
	if input.PriceAmount < 0 {
		return fmt.Errorf("price_amount must be >= 0")
	}
	if input.DurationDays <= 0 {
		return fmt.Errorf("duration_days must be > 0")
	}
	if len(input.PriceCurrency) != priceCurrencyLen {
		return fmt.Errorf("price_currency must be %d characters", priceCurrencyLen)
	}

	// Ensure nil slices become empty slices for consistent JSON output.
	if input.InternalSquadUUIDs == nil {
		input.InternalSquadUUIDs = []string{}
	}
	if input.ExternalSquadUUIDs == nil {
		input.ExternalSquadUUIDs = []string{}
	}
	if input.Features == nil {
		input.Features = []string{}
	}
	return nil
}

// documentToTariff converts a collection Document into a TariffResponse.
// The document's Data field is expected to contain a JSON-encoded TariffInput.
func documentToTariff(doc *pluginstore.Document) (*TariffResponse, error) {
	var tariff TariffResponse
	if err := json.Unmarshal(doc.Data, &tariff); err != nil {
		return nil, fmt.Errorf("unmarshal tariff document: %w", err)
	}
	tariff.ID = doc.ID
	tariff.CreatedAt = doc.CreatedAt.Format(time.RFC3339)
	tariff.UpdatedAt = doc.UpdatedAt.Format(time.RFC3339)

	// Ensure nil slices become empty slices for consistent JSON output.
	if tariff.InternalSquadUUIDs == nil {
		tariff.InternalSquadUUIDs = []string{}
	}
	if tariff.ExternalSquadUUIDs == nil {
		tariff.ExternalSquadUUIDs = []string{}
	}
	if tariff.Features == nil {
		tariff.Features = []string{}
	}
	return &tariff, nil
}
