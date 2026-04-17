// Package balance implements the user-balance built-in plugin.
// service.go contains the core business logic for balance operations.
// Currently uses pluginstore for persistence; Phase 2+ will migrate
// to dedicated PG tables (migration 035) for transactional safety.
package balance

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
)

// BalanceService provides the core balance operations.
type BalanceService struct {
	store pluginstore.Store
}

// NewBalanceService creates a BalanceService.
func NewBalanceService(store pluginstore.Store) *BalanceService {
	return &BalanceService{store: store}
}

// GetWallets returns both wallets for a user.
func (s *BalanceService) GetWallets(ctx context.Context, userID string) ([]Wallet, error) {
	docs, err := s.store.ListDocuments(ctx, PluginSlug, CollectionWallets)
	if err != nil {
		return nil, err
	}

	wallets := make([]Wallet, 0, 2)
	for _, doc := range docs {
		var w Wallet
		if json.Unmarshal(doc.Data, &w) == nil && w.UserID == userID {
			w.AvailableCents = w.BalanceCents - w.HoldCents
			wallets = append(wallets, w)
		}
	}
	return wallets, nil
}

// GetAvailableBalance returns the available balance for a specific wallet.
func (s *BalanceService) GetAvailableBalance(ctx context.Context, userID string, wallet WalletKind) (int64, error) {
	wallets, err := s.GetWallets(ctx, userID)
	if err != nil {
		return 0, err
	}
	for _, w := range wallets {
		if w.Kind == wallet {
			return w.AvailableCents, nil
		}
	}
	return 0, nil
}

// CreateEntry creates a new ledger entry and updates the wallet balance.
// This is the atomic unit of all balance operations.
func (s *BalanceService) CreateEntry(ctx context.Context, entry LedgerEntry) (*LedgerEntry, error) {
	if entry.AmountCents == 0 {
		return nil, fmt.Errorf("amount_cents must be non-zero")
	}
	if entry.ReferenceID == "" {
		return nil, fmt.Errorf("reference_id is required for idempotency")
	}

	// Check idempotency — scan for existing entry with same reference
	docs, err := s.store.ListDocuments(ctx, PluginSlug, CollectionLedger)
	if err != nil {
		return nil, err
	}
	for _, doc := range docs {
		var existing LedgerEntry
		if json.Unmarshal(doc.Data, &existing) == nil &&
			existing.UserID == entry.UserID &&
			existing.Wallet == entry.Wallet &&
			existing.SourceType == entry.SourceType &&
			existing.ReferenceID == entry.ReferenceID {
			return &existing, nil // idempotent return
		}
	}

	// For debits, check sufficient balance
	if entry.AmountCents < 0 {
		available, err := s.GetAvailableBalance(ctx, entry.UserID, entry.Wallet)
		if err != nil {
			return nil, err
		}
		if available+entry.AmountCents < 0 {
			return nil, fmt.Errorf("insufficient balance: available=%d, requested=%d", available, -entry.AmountCents)
		}
	}

	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	if entry.CreatedBy == "" {
		entry.CreatedBy = "system"
	}

	// Insert ledger entry
	data, err := json.Marshal(entry)
	if err != nil {
		return nil, err
	}
	doc, err := s.store.InsertDocument(ctx, PluginSlug, CollectionLedger, json.RawMessage(data))
	if err != nil {
		return nil, err
	}
	entry.ID = doc.ID

	// Update wallet snapshot
	if err := s.updateWalletBalance(ctx, entry.UserID, entry.Wallet, entry.AmountCents, entry.Currency); err != nil {
		return nil, err
	}

	// Set balance_after
	available, _ := s.GetAvailableBalance(ctx, entry.UserID, entry.Wallet)
	entry.BalanceAfter = available

	return &entry, nil
}

// Spend deducts from a wallet (consumption at checkout).
func (s *BalanceService) Spend(ctx context.Context, userID string, wallet WalletKind, amountCents int64, currency, invoiceID string) (*LedgerEntry, error) {
	return s.CreateEntry(ctx, LedgerEntry{
		UserID:      userID,
		Wallet:      wallet,
		AmountCents: -amountCents,
		Currency:    currency,
		Kind:        EntryConsumption,
		SourceType:  "invoice",
		SourceID:    invoiceID,
		ReferenceID: "consumption_" + invoiceID,
		Description: fmt.Sprintf("Payment for invoice %s", invoiceID),
	})
}

