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
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
)

const (
	// PluginSlug is the canonical slug for the tariff-manager plugin.
	PluginSlug = "tariff-manager"

	// CollectionName is the plugin collection where tariffs are stored.
	CollectionName = "tariffs"

	priceCurrencyLen = 3
)

// Handler provides HTTP endpoints for tariff CRUD and Remnawave data lookups.
type Handler struct {
	collections pluginstore.Store
	pluginRepo  plugin.PluginRepository
	logger      *slog.Logger
}

// NewHandler creates a tariff Handler.
func NewHandler(collections pluginstore.Store, pluginRepo plugin.PluginRepository, logger *slog.Logger) *Handler {
	return &Handler{
		collections: collections,
		pluginRepo:  pluginRepo,
		logger:      logger,
	}
}

// remnawaveClient creates a temporary client from the remnawave-provider
// plugin config stored in the database. Returns nil if the plugin is
// not enabled — callers must check before use.
func (h *Handler) remnawaveClient(ctx context.Context) *remnawave.Client {
	p, err := h.pluginRepo.GetBySlug(ctx, plugin.BuiltInSlugRemnawaveProvider)
	if err != nil {
		return nil
	}
	if p.Status != plugin.StatusEnabled {
		return nil
	}
	return remnawave.NewClient(
		p.Config[plugin.RemnawaveConfigKeyURL],
		p.Config[plugin.RemnawaveConfigKeyAPIToken],
	)
}

// --- Request / Response DTOs ---

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

func (h *Handler) ListTariffs(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionName)
	if err != nil {
		h.logger.Error("failed to list tariffs", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	activeOnly := r.URL.Query().Get("active") == "true"

	tariffs := make([]TariffResponse, 0, len(docs))
	for _, doc := range docs {
		t, convErr := documentToTariff(&doc)
		if convErr != nil {
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
	var input TariffInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := validateTariffInput(&input); err != nil {
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

	writeJSON(w, http.StatusCreated, t)
}

func (h *Handler) UpdateTariff(w http.ResponseWriter, r *http.Request) {
	tariffID := chi.URLParam(r, "tariffID")
	if tariffID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("tariff ID is required"))
		return
	}

	var input TariffInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if err := validateTariffInput(&input); err != nil {
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

	writeJSON(w, http.StatusOK, t)
}

func (h *Handler) DeleteTariff(w http.ResponseWriter, r *http.Request) {
	tariffID := chi.URLParam(r, "tariffID")
	if tariffID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("tariff ID is required"))
		return
	}

	if err := h.collections.DeleteDocument(r.Context(), PluginSlug, CollectionName, tariffID); err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListInternalSquads(w http.ResponseWriter, r *http.Request) {
	client := h.remnawaveClient(r.Context())
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
	client := h.remnawaveClient(r.Context())
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
	client := h.remnawaveClient(r.Context())
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

// --- Helpers ---

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

func documentToTariff(doc *pluginstore.Document) (*TariffResponse, error) {
	var t TariffResponse
	if err := json.Unmarshal(doc.Data, &t); err != nil {
		return nil, fmt.Errorf("unmarshal tariff document: %w", err)
	}
	t.ID = doc.ID
	t.CreatedAt = doc.CreatedAt.Format(time.RFC3339)
	t.UpdatedAt = doc.UpdatedAt.Format(time.RFC3339)
	if t.InternalSquadUUIDs == nil {
		t.InternalSquadUUIDs = []string{}
	}
	if t.ExternalSquadUUIDs == nil {
		t.ExternalSquadUUIDs = []string{}
	}
	if t.Features == nil {
		t.Features = []string{}
	}
	return &t, nil
}
