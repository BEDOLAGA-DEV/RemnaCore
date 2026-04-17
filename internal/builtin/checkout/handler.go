package checkout

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
)

const (
	collectionSessions     = "sessions"
	collectionSavedMethods = "saved_methods"

	sessionTTL        = 30 * time.Minute
	defaultCurrency   = "RUB"
	defaultPriceCents int64 = 0
)

// Handler provides HTTP endpoints for the checkout built-in plugin.
type Handler struct {
	collections pluginstore.Store
	pluginRepo  plugin.PluginRepository
	logger      *slog.Logger
}

// NewHandler creates a checkout Handler.
func NewHandler(collections pluginstore.Store, pluginRepo plugin.PluginRepository, logger *slog.Logger) *Handler {
	return &Handler{
		collections: collections,
		pluginRepo:  pluginRepo,
		logger:      logger,
	}
}

// ---------------------------------------------------------------------------
// Session management
// ---------------------------------------------------------------------------

// CreateSession creates a new checkout session.
// POST /api/checkout/session
func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if req.TariffID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("tariff_id is required"))
		return
	}
	if req.Quantity < 1 {
		req.Quantity = 1
	}

	uid := r.Header.Get("X-User-ID")

	now := time.Now()
	session := CheckoutSession{
		ID:              uuid.NewString(),
		UserID:          uid,
		TariffID:        req.TariffID,
		Quantity:        req.Quantity,
		Currency:        defaultCurrency,
		FinalPriceCents: defaultPriceCents,
		PaymentMode:     PaymentSingle,
		Status:          SessionPending,
		ExpiresAt:       now.Add(sessionTTL),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	data, err := json.Marshal(session)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	doc, err := h.collections.InsertDocument(r.Context(), PluginSlug, collectionSessions, json.RawMessage(data))
	if err != nil {
		h.logger.Error("failed to create checkout session", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	// Overwrite generated ID with the store-assigned document ID.
	session.ID = doc.ID
	writeJSON(w, http.StatusCreated, session)
}

// GetSession returns a checkout session by ID.
// GET /api/checkout/{sessionID}
func (h *Handler) GetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	if sessionID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("session ID is required"))
		return
	}

	session, err := h.loadSession(w, r, sessionID)
	if err != nil {
		return // error already written
	}

	writeJSON(w, http.StatusOK, session)
}

// ListSources returns available payment sources for a checkout session.
// GET /api/checkout/{sessionID}/sources
func (h *Handler) ListSources(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	if sessionID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("session ID is required"))
		return
	}

	// Verify session exists.
	if _, err := h.loadSession(w, r, sessionID); err != nil {
		return
	}

	// Hardcoded sources -- in production this would query the balance plugin
	// and configured payment providers.
	sources := []AvailableSource{
		{
			Kind:           "balance_money",
			Label:          "Balance (money)",
			Currency:       defaultCurrency,
			MaxAmountCents: 0,
			Icon:           "wallet",
			Order:          1,
		},
		{
			Kind:           "balance_bonus",
			Label:          "Balance (bonus)",
			Currency:       defaultCurrency,
			MaxAmountCents: 0,
			Icon:           "gift",
			Order:          2,
		},
		{
			Kind:     "stripe",
			Label:    "Stripe",
			Currency: "USD",
			Icon:     "credit-card",
			Order:    3,
		},
		{
			Kind:     "yookassa",
			Label:    "YooKassa",
			Currency: defaultCurrency,
			Icon:     "credit-card",
			Order:    4,
		},
	}

	writeJSON(w, http.StatusOK, sources)
}

// ValidateSession checks whether a checkout session is still valid.
// POST /api/checkout/{sessionID}/validate
func (h *Handler) ValidateSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	if sessionID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("session ID is required"))
		return
	}

	session, err := h.loadSession(w, r, sessionID)
	if err != nil {
		return
	}

	valid := session.Status == SessionPending && time.Now().Before(session.ExpiresAt)

	reason := ""
	if session.Status != SessionPending {
		reason = "session status is " + string(session.Status)
	} else if !time.Now().Before(session.ExpiresAt) {
		reason = "session has expired"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": sessionID,
		"valid":      valid,
		"status":     session.Status,
		"expires_at": session.ExpiresAt,
		"reason":     reason,
	})
}

