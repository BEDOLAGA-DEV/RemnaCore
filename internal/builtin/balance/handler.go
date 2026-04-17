package balance

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
)

// Collection names for balance plugin documents.
const (
	CollectionWallets = "wallets"
	CollectionLedger  = "ledger"
	CollectionTopUps  = "topups"
)

// Config key names stored in plugin.Config map.
const (
	ConfigKeyMinTopUpCents   = "min_topup_cents"
	ConfigKeyMaxTopUpCents   = "max_topup_cents"
	ConfigKeyCurrency        = "currency"
	ConfigKeyTopUpPresets     = "topup_presets"
	ConfigKeyAllowedProviders = "allowed_providers"

	defaultMinTopUpCents = 100
	defaultMaxTopUpCents = 1_000_000
	defaultCurrency      = "USD"
	defaultLimit         = 50
	maxLimit             = 200
)

// Handler provides HTTP endpoints for user-facing balance operations.
type Handler struct {
	collections pluginstore.Store
	pluginRepo  plugin.PluginRepository
	logger      *slog.Logger
}

// NewHandler creates a balance Handler.
func NewHandler(collections pluginstore.Store, pluginRepo plugin.PluginRepository, logger *slog.Logger) *Handler {
	return &Handler{
		collections: collections,
		pluginRepo:  pluginRepo,
		logger:      logger,
	}
}

// loadConfig reads the balance plugin config from the repository.
func (h *Handler) loadConfig(r *http.Request) map[string]string {
	p, err := h.pluginRepo.GetBySlug(r.Context(), PluginSlug)
	if err != nil {
		return map[string]string{}
	}
	if p.Config == nil {
		return map[string]string{}
	}
	return p.Config
}

func configInt64(cfg map[string]string, key string, fallback int64) int64 {
	raw, ok := cfg[key]
	if !ok || raw == "" {
		return fallback
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return v
}

func configString(cfg map[string]string, key, fallback string) string {
	v, ok := cfg[key]
	if !ok || v == "" {
		return fallback
	}
	return v
}

func configStringSlice(cfg map[string]string, key string) []string {
	raw, ok := cfg[key]
	if !ok || raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func configInt64Slice(cfg map[string]string, key string) []int64 {
	raw, ok := cfg[key]
	if !ok || raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		v, err := strconv.ParseInt(p, 10, 64)
		if err == nil {
			out = append(out, v)
		}
	}
	return out
}

func parseLimit(r *http.Request) int {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return defaultLimit
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return defaultLimit
	}
	if v > maxLimit {
		return maxLimit
	}
	return v
}

// userID extracts the authenticated user identifier from the request.
// Uses only the trusted X-User-ID header set by auth middleware.
// Never trusts query parameters for user identity.
func userID(r *http.Request) string {
	return r.Header.Get("X-User-ID")
}

// =====================================================================
// GetMyBalance — GET /api/balance/me
// =====================================================================

func (h *Handler) GetMyBalance(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	if uid == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("user_id is required"))
		return
	}

	cfg := h.loadConfig(r)
	currency := configString(cfg, ConfigKeyCurrency, defaultCurrency)

	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionWallets)
	if err != nil {
		h.logger.Error("failed to list wallets", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	wallets := make([]WalletInfo, 0, 2)
	for _, doc := range docs {
		var w Wallet
		if json.Unmarshal(doc.Data, &w) != nil {
			continue
		}
		if w.UserID != uid {
			continue
		}
		wallets = append(wallets, WalletInfo{
			Kind:           w.Kind,
			BalanceCents:   w.BalanceCents,
			HoldCents:      w.HoldCents,
			AvailableCents: w.AvailableCents,
		})
	}

	// Ensure both wallet kinds are present, even with zero balance.
	hasKind := func(k WalletKind) bool {
		for _, wi := range wallets {
			if wi.Kind == k {
				return true
			}
		}
		return false
	}
	for _, k := range []WalletKind{WalletMoney, WalletBonus} {
		if !hasKind(k) {
			wallets = append(wallets, WalletInfo{Kind: k})
		}
	}

	presets := configInt64Slice(cfg, ConfigKeyTopUpPresets)
	if presets == nil {
		presets = []int64{}
	}
	providers := configStringSlice(cfg, ConfigKeyAllowedProviders)
	if providers == nil {
		providers = []string{}
	}

	writeJSON(w, http.StatusOK, BalanceResponse{
		Currency:       currency,
		Wallets:        wallets,
		TopUpPresets:   presets,
		TopUpProviders: providers,
		CanTopUp:       len(providers) > 0,
	})
}

// =====================================================================
// ListMyTransactions — GET /api/balance/me/transactions
// =====================================================================

func (h *Handler) ListMyTransactions(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	if uid == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("user_id is required"))
		return
	}

	limit := parseLimit(r)
	cursor := r.URL.Query().Get("cursor")

	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionLedger)
	if err != nil {
		h.logger.Error("failed to list ledger entries", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	entries := make([]LedgerEntry, 0, len(docs))
	pastCursor := cursor == ""
	for _, doc := range docs {
		if !pastCursor {
			if doc.ID == cursor {
				pastCursor = true
			}
			continue
		}
		var entry LedgerEntry
		if json.Unmarshal(doc.Data, &entry) != nil {
			continue
		}
		if entry.UserID != uid {
			continue
		}
		entry.ID = doc.ID
		entries = append(entries, entry)
		if len(entries) >= limit {
			break
		}
	}

	var nextCursor string
	if len(entries) == limit {
		nextCursor = entries[len(entries)-1].ID
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"entries":     entries,
		"next_cursor": nextCursor,
	})
}

