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

const (
	collectionWallets = "wallets"
	collectionLedger  = "ledger"
	collectionTopUps  = "topups"
)

func (h *Handler) AdminListWallets(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, collectionWallets)
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
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, collectionWallets)
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
	entry := LedgerEntry{
		UserID: userID, Wallet: WalletKind(req.WalletKind), AmountCents: req.AmountCents,
		Kind: EntryAdjust, SourceType: "admin_adjust",
		ReferenceID: "adjust_" + time.Now().Format("20060102150405"),
		Description: req.Description, CreatedAt: time.Now(), CreatedBy: "admin",
	}
	data, err := json.Marshal(entry)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	doc, err := h.collections.InsertDocument(r.Context(), PluginSlug, collectionLedger, json.RawMessage(data))
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
	now := time.Now()
	ref := "transfer_" + now.Format("20060102150405")
	debit := LedgerEntry{
		UserID: userID, Wallet: WalletKind(req.FromWallet), AmountCents: -req.AmountCents,
		Kind: EntryTransfer, SourceType: "transfer", ReferenceID: ref + "_debit",
		Description: req.Description, CreatedAt: now, CreatedBy: "admin",
	}
	debitData, _ := json.Marshal(debit)
	h.collections.InsertDocument(r.Context(), PluginSlug, collectionLedger, json.RawMessage(debitData))

	credit := LedgerEntry{
		UserID: userID, Wallet: WalletKind(req.ToWallet), AmountCents: req.AmountCents,
		Kind: EntryTransfer, SourceType: "transfer", ReferenceID: ref + "_credit",
		Description: req.Description, CreatedAt: now, CreatedBy: "admin",
	}
	creditData, _ := json.Marshal(credit)
	h.collections.InsertDocument(r.Context(), PluginSlug, collectionLedger, json.RawMessage(creditData))

	writeJSON(w, http.StatusOK, map[string]any{"action": "transferred", "amount": req.AmountCents})
}

func (h *Handler) AdminUserLedger(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, collectionLedger)
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
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, collectionLedger)
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
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, collectionTopUps)
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
	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, collectionTopUps, topupID)
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
	data, err := json.Marshal(topup)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	if err := h.collections.UpdateDocument(r.Context(), PluginSlug, collectionTopUps, topupID, json.RawMessage(data)); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"topup_id": topupID, "status": "completed"})
}

func (h *Handler) AdminRejectTopUp(w http.ResponseWriter, r *http.Request) {
	topupID := chi.URLParam(r, "id")
	if topupID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("topup ID is required"))
		return
	}
	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, collectionTopUps, topupID)
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
	if err := h.collections.UpdateDocument(r.Context(), PluginSlug, collectionTopUps, topupID, json.RawMessage(data)); err != nil {
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
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, collectionLedger)
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
