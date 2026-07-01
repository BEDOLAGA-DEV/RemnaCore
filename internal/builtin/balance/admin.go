package balance

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
)

// Collection constants defined in handler.go: CollectionWallets, CollectionLedger, CollectionTopUps

// compactTimestampLayout is the Go time layout for the compact timestamp suffix
// used in ledger reference IDs (e.g. adjust_20060102150405).
const compactTimestampLayout = "20060102150405"

func (h *Handler) AdminListWallets(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionWallets)
	if err != nil {
		h.logger.Error("failed to list wallets", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}
	wallets := make([]json.RawMessage, 0, len(docs))
	for _, doc := range docs {
		wallets = append(wallets, doc.Data)
	}
	writeJSON(w, http.StatusOK, map[string]any{"wallets": wallets, "total": len(wallets)})
}

func (h *Handler) AdminGetUserBalance(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	if userID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("userID is required"))
		return
	}
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionWallets)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	userWallets := make([]json.RawMessage, 0)
	for _, doc := range docs {
		var wt Wallet
		if json.Unmarshal(doc.Data, &wt) == nil && wt.UserID == userID {
			userWallets = append(userWallets, doc.Data)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"user_id": userID, "wallets": userWallets})
}

func (h *Handler) AdminAdjust(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	if userID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("userID is required"))
		return
	}
	var req AdjustRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}
	if req.AmountCents == 0 {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("amount_cents must be non-zero"))
		return
	}
	if !isValidWalletKind(req.WalletKind) {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("wallet_kind must be 'money' or 'bonus'"))
		return
	}
	entry := LedgerEntry{
		UserID: userID, Wallet: WalletKind(req.WalletKind), AmountCents: req.AmountCents,
		Kind: EntryAdjust, SourceType: "admin_adjust",
		ReferenceID: "adjust_" + time.Now().Format(compactTimestampLayout),
		Description: req.Description, CreatedAt: time.Now(), CreatedBy: "admin",
	}
	data, err := json.Marshal(entry)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	doc, err := h.collections.InsertDocument(r.Context(), PluginSlug, CollectionLedger, json.RawMessage(data))
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"entry_id": doc.ID, "amount_cents": req.AmountCents})
}

func (h *Handler) AdminTransfer(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	if userID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("userID is required"))
		return
	}
	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("invalid request body"))
		return
	}
	if req.AmountCents <= 0 {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("amount_cents must be positive"))
		return
	}
	if !isValidWalletKind(req.FromWallet) || !isValidWalletKind(req.ToWallet) {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("from_wallet and to_wallet must be 'money' or 'bonus'"))
		return
	}
	if req.FromWallet == req.ToWallet {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("from_wallet and to_wallet must be different"))
		return
	}
	now := time.Now()
	ref := "transfer_" + now.Format(compactTimestampLayout)
	debit := LedgerEntry{
		UserID: userID, Wallet: WalletKind(req.FromWallet), AmountCents: -req.AmountCents,
		Kind: EntryTransfer, SourceType: "transfer", ReferenceID: ref + "_debit",
		Description: req.Description, CreatedAt: now, CreatedBy: "admin",
	}
	debitData, err := json.Marshal(debit)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	if _, err := h.collections.InsertDocument(r.Context(), PluginSlug, CollectionLedger, json.RawMessage(debitData)); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	credit := LedgerEntry{
		UserID: userID, Wallet: WalletKind(req.ToWallet), AmountCents: req.AmountCents,
		Kind: EntryTransfer, SourceType: "transfer", ReferenceID: ref + "_credit",
		Description: req.Description, CreatedAt: now, CreatedBy: "admin",
	}
	creditData, err := json.Marshal(credit)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	if _, err := h.collections.InsertDocument(r.Context(), PluginSlug, CollectionLedger, json.RawMessage(creditData)); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"action": "transferred", "amount": req.AmountCents})
}

func (h *Handler) AdminUserLedger(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionLedger)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	entries := make([]json.RawMessage, 0)
	for _, doc := range docs {
		var e LedgerEntry
		if json.Unmarshal(doc.Data, &e) == nil && e.UserID == userID {
			entries = append(entries, doc.Data)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"user_id": userID, "entries": entries})
}

func (h *Handler) AdminLedger(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionLedger)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	entries := make([]json.RawMessage, 0, len(docs))
	for _, doc := range docs {
		entries = append(entries, doc.Data)
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries, "total": len(entries)})
}

func (h *Handler) AdminListTopUps(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionTopUps)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	topups := make([]json.RawMessage, 0, len(docs))
	for _, doc := range docs {
		topups = append(topups, doc.Data)
	}
	writeJSON(w, http.StatusOK, map[string]any{"topups": topups, "total": len(topups)})
}

func (h *Handler) AdminApproveTopUp(w http.ResponseWriter, r *http.Request) {
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
	if topup.Status != TopUpPending && topup.Status != TopUpProcessing {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("topup is not pending"))
		return
	}
	now := time.Now()
	topup.Status = TopUpCompleted
	topup.CompletedAt = &now

	// Credit the user's money wallet via ledger entry
	creditEntry := LedgerEntry{
		UserID:      topup.UserID,
		Wallet:      WalletMoney,
		AmountCents: topup.AmountCents,
		Currency:    topup.Currency,
		Kind:        EntryTopUp,
		SourceType:  "topup",
		SourceID:    topupID,
		ReferenceID: "topup_" + topupID,
		Description: "Top-up approved by admin",
		CreatedAt:   now,
		CreatedBy:   "admin",
	}
	creditData, err := json.Marshal(creditEntry)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	if _, err := h.collections.InsertDocument(r.Context(), PluginSlug, CollectionLedger, json.RawMessage(creditData)); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	// Update topup status
	data, err := json.Marshal(topup)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	if err := h.collections.UpdateDocument(r.Context(), PluginSlug, CollectionTopUps, topupID, json.RawMessage(data)); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"topup_id":     topupID,
		"status":       "completed",
		"amount_cents": topup.AmountCents,
		"wallet":       "money",
	})
}

func (h *Handler) AdminRejectTopUp(w http.ResponseWriter, r *http.Request) {
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
	topup.Status = TopUpFailed
	data, err := json.Marshal(topup)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	if err := h.collections.UpdateDocument(r.Context(), PluginSlug, CollectionTopUps, topupID, json.RawMessage(data)); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"topup_id": topupID, "status": "rejected"})
}

func (h *Handler) AdminAnalytics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"total_money_balance":  0,
		"total_bonus_balance":  0,
		"monthly_topup_volume": 0,
		"active_wallets":       0,
	})
}

func (h *Handler) AdminExportCSV(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionLedger)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	entries := make([]json.RawMessage, 0, len(docs))
	for _, doc := range docs {
		entries = append(entries, doc.Data)
	}
	w.Header().Set("Content-Disposition", "attachment; filename=ledger_export.json")
	writeJSON(w, http.StatusOK, entries)
}

// isValidWalletKind checks if a wallet kind string is valid.
func isValidWalletKind(kind string) bool {
	return kind == string(WalletMoney) || kind == string(WalletBonus)
}