// =====================================================================
// CreateTopUp — POST /api/balance/topup
// =====================================================================

func (h *Handler) CreateTopUp(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	if uid == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("user_id is required"))
		return
	}

	var req TopUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	cfg := h.loadConfig(r)
	minCents := configInt64(cfg, ConfigKeyMinTopUpCents, defaultMinTopUpCents)
	maxCents := configInt64(cfg, ConfigKeyMaxTopUpCents, defaultMaxTopUpCents)
	currency := configString(cfg, ConfigKeyCurrency, defaultCurrency)

	if req.AmountCents < minCents {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(
			fmt.Sprintf("minimum top-up amount is %d cents", minCents)))
		return
	}
	if req.AmountCents > maxCents {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(
			fmt.Sprintf("maximum top-up amount is %d cents", maxCents)))
		return
	}

	if req.PaymentMethod == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("payment_method is required"))
		return
	}

	now := time.Now().UTC()
	topup := TopUp{
		ID:            uuid.New().String(),
		UserID:        uid,
		AmountCents:   req.AmountCents,
		Currency:      currency,
		Status:        TopUpPending,
		PaymentMethod: req.PaymentMethod,
		CreatedAt:     now,
	}

	data, err := json.Marshal(topup)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	doc, err := h.collections.InsertDocument(r.Context(), PluginSlug, CollectionTopUps, json.RawMessage(data))
	if err != nil {
		h.logger.Error("failed to create topup", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	topup.ID = doc.ID

	writeJSON(w, http.StatusCreated, map[string]any{
		"topup":       topup,
		"payment_url": fmt.Sprintf("https://pay.example.com/mock/%s", topup.ID),
	})
}

// =====================================================================
// GetTopUp — GET /api/balance/topups/{id}
// =====================================================================

func (h *Handler) GetTopUp(w http.ResponseWriter, r *http.Request) {
	topupID := chi.URLParam(r, "id")
	if topupID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("topup ID is required"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionTopUps, topupID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	var topup TopUp
	if err := json.Unmarshal(doc.Data, &topup); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	topup.ID = doc.ID

	// Ownership check
	uid := userID(r)
	if uid != "" && topup.UserID != "" && topup.UserID != uid {
		writeAPIError(w, apierror.NotFound)
		return
	}

	writeJSON(w, http.StatusOK, topup)
}

// =====================================================================
// CancelTopUp — POST /api/balance/topups/{id}/cancel
// =====================================================================

func (h *Handler) CancelTopUp(w http.ResponseWriter, r *http.Request) {
	topupID := chi.URLParam(r, "id")
	if topupID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("topup ID is required"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionTopUps, topupID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	var topup TopUp
	if err := json.Unmarshal(doc.Data, &topup); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	// Ownership check
	uid := userID(r)
	if uid != "" && topup.UserID != "" && topup.UserID != uid {
		writeAPIError(w, apierror.NotFound)
		return
	}

	if topup.Status != TopUpPending {
		writeAPIError(w, apierror.ValidationFailed.WithDetails(
			fmt.Sprintf("cannot cancel topup with status %q, must be %q", topup.Status, TopUpPending)))
		return
	}

	topup.Status = TopUpCancelled

	data, err := json.Marshal(topup)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	if err := h.collections.UpdateDocument(r.Context(), PluginSlug, CollectionTopUps, topupID, json.RawMessage(data)); err != nil {
		h.logger.Error("failed to cancel topup", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	topup.ID = doc.ID
	writeJSON(w, http.StatusOK, topup)
}
