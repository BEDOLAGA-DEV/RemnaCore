package balance

import (
	"context"
	"encoding/json"
	"fmt"
)

// PaymentChargeRequest matches the payment.create_charge hook payload.
type PaymentChargeRequest struct {
	InvoiceID     string `json:"invoice_id"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	UserID        string `json:"user_id"`
	UserEmail     string `json:"user_email"`
	PlanName      string `json:"plan_name"`
	PaymentMethod string `json:"payment_method"` // "balance", "balance_money", "balance_bonus"
	ReturnURL     string `json:"return_url"`
}

// PaymentChargeResult is returned when balance successfully processes a charge.
type PaymentChargeResult struct {
	Provider   string `json:"provider"`
	ExternalID string `json:"external_id"` // ledger entry ID
	Status     string `json:"status"`      // "completed" (instant)
}

// PaymentHandler processes payment.create_charge hooks for balance payments.
// When payment_method is "balance", "balance_money", or "balance_bonus",
// it deducts from the user's wallet instead of redirecting to an external provider.
type PaymentHandler struct {
	service *BalanceService
}

// NewPaymentHandler creates a PaymentHandler.
func NewPaymentHandler(service *BalanceService) *PaymentHandler {
	return &PaymentHandler{service: service}
}

// HandleCreateCharge processes a balance payment. Returns nil if payment_method
// is not a balance type (so other plugins can handle it).
func (h *PaymentHandler) HandleCreateCharge(ctx context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var req PaymentChargeRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("unmarshal charge request: %w", err)
	}

	// Only handle balance payment methods
	wallet, ok := resolveWalletKind(req.PaymentMethod)
	if !ok {
		return nil, nil // not a balance payment, let other plugins handle
	}

	// Deduct from wallet
	entry, err := h.service.Spend(ctx, req.UserID, wallet, req.Amount, req.Currency, req.InvoiceID)
	if err != nil {
		return nil, fmt.Errorf("balance deduction failed: %w", err)
	}

	result := PaymentChargeResult{
		Provider:   "balance",
		ExternalID: entry.ID,
		Status:     "completed", // balance payments are instant
	}

	return json.Marshal(result)
}

// resolveWalletKind maps payment_method strings to wallet kinds.
func resolveWalletKind(method string) (WalletKind, bool) {
	switch method {
	case "balance", "balance_money":
		return WalletMoney, true
	case "balance_bonus":
		return WalletBonus, true
	default:
		return "", false
	}
}
