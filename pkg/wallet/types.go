// Package wallet provides shared types for cross-plugin balance integration.
package wallet

import "time"

// Balance represents a user's complete wallet state.
type Balance struct {
	Currency    string     `json:"currency"`
	MoneyWallet WalletInfo `json:"money_wallet"`
	BonusWallet WalletInfo `json:"bonus_wallet"`
}

// WalletInfo holds balance details for a single wallet.
type WalletInfo struct {
	BalanceCents   int64          `json:"balance_cents"`
	HoldCents      int64          `json:"hold_cents"`
	AvailableCents int64          `json:"available_cents"`
	ExpiringSoon   []ExpiringEntry `json:"expiring_soon,omitempty"`
}

// ExpiringEntry represents a bonus amount that will expire soon.
type ExpiringEntry struct {
	AmountCents int64     `json:"amount_cents"`
	ExpiresAt   time.Time `json:"expires_at"`
}