// Hold places a hold on funds (blocks them from being used elsewhere).
// Creates a negative ledger entry and increases hold_cents on the wallet.
// The balance_cents is reduced by the hold amount; on release, a positive
// compensating entry restores it.
func (s *BalanceService) Hold(ctx context.Context, userID string, wallet WalletKind, amountCents int64, currency, referenceID string) (*LedgerEntry, error) {
	available, err := s.GetAvailableBalance(ctx, userID, wallet)
	if err != nil {
		return nil, err
	}
	if available < amountCents {
		return nil, fmt.Errorf("insufficient available balance for hold: available=%d, requested=%d", available, amountCents)
	}

	// Update hold tracking on wallet
	if err := s.updateWalletHold(ctx, userID, wallet, amountCents); err != nil {
		return nil, err
	}

	// Create negative hold entry (satisfies CHECK amount_cents <> 0)
	return s.CreateEntry(ctx, LedgerEntry{
		UserID:      userID,
		Wallet:      wallet,
		AmountCents: -amountCents,
		Currency:    currency,
		Kind:        EntryHold,
		SourceType:  "hold",
		ReferenceID: "hold_" + referenceID,
		Description: fmt.Sprintf("Hold %d cents for %s", amountCents, referenceID),
	})
}

// ReleaseHold releases a previously placed hold by creating a positive
// compensating entry and reducing hold_cents on the wallet.
func (s *BalanceService) ReleaseHold(ctx context.Context, userID string, wallet WalletKind, amountCents int64, currency, referenceID string) (*LedgerEntry, error) {
	if err := s.updateWalletHold(ctx, userID, wallet, -amountCents); err != nil {
		return nil, err
	}

	return s.CreateEntry(ctx, LedgerEntry{
		UserID:      userID,
		Wallet:      wallet,
		AmountCents: amountCents,
		Currency:    currency,
		Kind:        EntryRelease,
		SourceType:  "hold",
		ReferenceID: "release_" + referenceID,
		Description: fmt.Sprintf("Release hold of %d cents for %s", amountCents, referenceID),
	})
}

// updateWalletBalance updates or creates a wallet document.
func (s *BalanceService) updateWalletBalance(ctx context.Context, userID string, wallet WalletKind, deltaCents int64, currency string) error {
	docs, err := s.store.ListDocuments(ctx, PluginSlug, CollectionWallets)
	if err != nil {
		return err
	}

	for _, doc := range docs {
		var w Wallet
		if json.Unmarshal(doc.Data, &w) == nil && w.UserID == userID && w.Kind == wallet {
			w.BalanceCents += deltaCents
			w.AvailableCents = w.BalanceCents - w.HoldCents
			w.UpdatedAt = time.Now()
			data, _ := json.Marshal(w)
			return s.store.UpdateDocument(ctx, PluginSlug, CollectionWallets, doc.ID, json.RawMessage(data))
		}
	}

	// Create new wallet
	w := Wallet{
		UserID:         userID,
		Kind:           wallet,
		Currency:       currency,
		BalanceCents:   deltaCents,
		HoldCents:      0,
		AvailableCents: deltaCents,
		UpdatedAt:      time.Now(),
	}
	data, _ := json.Marshal(w)
	_, err = s.store.InsertDocument(ctx, PluginSlug, CollectionWallets, json.RawMessage(data))
	return err
}

// updateWalletHold adjusts the hold amount on a wallet.
func (s *BalanceService) updateWalletHold(ctx context.Context, userID string, wallet WalletKind, deltaCents int64) error {
	docs, err := s.store.ListDocuments(ctx, PluginSlug, CollectionWallets)
	if err != nil {
		return err
	}

	for _, doc := range docs {
		var w Wallet
		if json.Unmarshal(doc.Data, &w) == nil && w.UserID == userID && w.Kind == wallet {
			w.HoldCents += deltaCents
			if w.HoldCents < 0 {
				w.HoldCents = 0
			}
			w.AvailableCents = w.BalanceCents - w.HoldCents
			w.UpdatedAt = time.Now()
			data, _ := json.Marshal(w)
			return s.store.UpdateDocument(ctx, PluginSlug, CollectionWallets, doc.ID, json.RawMessage(data))
		}
	}

	return fmt.Errorf("wallet not found for user %s kind %s", userID, wallet)
}