// ConfirmSession confirms a checkout session with selected payment sources.
// POST /api/checkout/{sessionID}/confirm
func (h *Handler) ConfirmSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	if sessionID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("session ID is required"))
		return
	}

	session, err := h.loadSession(w, r, sessionID)
	if err != nil {
		return
	}

	if session.Status != SessionPending {
		writeAPIError(w, apierror.BillingCheckoutBlocked.WithDetails("session is not pending"))
		return
	}
	if time.Now().After(session.ExpiresAt) {
		writeAPIError(w, apierror.BillingCheckoutBlocked.WithDetails("session has expired"))
		return
	}

	var req ConfirmSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}

	if len(req.Sources) == 0 {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("at least one payment source is required"))
		return
	}

	// Validate sum of sources equals final price.
	var totalCents int64
	for _, src := range req.Sources {
		totalCents += src.AmountCents
	}
	if session.FinalPriceCents > 0 && totalCents != session.FinalPriceCents {
		writeAPIError(w, apierror.BillingInsufficientFunds.WithDetails("sum of sources does not match final price"))
		return
	}

	// Build sub-invoices from sources.
	subInvoices := make([]SubInvoice, 0, len(req.Sources))
	for _, src := range req.Sources {
		subInvoices = append(subInvoices, SubInvoice{
			SubInvoiceID: uuid.NewString(),
			SourceKind:   src.Kind,
			AmountCents:  src.AmountCents,
			Status:       "pending",
		})
	}

	// Determine payment mode.
	mode := PaymentSingle
	if len(req.Sources) > 1 {
		mode = PaymentSplit
	}

	now := time.Now()
	session.Sources = req.Sources
	session.SubInvoices = subInvoices
	session.PaymentMode = mode
	session.Status = SessionProcessing
	session.UpdatedAt = now

	if req.PromoCode != "" {
		session.PromoCode = req.PromoCode
	}

	if err := h.saveSession(w, r, sessionID, session); err != nil {
		return
	}

	// Determine next_action based on sources.
	nextAction := "none"
	for _, si := range subInvoices {
		if si.SourceKind != "balance_money" && si.SourceKind != "balance_bonus" {
			nextAction = "redirect"
			break
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"session":      session,
		"sub_invoices": subInvoices,
		"next_action":  nextAction,
	})
}

// CancelSession cancels a checkout session.
// POST /api/checkout/{sessionID}/cancel
func (h *Handler) CancelSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	if sessionID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("session ID is required"))
		return
	}

	session, err := h.loadSession(w, r, sessionID)
	if err != nil {
		return
	}

	if session.Status != SessionPending && session.Status != SessionProcessing {
		writeAPIError(w, apierror.BillingCheckoutBlocked.WithDetails("session cannot be cancelled in status "+string(session.Status)))
		return
	}

	now := time.Now()
	session.Status = SessionCancelled
	session.UpdatedAt = now

	if err := h.saveSession(w, r, sessionID, session); err != nil {
		return
	}

	writeJSON(w, http.StatusOK, session)
}

// ApplyPromo applies a promo code to a checkout session.
// POST /api/checkout/{sessionID}/apply-promo
func (h *Handler) ApplyPromo(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	if sessionID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("session ID is required"))
		return
	}

	session, err := h.loadSession(w, r, sessionID)
	if err != nil {
		return
	}

	if session.Status != SessionPending {
		writeAPIError(w, apierror.BillingCheckoutBlocked.WithDetails("promo can only be applied to pending sessions"))
		return
	}

	var req ApplyPromoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}
	if req.PromoCode == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("promo_code is required"))
		return
	}

	// Store promo code on session. Actual validation would go through tariff plugin hook.
	session.PromoCode = req.PromoCode
	session.UpdatedAt = time.Now()

	if err := h.saveSession(w, r, sessionID, session); err != nil {
		return
	}

	writeJSON(w, http.StatusOK, session)
}

