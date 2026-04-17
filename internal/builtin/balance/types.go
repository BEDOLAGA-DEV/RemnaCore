package balance

import "time"

// WalletKind distinguishes between real money and bonus credits.
type WalletKind string

const (
	WalletMoney WalletKind = "money" // real money, refundable
	WalletBonus WalletKind = "bonus" // cashback/promo, non-refundable, expiring
)

// EntryKind categorizes ledger movements.
type EntryKind string

const (
	EntryTopUp       EntryKind = "topup"
	EntryConsumption EntryKind = "consumption"
	EntryCashback    EntryKind = "cashback"
	EntryRefund      EntryKind = "refund"
	EntryExpire      EntryKind = "expire"
	EntryAdjust      EntryKind = "adjust"
	EntryTransfer    EntryKind = "transfer"
	EntryHold        EntryKind = "hold"
	EntryRelease     EntryKind = "release"
)

// TopUpStatus tracks the state of a top-up request.
type TopUpStatus string

const (
	TopUpPending    TopUpStatus = "pending"
	TopUpProcessing TopUpStatus = "processing"
	TopUpCompleted  TopUpStatus = "completed"
	TopUpFailed     TopUpStatus = "failed"
	TopUpCancelled  TopUpStatus = "cancelled"
)

// PluginSlug is the canonical slug for the balance plugin.
const PluginSlug = "user-balance"

// LedgerEntry represents a single immutable journal entry.
type LedgerEntry struct {
	ID           string            `json:"id"`
	UserID       string            `json:"user_id"`
	TenantID     string            `json:"tenant_id,omitempty"`
	Wallet       WalletKind        `json:"wallet"`
	AmountCents  int64             `json:"amount_cents"`
	Currency     string            `json:"currency"`
	Kind         EntryKind         `json:"kind"`
	SourceType   string            `json:"source_type"`
	SourceID     string            `json:"source_id,omitempty"`
	ReferenceID  string            `json:"reference_id"`
	Description  string            `json:"description"`
	BalanceAfter int64             `json:"balance_after"`
	ExpiresAt    *time.Time        `json:"expires_at,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	CreatedBy    string            `json:"created_by"`
}

// Wallet represents the current state of a user's wallet.
type Wallet struct {
	UserID         string     `json:"user_id"`
	TenantID       string     `json:"tenant_id,omitempty"`
	Kind           WalletKind `json:"kind"`
	Currency       string     `json:"currency"`
	BalanceCents   int64      `json:"balance_cents"`
	HoldCents      int64      `json:"hold_cents"`
	AvailableCents int64      `json:"available_cents"` // computed: balance - hold
	UpdatedAt      time.Time  `json:"updated_at"`
}

// TopUp represents a pending or completed top-up request.
type TopUp struct {
	ID            string      `json:"id"`
	UserID        string      `json:"user_id"`
	TenantID      string      `json:"tenant_id,omitempty"`
	AmountCents   int64       `json:"amount_cents"`
	Currency      string      `json:"currency"`
	Status        TopUpStatus `json:"status"`
	PaymentMethod string      `json:"payment_method"`
	PaymentID     string      `json:"payment_id,omitempty"`
	LedgerEntryID string      `json:"ledger_entry_id,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	CompletedAt   *time.Time  `json:"completed_at,omitempty"`
}

// --- Request/Response DTOs ---

type TopUpRequest struct {
	AmountCents   int64  `json:"amount_cents"`
	PaymentMethod string `json:"payment_method"`
	ReturnURL     string `json:"return_url"`
}

type AdjustRequest struct {
	WalletKind  string `json:"wallet_kind"`
	AmountCents int64  `json:"amount_cents"`
	Description string `json:"description"`
	ReasonCode  string `json:"reason_code"`
}

type TransferRequest struct {
	FromWallet  string `json:"from_wallet"`
	ToWallet    string `json:"to_wallet"`
	AmountCents int64  `json:"amount_cents"`
	Description string `json:"description"`
}

type BalanceResponse struct {
	Currency      string         `json:"currency"`
	Wallets       []WalletInfo   `json:"wallets"`
	TopUpPresets  []int64        `json:"topup_presets,omitempty"`
	TopUpProviders []string      `json:"topup_providers,omitempty"`
	CanTopUp      bool           `json:"can_topup"`
}

type WalletInfo struct {
	Kind           WalletKind `json:"kind"`
	BalanceCents   int64      `json:"balance_cents"`
	HoldCents      int64      `json:"hold_cents"`
	AvailableCents int64      `json:"available_cents"`
}
