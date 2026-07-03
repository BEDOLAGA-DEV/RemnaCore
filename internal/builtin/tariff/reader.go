package tariff

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

// listMergedTariffs performs the dual-read (own-tenant documents + platform
// shared templates), merges and deduplicates them by document ID, and converts
// each document to a TariffResponse. Documents that fail to decode are silently
// skipped. The result is NOT sorted; callers sort at the right level.
//
// ctx must carry the tenant GUC. The platform-scoped read is done internally
// by wrapping ctx with tenantctx.WithPlatformScope.
func (h *Handler) listMergedTariffs(ctx context.Context) ([]TariffResponse, error) {
	ownDocs, err := h.collections.ListDocuments(ctx, PluginSlug, CollectionName)
	if err != nil {
		return nil, fmt.Errorf("list own tariffs: %w", err)
	}
	templateDocs, err := h.collections.ListDocuments(
		tenantctx.WithPlatformScope(ctx), PluginSlug, CollectionName,
	)
	if err != nil {
		return nil, fmt.Errorf("list tariff templates: %w", err)
	}
	docs := mergeTariffDocs(ownDocs, templateDocs)

	tariffs := make([]TariffResponse, 0, len(docs))
	for _, doc := range docs {
		t, convErr := documentToTariff(&doc)
		if convErr != nil {
			continue
		}
		tariffs = append(tariffs, *t)
	}
	return tariffs, nil
}

// ListVisibleTariffs returns the active tariffs visible in the given channel
// for the tenant encoded in ctx.
//
// ctx must carry the tenant GUC — the caller (e.g. a bot op wrapped in
// RunInTx(WithTenantID(...))) owns that contract; this method does NOT set the
// GUC itself.
//
// Channel mapping:
//   - "telegram" → VisibleInTelegram
//   - "cabinet"  → VisibleInCabinet
//   - "public"   → VisibleInPublic
//   - unknown    → VisibleInTelegram (default)
func (h *Handler) ListVisibleTariffs(ctx context.Context, channel string) ([]TariffResponse, error) {
	all, err := h.listMergedTariffs(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]TariffResponse, 0, len(all))
	for _, t := range all {
		if !t.IsActive {
			continue
		}
		if !visibleInChannel(&t, channel) {
			continue
		}
		out = append(out, t)
	}

	slices.SortFunc(out, func(a, b TariffResponse) int {
		return a.SortOrder - b.SortOrder
	})

	// Personalize per-period prices when the bot passed price hints (geo/promo/
	// user). With no hints this is a no-op and list prices are returned.
	h.personalizeVisible(ctx, out)

	return out, nil
}

// personalizeVisible applies the pricing pipeline to each visible tariff's
// periods when ctx carries bothost.PriceHints. It mutates out in place. Failures
// to load a rule/promo degrade to list prices for that tariff (never an error).
func (h *Handler) personalizeVisible(ctx context.Context, out []TariffResponse) {
	hints := bothost.PriceHintsFromContext(ctx)
	if hints.IsZero() {
		return
	}
	var promo *PromoCodeInput
	if hints.PromoCode != "" {
		promo = h.findPromoByCode(ctx, hints.PromoCode)
	}
	for i := range out {
		t := &out[i]
		var rule *PricingRuleInput
		if t.PricingRuleID != "" {
			if doc, err := h.collections.GetDocument(ctx, PluginSlug, CollectionPricingRules, t.PricingRuleID); err == nil {
				var pr PricingRuleInput
				if json.Unmarshal(doc.Data, &pr) == nil {
					rule = &pr
				}
			}
		}
		cur := t.PriceCurrency
		if len(t.PricingPeriods) > 0 {
			for j := range t.PricingPeriods {
				t.PricingPeriods[j].PriceAmount = h.pricePeriod(rule, promo, t.PriceCharmStrategy, t.PricingPeriods[j].PriceAmount, hints, t.PricingPeriods[j].DurationDays, cur, t.ID)
			}
		} else {
			t.PriceAmount = h.pricePeriod(rule, promo, t.PriceCharmStrategy, t.PriceAmount, hints, t.DurationDays, cur, t.ID)
		}
	}
}

// pricePeriod runs the pricing pipeline (without the base-price stage — the base
// is the period's own stored price) over a single period, returning the
// personalized amount. On any pipeline error it returns the unmodified base.
func (h *Handler) pricePeriod(rule *PricingRuleInput, promo *PromoCodeInput, charm string, base int64, hints bothost.PriceHints, durationDays int, currency, tariffID string) int64 {
	var modifiers []PricingRuleModifier
	var floor int64
	if rule != nil {
		modifiers = rule.Modifiers
		floor = rule.PriceFloorCents
		if charm == "" {
			charm = rule.Rounding
		}
	}
	pctx := &PriceContext{
		TariffID:   tariffID,
		UserID:     hints.UserID,
		Country:    hints.Country,
		Currency:   currency,
		Duration:   durationDays,
		Quantity:   1,
		PromoCode:  hints.PromoCode,
		BasePrice:  base,
		FinalPrice: base,
	}
	pipe := NewPricingPipeline(
		&geoAdjustStage{modifiers: modifiers},
		&promoGroupAdjustStage{},
		&loyaltyDiscountStage{},
		&bulkDiscountStage{modifiers: modifiers},
		&promoCodeStage{promoCode: promo},
		&priceFloorStage{floorCents: floor},
		&charmPriceStage{strategy: charm},
	)
	if err := pipe.Calculate(pctx); err != nil {
		return base
	}
	return pctx.FinalPrice
}

// visibleInChannel reports whether a tariff is visible in the named channel.
// Unknown channels fall back to the telegram flag (the bot-dispatch default).
func visibleInChannel(t *TariffResponse, channel string) bool {
	switch channel {
	case bothost.ChannelTelegram:
		return t.VisibleInTelegram
	case bothost.ChannelCabinet:
		return t.VisibleInCabinet
	case bothost.ChannelPublic:
		return t.VisibleInPublic
	default:
		return t.VisibleInTelegram
	}
}

// GetTariffByPlanID resolves a planID to a TariffResponse by scanning active
// telegram-visible tariffs. planID may be either a tariff document ID
// (single-period tariff) or a derived UUIDv5 (one period of a multi-period
// tariff).
//
// Returns pluginstore.ErrDocumentNotFound (wrapped) when no matching tariff is
// found. ctx is forwarded to ListVisibleTariffs without modification.
func (h *Handler) GetTariffByPlanID(ctx context.Context, planID string) (*TariffResponse, error) {
	all, err := h.ListVisibleTariffs(ctx, bothost.ChannelTelegram)
	if err != nil {
		return nil, err
	}

	for i := range all {
		t := &all[i]
		// Single-period tariff: the document ID IS the plan ID.
		if t.ID == planID {
			return t, nil
		}
		// Multi-period tariff: check each period's derived plan ID.
		for _, p := range t.PricingPeriods {
			derived, deriveErr := DerivePlanID(t.ID, p.DurationDays, len(t.PricingPeriods) > 0)
			if deriveErr != nil {
				continue
			}
			if derived == planID {
				return t, nil
			}
		}
	}
	return nil, fmt.Errorf("tariff with plan ID %s: %w", planID, pluginstore.ErrDocumentNotFound)
}