// ---------------------------------------------------------------------------
// Saved payment methods
// ---------------------------------------------------------------------------

// ListSavedMethods lists saved payment methods.
// GET /api/checkout/saved-methods
func (h *Handler) ListSavedMethods(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, collectionSavedMethods)
	if err != nil {
		h.logger.Error("failed to list saved methods", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	uid := r.Header.Get("X-User-ID")
	methods := make([]SavedMethod, 0, len(docs))
	for _, doc := range docs {
		var m SavedMethod
		if json.Unmarshal(doc.Data, &m) != nil {
			continue
		}
		// Filter by user — never show other users' saved methods
		if uid != "" && m.UserID != "" && m.UserID != uid {
			continue
		}
		m.ID = doc.ID
		m.TokenID = "" // never expose raw token to client
		methods = append(methods, m)
	}

	writeJSON(w, http.StatusOK, methods)
}

// SetDefaultMethod sets a saved payment method as default.
// POST /api/checkout/saved-methods/{id}/default
func (h *Handler) SetDefaultMethod(w http.ResponseWriter, r *http.Request) {
	methodID := chi.URLParam(r, "id")
	if methodID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("method ID is required"))
		return
	}

	// Load the target method.
	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, collectionSavedMethods, methodID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	var target SavedMethod
	if err := json.Unmarshal(doc.Data, &target); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	// Clear default flag on all other methods.
	allDocs, err := h.collections.ListDocuments(r.Context(), PluginSlug, collectionSavedMethods)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	for _, d := range allDocs {
		if d.ID == methodID {
			continue
		}
		var m SavedMethod
		if json.Unmarshal(d.Data, &m) != nil {
			continue
		}
		if m.IsDefault {
			m.IsDefault = false
			updated, _ := json.Marshal(m)
			_ = h.collections.UpdateDocument(r.Context(), PluginSlug, collectionSavedMethods, d.ID, json.RawMessage(updated))
		}
	}

	// Set the target as default.
	target.IsDefault = true
	data, err := json.Marshal(target)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	if err := h.collections.UpdateDocument(r.Context(), PluginSlug, collectionSavedMethods, methodID, json.RawMessage(data)); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, target)
}

// DeleteSavedMethod removes a saved payment method.
// DELETE /api/checkout/saved-methods/{id}
func (h *Handler) DeleteSavedMethod(w http.ResponseWriter, r *http.Request) {
	methodID := chi.URLParam(r, "id")
	if methodID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("method ID is required"))
		return
	}

	if err := h.collections.DeleteDocument(r.Context(), PluginSlug, collectionSavedMethods, methodID); err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		h.logger.Error("failed to delete saved method", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// loadSession fetches and unmarshals a session from the store. On error it
// writes the HTTP error response so callers can return immediately.
func (h *Handler) loadSession(w http.ResponseWriter, r *http.Request, sessionID string) (*CheckoutSession, error) {
	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, collectionSessions, sessionID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return nil, err
		}
		h.logger.Error("failed to get checkout session", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return nil, err
	}

	var session CheckoutSession
	if err := json.Unmarshal(doc.Data, &session); err != nil {
		h.logger.Error("failed to unmarshal checkout session", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return nil, err
	}
	session.ID = doc.ID

	// Ownership check — reject if session belongs to a different user
	uid := r.Header.Get("X-User-ID")
	if uid != "" && session.UserID != "" && session.UserID != uid {
		writeAPIError(w, apierror.NotFound)
		return nil, fmt.Errorf("session ownership mismatch")
	}

	return &session, nil
}

// saveSession marshals and persists a session. On error it writes the HTTP
// error response so callers can return immediately.
func (h *Handler) saveSession(w http.ResponseWriter, r *http.Request, sessionID string, session *CheckoutSession) error {
	data, err := json.Marshal(session)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return err
	}
	if err := h.collections.UpdateDocument(r.Context(), PluginSlug, collectionSessions, sessionID, json.RawMessage(data)); err != nil {
		h.logger.Error("failed to update checkout session", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return err
	}
	return nil
}
