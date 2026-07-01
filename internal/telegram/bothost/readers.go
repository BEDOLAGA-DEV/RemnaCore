package bothost

import "context"

// Subscription is the bot-plugin view of a billing subscription. It is
// serializable and safe to pass across the WASM boundary unchanged.
// UserID is intentionally absent from this view; the ownerUserID return value
// on SubscriptionReader.Get provides it for op-level ownership checks without
// leaking it into the serialized payload.
type Subscription struct {
	ID        string `json:"id"`
	PlanID    string `json:"plan_id"`
	Status    string `json:"status"`
	ExpiresAt string `json:"expires_at,omitempty"` // RFC3339; empty when open-ended
}

// SubscriptionReader is the read-only subscription surface exposed to bot
// plugins via the OpContext.
//
// Tenant scoping is the caller's responsibility: the ctx passed to each method
// must carry the tenant GUC (set via RunInTx + WithTenantID). This interface
// and its adapters must NOT set the GUC themselves.
type SubscriptionReader interface {
	// ActiveByUser returns all active subscriptions for the given platform user.
	ActiveByUser(ctx context.Context, userID string) ([]Subscription, error)

	// Get resolves a subscription by ID, returning the view alongside the
	// owning platform user ID so that ops can enforce ownership without leaking
	// UserID into the serialized Subscription view.
	Get(ctx context.Context, id string) (sub *Subscription, ownerUserID string, err error)
}

// Invoice is the bot-plugin view of a billing invoice.
type Invoice struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

// InvoiceReader is the read-only invoice surface exposed to bot plugins via
// the OpContext.
//
// Tenant scoping is the caller's responsibility: the ctx passed to each method
// must carry the tenant GUC (set via RunInTx + WithTenantID). This interface
// and its adapters must NOT set the GUC themselves.
type InvoiceReader interface {
	// PendingByUser returns all unpaid invoices for the given platform user.
	PendingByUser(ctx context.Context, userID string) ([]Invoice, error)
}

// Wallet is the bot-plugin view of a single user balance wallet.
type Wallet struct {
	Kind      string `json:"kind"`
	Currency  string `json:"currency"`
	Balance   int64  `json:"balance"`
	Available int64  `json:"available"`
}

// BalanceReader is the read-only balance surface exposed to bot plugins via
// the OpContext.
//
// Tenant scoping is the caller's responsibility: the ctx passed to each method
// must carry the tenant GUC (set via RunInTx + WithTenantID). This interface
// and its adapters must NOT set the GUC themselves.
//
// IMPORTANT — import-cycle note: the concrete adapter for this interface lives
// in internal/builtin/balance (package balance), NOT in internal/telegram.
// This is required because internal/builtin/balance/plugin.go imports
// internal/gateway, and internal/gateway imports internal/telegram; placing the
// adapter in internal/telegram would create a telegram→balance→gateway→telegram
// import cycle.
type BalanceReader interface {
	// WalletsByUser returns all wallets for the given platform user.
	WalletsByUser(ctx context.Context, userID string) ([]Wallet, error)
}

// CheckoutInput carries the parameters for starting a checkout flow from a bot
// plugin.
type CheckoutInput struct {
	UserID      string
	PlanID      string
	UserEmail   string
	UserCountry string
}

// CheckoutResult holds the outcome of a started checkout flow.
type CheckoutResult struct {
	CheckoutURL    string `json:"checkout_url"`
	SubscriptionID string `json:"subscription_id"`
	InvoiceID      string `json:"invoice_id"`
	Provider       string `json:"provider"`
}

// CheckoutStarter is the checkout surface exposed to bot plugins via the
// OpContext.
//
// Tenant scoping is the caller's responsibility: the ctx passed to Start must
// carry the tenant GUC via RunInTx + WithTenantID.
//
// Note on transactions: the underlying CheckoutService.StartCheckout
// self-wraps its own RunInTx internally. Callers must NOT wrap Start inside
// another RunInTx — doing so would create a nested transaction that the GUC
// isolation model does not support.
type CheckoutStarter interface {
	// Start creates a subscription and invoice, then initiates a payment charge.
	// Returns the checkout URL and identifiers for the caller to redirect the user.
	Start(ctx context.Context, in CheckoutInput) (CheckoutResult, error)
}
