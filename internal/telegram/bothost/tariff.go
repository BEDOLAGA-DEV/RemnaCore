package bothost

import "context"

// TariffPrice is one subscription period option surfaced to a bot plugin.
type TariffPrice struct {
	Days     int    `json:"days"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Label    string `json:"label"`
	PlanID   string `json:"plan_id"` // checkout PlanID for this period
}

// TariffOffer is the bot-plugin view of a tariff. It groups all available
// billing periods for a single product under one offer. The top-level PlanID
// equals the tariff document ID (single-period plan ID); each Periods entry
// carries its own per-period PlanID (which equals the document ID for
// single-period tariffs and a derived UUIDv5 for multi-period ones).
type TariffOffer struct {
	PlanID      string        `json:"plan_id"`      // docID — whole-offer handle
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Periods     []TariffPrice `json:"periods"`
}

// TariffReader is the read-only tariff surface exposed to bot plugins via the
// OpContext. Implementations are expected to be tenant-scoped through the
// context (set by the caller via RunInTx + WithTenantID); they must NOT set
// the GUC themselves.
type TariffReader interface {
	// ListVisible returns the active tariffs visible in the named channel
	// (e.g. "telegram") for the tenant encoded in ctx.
	ListVisible(ctx context.Context, channel string) ([]TariffOffer, error)

	// Get resolves a planID (either a docID or a derived period UUID) to a
	// TariffOffer. Returns a not-found error when no matching tariff exists.
	Get(ctx context.Context, planID string) (*TariffOffer, error)
}
