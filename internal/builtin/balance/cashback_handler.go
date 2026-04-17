package balance

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// CashbackConfig holds cashback-related settings from plugin config.
type CashbackConfig struct {
	Enabled          bool
	PercentBP        int64  // basis points (500 = 5%)
	MinInvoiceCents  int64
	ExcludedPlanIDs  []string
	BonusExpiryDays  int
}

// CashbackHandler listens for invoice.paid events and awards cashback.
type CashbackHandler struct {
	service *BalanceService
	config  CashbackConfig
	logger  *slog.Logger
}

// NewCashbackHandler creates a CashbackHandler.
func NewCashbackHandler(service *BalanceService, config CashbackConfig, logger *slog.Logger) *CashbackHandler {
	return &CashbackHandler{service: service, config: config, logger: logger}
}

// InvoicePaidPayload is the expected shape of an invoice.paid event.
type InvoicePaidPayload struct {
	InvoiceID   string `json:"invoice_id"`
	UserID      string `json:"user_id"`
	AmountCents int64  `json:"amount_cents"`
	Currency    string `json:"currency"`
	PlanID      string `json:"plan_id"`
}

// OnInvoicePaid handles the invoice.paid event by creating a cashback entry.
func (h *CashbackHandler) OnInvoicePaid(ctx context.Context, evt InvoicePaidPayload) error {
	if !h.config.Enabled {
		return nil
	}

	if evt.AmountCents < h.config.MinInvoiceCents {
		h.logger.Debug("invoice below cashback threshold",
			slog.Int64("amount", evt.AmountCents),
			slog.Int64("min", h.config.MinInvoiceCents))
		return nil
	}

	if h.isExcludedPlan(evt.PlanID) {
		return nil
	}

	cashbackCents := evt.AmountCents * h.config.PercentBP / 10000
	if cashbackCents <= 0 {
		return nil
	}

	var expiresAt *time.Time
	if h.config.BonusExpiryDays > 0 {
		t := time.Now().Add(time.Duration(h.config.BonusExpiryDays) * 24 * time.Hour)
		expiresAt = &t
	}

	entry := LedgerEntry{
		UserID:      evt.UserID,
		Wallet:      WalletBonus,
		AmountCents: cashbackCents,
		Currency:    evt.Currency,
		Kind:        EntryCashback,
		SourceType:  "invoice",
		SourceID:    evt.InvoiceID,
		ReferenceID: "cashback_" + evt.InvoiceID, // idempotency
		ExpiresAt:   expiresAt,
		Description: fmt.Sprintf("%.1f%% cashback for invoice %s",
			float64(h.config.PercentBP)/100, evt.InvoiceID),
	}

	_, err := h.service.CreateEntry(ctx, entry)
	if err != nil {
		h.logger.Error("failed to create cashback entry",
			slog.Any("error", err),
			slog.String("invoice_id", evt.InvoiceID),
			slog.String("user_id", evt.UserID))
		return err
	}

	h.logger.Info("cashback awarded",
		slog.String("user_id", evt.UserID),
		slog.Int64("cashback_cents", cashbackCents),
		slog.String("invoice_id", evt.InvoiceID))

	return nil
}

// isExcludedPlan checks if a plan is excluded from cashback.
func (h *CashbackHandler) isExcludedPlan(planID string) bool {
	for _, excluded := range h.config.ExcludedPlanIDs {
		if strings.EqualFold(excluded, planID) {
			return true
		}
	}
	return false
}
