package tariff

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
)

// visibleTariffLister is the narrow callable surface the adapter requires from
// the tariff handler. *Handler satisfies this interface; the narrow type is
// declared here so tests can inject a stub without needing the full Handler.
type visibleTariffLister interface {
	ListVisibleTariffs(ctx context.Context, channel string) ([]TariffResponse, error)
	GetTariffByPlanID(ctx context.Context, planID string) (*TariffResponse, error)
}

// TariffReaderAdapter adapts the tariff Handler to the bothost.TariffReader
// interface. It maps TariffResponse → bothost.TariffOffer, building per-period
// PlanIDs via DerivePlanID so that the planIDs presented to bot plugins match
// exactly the planIDs the checkout path resolves.
//
// Tenant scoping is the caller's responsibility: the ctx passed to each method
// must carry the tenant GUC (wrapped in RunInTx(WithTenantID(...))); this
// adapter does not set the GUC itself.
type TariffReaderAdapter struct {
	tariffs visibleTariffLister
	logger  *slog.Logger
}

// NewTariffReaderAdapter returns an adapter backed by lister.
// In production, pass the *tariff.Handler directly.
func NewTariffReaderAdapter(lister visibleTariffLister, logger *slog.Logger) *TariffReaderAdapter {
	return &TariffReaderAdapter{tariffs: lister, logger: logger}
}

// ListVisible implements bothost.TariffReader.
func (a *TariffReaderAdapter) ListVisible(ctx context.Context, channel string) ([]bothost.TariffOffer, error) {
	resps, err := a.tariffs.ListVisibleTariffs(ctx, channel)
	if err != nil {
		return nil, fmt.Errorf("list visible tariffs: %w", err)
	}
	offers := make([]bothost.TariffOffer, 0, len(resps))
	for _, r := range resps {
		offer, mapErr := tariffResponseToOffer(r)
		if mapErr != nil {
			// A corrupted document must not hide the rest of the catalog, but it
			// must not vanish silently either.
			a.logger.WarnContext(ctx, "skipping unmappable tariff in bot catalog",
				slog.String("tariff", r.ID), slog.Any("error", mapErr))
			continue
		}
		offers = append(offers, offer)
	}
	return offers, nil
}

// Get implements bothost.TariffReader.
func (a *TariffReaderAdapter) Get(ctx context.Context, planID string) (*bothost.TariffOffer, error) {
	resp, err := a.tariffs.GetTariffByPlanID(ctx, planID)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}
	offer, err := tariffResponseToOffer(*resp)
	if err != nil {
		return nil, fmt.Errorf("map tariff %s: %w", resp.ID, err)
	}
	return &offer, nil
}

// tariffResponseToOffer converts a TariffResponse into a bothost.TariffOffer.
// When PricingPeriods is empty, a single period is synthesised from the
// top-level DurationDays / PriceAmount fields so the offer always has at least
// one Periods entry.
func tariffResponseToOffer(resp TariffResponse) (bothost.TariffOffer, error) {
	// Determine whether this tariff has multiple pricing periods. This flag is
	// the same one DerivePlanID uses, so plan IDs remain consistent.
	multiPeriod := len(resp.PricingPeriods) > 0

	// Build the concrete period list: either the stored periods or a synthesised
	// single entry matching the sync-side behaviour in syncTariffToPlan.
	type rawPeriod struct {
		durationDays int
		priceAmount  int64
		label        string
	}
	var rawPeriods []rawPeriod
	if multiPeriod {
		for _, p := range resp.PricingPeriods {
			rawPeriods = append(rawPeriods, rawPeriod{
				durationDays: p.DurationDays,
				priceAmount:  p.PriceAmount,
				label:        p.Label,
			})
		}
	} else {
		rawPeriods = []rawPeriod{{
			durationDays: resp.DurationDays,
			priceAmount:  resp.PriceAmount,
			label:        "",
		}}
	}

	prices := make([]bothost.TariffPrice, 0, len(rawPeriods))
	for _, rp := range rawPeriods {
		planID, err := DerivePlanID(resp.ID, rp.durationDays, multiPeriod)
		if err != nil {
			// DerivePlanID fails only on an unparseable document ID, which breaks
			// every period identically — surface it instead of emitting a broken
			// offer with missing periods.
			return bothost.TariffOffer{}, fmt.Errorf("derive plan id for tariff %s: %w", resp.ID, err)
		}
		prices = append(prices, bothost.TariffPrice{
			Days:     rp.durationDays,
			Amount:   rp.priceAmount,
			Currency: resp.PriceCurrency,
			Label:    rp.label,
			PlanID:   planID,
		})
	}

	return bothost.TariffOffer{
		PlanID:      resp.ID,
		Name:        resp.Name,
		Description: resp.Description,
		Periods:     prices,
	}, nil
}

// Compile-time assertion: TariffReaderAdapter must implement bothost.TariffReader.
var _ bothost.TariffReader = (*TariffReaderAdapter)(nil)
