package balance

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// ExpiryWorker finds and expires bonus entries past their expiry date.
// Designed to run as a daily cron job.
type ExpiryWorker struct {
	service *BalanceService
	store   *BalanceService // same service, used for reads
	logger  *slog.Logger
}

// NewExpiryWorker creates an ExpiryWorker.
func NewExpiryWorker(service *BalanceService, logger *slog.Logger) *ExpiryWorker {
	return &ExpiryWorker{service: service, store: service, logger: logger}
}

// Run processes all expired bonus entries. Should be called once per day.
// Each expired entry generates a compensating entry that zeroes out the
// original amount. Idempotent via reference_id = "expire_" + original_id.
func (w *ExpiryWorker) Run(ctx context.Context) (int, error) {
	docs, err := w.service.store.ListDocuments(ctx, PluginSlug, CollectionLedger)
	if err != nil {
		return 0, fmt.Errorf("list ledger entries: %w", err)
	}

	now := time.Now()
	expired := 0

	// Build set of already-expired reference IDs for idempotency check
	expiredRefs := make(map[string]bool)
	for _, doc := range docs {
		var e LedgerEntry
		if json.Unmarshal(doc.Data, &e) == nil && e.Kind == EntryExpire {
			expiredRefs[e.ReferenceID] = true
		}
	}

	for _, doc := range docs {
		var entry LedgerEntry
		if json.Unmarshal(doc.Data, &entry) != nil {
			continue
		}

		// Only expire bonus entries with a past expiry date
		if entry.Wallet != WalletBonus {
			continue
		}
		if entry.ExpiresAt == nil || entry.ExpiresAt.After(now) {
			continue
		}
		if entry.AmountCents <= 0 {
			continue // already a debit or compensating entry
		}

		// Check idempotency — don't expire the same entry twice
		ref := "expire_" + doc.ID
		if expiredRefs[ref] {
			continue
		}

		// Create compensating entry
		compensating := LedgerEntry{
			UserID:      entry.UserID,
			Wallet:      WalletBonus,
			AmountCents: -entry.AmountCents,
			Currency:    entry.Currency,
			Kind:        EntryExpire,
			SourceType:  "expire",
			SourceID:    doc.ID,
			ReferenceID: ref,
			Description: fmt.Sprintf("Bonus expired (originally %d cents from %s)",
				entry.AmountCents, entry.Kind),
		}

		if _, err := w.service.CreateEntry(ctx, compensating); err != nil {
			w.logger.Warn("failed to expire bonus entry",
				slog.String("entry_id", doc.ID),
				slog.Any("error", err))
			continue
		}

		expired++
		w.logger.Info("bonus expired",
			slog.String("user_id", entry.UserID),
			slog.Int64("amount_cents", entry.AmountCents),
			slog.String("original_entry", doc.ID))
	}

	return expired, nil
}
