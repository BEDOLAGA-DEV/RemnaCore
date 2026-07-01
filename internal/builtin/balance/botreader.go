package balance

import (
	"context"
	"fmt"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
)

// walletGetter is the narrow callable surface the adapter requires from
// BalanceService. *BalanceService satisfies this; the narrow type is declared
// here so tests can inject a stub without needing the full service.
type walletGetter interface {
	GetWallets(ctx context.Context, userID string) ([]Wallet, error)
}

// BalanceReaderAdapter adapts BalanceService to the bothost.BalanceReader
// interface, mapping internal Wallet values to the serializable bothost.Wallet
// view safe to pass across the WASM boundary.
//
// Tenant scoping is the caller's responsibility: the ctx passed to each method
// must carry the tenant GUC (set via RunInTx + WithTenantID); this adapter
// must NOT set the GUC itself.
//
// Placement rationale: this adapter lives in package balance (not in
// internal/telegram) because internal/builtin/balance/plugin.go imports
// internal/gateway, which imports internal/telegram. Placing the adapter in
// internal/telegram would create a telegram→balance→gateway→telegram cycle.
type BalanceReaderAdapter struct {
	svc walletGetter
}

// NewBalanceReaderAdapter returns a bothost.BalanceReader backed by svc.
// In production, pass the *BalanceService directly. Returning the interface
// type (matching the sibling adapter constructors) is what the fx wiring binds.
func NewBalanceReaderAdapter(svc *BalanceService) bothost.BalanceReader {
	return &BalanceReaderAdapter{svc: svc}
}

// WalletsByUser implements bothost.BalanceReader.
func (a *BalanceReaderAdapter) WalletsByUser(ctx context.Context, userID string) ([]bothost.Wallet, error) {
	wallets, err := a.svc.GetWallets(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("balance.wallets_by_user: %w", err)
	}
	views := make([]bothost.Wallet, 0, len(wallets))
	for _, w := range wallets {
		views = append(views, bothost.Wallet{
			Kind:      string(w.Kind),
			Currency:  w.Currency,
			Balance:   w.BalanceCents,
			Available: w.AvailableCents,
		})
	}
	return views, nil
}

// Compile-time assertion: BalanceReaderAdapter must implement bothost.BalanceReader.
var _ bothost.BalanceReader = (*BalanceReaderAdapter)(nil)
