package tariff

import (
	"fmt"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/money"
)

// PriceContext flows through the pricing pipeline, accumulating
// modifications and producing a final price with full audit trail.
type PriceContext struct {
	TariffID string `json:"tariff_id"`
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	Country  string `json:"country"` // ISO-3166
	Currency string `json:"currency"`
	Duration int    `json:"duration"` // days
	Quantity int    `json:"quantity"` // seats (B2B)

	PromoCode string `json:"promo_code,omitempty"`

	// Extension cache: populated by early hooks, read by later stages.
	UserGroups []string `json:"user_groups,omitempty"`
	UserCohort string   `json:"user_cohort,omitempty"`
	UserTier   string   `json:"user_tier,omitempty"`

	BasePrice  int64               `json:"base_price"`
	Modifiers  []PriceModification `json:"modifiers"`
	FinalPrice int64               `json:"final_price"`
	Breakdown  map[string]int64    `json:"breakdown"`
}

// PriceModification is a single audit entry in the pricing pipeline.
type PriceModification struct {
	Step        string `json:"step"`
	RuleID      string `json:"rule_id,omitempty"`
	AmountCents int64  `json:"amount_cents"`
	Reason      string `json:"reason"`
}

// PricingStage is a single step in the pricing pipeline.
type PricingStage interface {
	Name() string
	Apply(ctx *PriceContext) error
}

// PricingPipeline orchestrates a sequence of pricing stages.
type PricingPipeline struct {
	stages []PricingStage
}

// NewPricingPipeline creates a pipeline from the given stages.
func NewPricingPipeline(stages ...PricingStage) *PricingPipeline {
	return &PricingPipeline{stages: stages}
}

// Calculate runs every stage and ensures the final price is non-negative.
func (p *PricingPipeline) Calculate(ctx *PriceContext) error {
	ctx.Modifiers = make([]PriceModification, 0)
	ctx.Breakdown = make(map[string]int64)
	ctx.FinalPrice = ctx.BasePrice
	ctx.Breakdown["base"] = ctx.BasePrice

	for _, stage := range p.stages {
		if err := stage.Apply(ctx); err != nil {
			return fmt.Errorf("pricing stage %s: %w", stage.Name(), err)
		}
	}

	if ctx.FinalPrice < 0 {
		ctx.FinalPrice = 0
	}

	return nil
}

// --- Modifier types ---

// ModifierOp enumerates the operations a pricing modifier can perform.
type ModifierOp string

const (
	ModifierPercentOff   ModifierOp = "percent_off"
	ModifierPercentUp    ModifierOp = "percent_up"
	ModifierFixedOff     ModifierOp = "fixed_off"
	ModifierFixedUp      ModifierOp = "fixed_up"
	ModifierReplacePrice ModifierOp = "replace_price"
	ModifierMultiply     ModifierOp = "multiply"
)

// StackingPolicy determines how multiple discounts combine.
type StackingPolicy string

const (
	StackingAdditive       StackingPolicy = "additive"
	StackingMultiplicative StackingPolicy = "multiplicative"
	StackingBestOnly       StackingPolicy = "best_only"
)

// PricingRuleModifier is a single modifier entry inside a PricingRule.
type PricingRuleModifier struct {
	Type       string         `json:"type"` // "geo", "loyalty", "bulk_duration", "quantity"
	Priority   int            `json:"priority"`
	Conditions map[string]any `json:"conditions"`
	Action     ModifierAction `json:"action"`
}

// ModifierAction describes the arithmetic to apply.
type ModifierAction struct {
	Op    ModifierOp `json:"op"`
	Value int64      `json:"value"` // basis points for percent (1000=10%), cents for fixed
}

// --- Concrete pipeline stages ---

// basePriceStage sets the starting price from the tariff.
type basePriceStage struct {
	tariff *TariffInput
}

func (s *basePriceStage) Name() string { return "base_price" }

func (s *basePriceStage) Apply(ctx *PriceContext) error {
	if s.tariff == nil {
		return nil
	}
	if price, ok := s.tariff.MultiCurrencyPrices[ctx.Currency]; ok {
		ctx.BasePrice = price
		ctx.FinalPrice = price
	} else {
		ctx.BasePrice = s.tariff.PriceAmount
		ctx.FinalPrice = s.tariff.PriceAmount
	}
	ctx.Breakdown["base"] = ctx.BasePrice
	return nil
}

// geoAdjustStage applies geo-based price adjustments.
type geoAdjustStage struct {
	modifiers []PricingRuleModifier
}

func (s *geoAdjustStage) Name() string { return "geo_adjust" }

func (s *geoAdjustStage) Apply(ctx *PriceContext) error {
	if ctx.Country == "" {
		return nil
	}
	for _, mod := range s.modifiers {
		if mod.Type != "geo" {
			continue
		}
		countries, ok := mod.Conditions["countries"]
		if !ok {
			continue
		}
		list, ok := countries.([]any)
		if !ok {
			continue
		}
		for _, c := range list {
			cs, ok := c.(string)
			if !ok || cs != ctx.Country {
				continue
			}
			delta := applyModifierAction(ctx.FinalPrice, mod.Action)
			ctx.FinalPrice += delta
			ctx.Modifiers = append(ctx.Modifiers, PriceModification{
				Step:        "geo_adjust",
				RuleID:      mod.Type,
				AmountCents: delta,
				Reason:      fmt.Sprintf("%s geo adjustment", ctx.Country),
			})
			ctx.Breakdown["geo"] = delta
			return nil
		}
	}
	return nil
}

// promoGroupAdjustStage is a no-op unless UserGroups is populated by
// the promo-groups plugin via user.resolve_groups hook.
type promoGroupAdjustStage struct{}

func (s *promoGroupAdjustStage) Name() string { return "promo_group_adjust" }

func (s *promoGroupAdjustStage) Apply(ctx *PriceContext) error {
	// Populated by promo-groups plugin via tariff.calculate_price hook.
	return nil
}

// loyaltyDiscountStage is a no-op unless UserTier is populated by the
// loyalty plugin via user.resolve_tier hook.
type loyaltyDiscountStage struct{}

func (s *loyaltyDiscountStage) Name() string { return "loyalty_discount" }

func (s *loyaltyDiscountStage) Apply(ctx *PriceContext) error {
	// Populated by loyalty plugin.
	return nil
}

// bulkDiscountStage applies discounts for long durations or large quantities.
type bulkDiscountStage struct {
	modifiers []PricingRuleModifier
}

func (s *bulkDiscountStage) Name() string { return "bulk_discount" }

func (s *bulkDiscountStage) Apply(ctx *PriceContext) error {
	for _, mod := range s.modifiers {
		if mod.Type != "bulk_duration" && mod.Type != "quantity" {
			continue
		}
		if mod.Type == "bulk_duration" {
			minDays, _ := toInt(mod.Conditions["duration_days_min"])
			if ctx.Duration < minDays {
				continue
			}
		}
		if mod.Type == "quantity" {
			minQty, _ := toInt(mod.Conditions["min_quantity"])
			if ctx.Quantity < minQty {
				continue
			}
		}
		delta := applyModifierAction(ctx.FinalPrice, mod.Action)
		ctx.FinalPrice += delta
		ctx.Modifiers = append(ctx.Modifiers, PriceModification{
			Step:        "bulk_discount",
			RuleID:      mod.Type,
			AmountCents: delta,
			Reason:      fmt.Sprintf("%s discount applied", mod.Type),
		})
		ctx.Breakdown["bulk"] += delta
	}
	return nil
}

// promoCodeStage applies a promo code discount.
type promoCodeStage struct {
	promoCode *PromoCodeInput
}

func (s *promoCodeStage) Name() string { return "promo_code" }

func (s *promoCodeStage) Apply(ctx *PriceContext) error {
	if s.promoCode == nil || ctx.PromoCode == "" {
		return nil
	}

	spec := s.promoCode.DiscountSpec
	delta := applyDiscountSpec(ctx.FinalPrice, spec)

	if spec.MaxDiscountCents > 0 && -delta > spec.MaxDiscountCents {
		delta = -spec.MaxDiscountCents
	}

	ctx.FinalPrice += delta
	ctx.Modifiers = append(ctx.Modifiers, PriceModification{
		Step:        "promo_code",
		RuleID:      ctx.PromoCode,
		AmountCents: delta,
		Reason:      fmt.Sprintf("promo code %s", ctx.PromoCode),
	})
	ctx.Breakdown["promo"] = delta
	return nil
}

// priceFloorStage ensures the price doesn't fall below a minimum.
type priceFloorStage struct {
	floorCents int64
}

func (s *priceFloorStage) Name() string { return "price_floor" }

func (s *priceFloorStage) Apply(ctx *PriceContext) error {
	if ctx.FinalPrice < s.floorCents {
		diff := s.floorCents - ctx.FinalPrice
		ctx.FinalPrice = s.floorCents
		ctx.Modifiers = append(ctx.Modifiers, PriceModification{
			Step:        "price_floor",
			AmountCents: diff,
			Reason:      "minimum price floor applied",
		})
		ctx.Breakdown["floor_adjust"] = diff
	}
	return nil
}

// charmPriceStage rounds the price to .99 or .95 endings.
type charmPriceStage struct {
	strategy string
}

func (s *charmPriceStage) Name() string { return "charm_price" }

func (s *charmPriceStage) Apply(ctx *PriceContext) error {
	if s.strategy == "" || s.strategy == CharmStrategyNone {
		return nil
	}

	original := ctx.FinalPrice
	remainder := ctx.FinalPrice % 100
	dollars := ctx.FinalPrice / 100

	// Skip charm rounding for sub-$1 prices to avoid making items free.
	if dollars == 0 {
		return nil
	}

	switch s.strategy {
	case CharmStrategy99:
		if remainder != 99 {
			ctx.FinalPrice = dollars*100 - 1
		}
	case CharmStrategy95:
		if remainder != 95 {
			ctx.FinalPrice = dollars*100 - 5
		}
	}

	if ctx.FinalPrice != original {
		delta := ctx.FinalPrice - original
		ctx.Modifiers = append(ctx.Modifiers, PriceModification{
			Step:        "charm_price",
			AmountCents: delta,
			Reason:      fmt.Sprintf("charm pricing .%s", s.strategy),
		})
	}
	return nil
}

// --- Helper functions ---

// applyModifierAction calculates the price delta for a modifier action.
func applyModifierAction(currentPrice int64, action ModifierAction) int64 {
	switch action.Op {
	case ModifierPercentOff:
		pct := action.Value
		if pct > money.PercentBase {
			pct = money.PercentBase
		}
		return -(currentPrice * pct / money.PercentBase)
	case ModifierPercentUp:
		return currentPrice * action.Value / money.PercentBase
	case ModifierFixedOff:
		return -action.Value
	case ModifierFixedUp:
		return action.Value
	case ModifierReplacePrice:
		return action.Value - currentPrice
	case ModifierMultiply:
		return currentPrice*action.Value/money.PercentBase - currentPrice
	default:
		return 0
	}
}

// applyDiscountSpec calculates the price delta from a DiscountSpec.
func applyDiscountSpec(currentPrice int64, spec DiscountSpec) int64 {
	switch ModifierOp(spec.Type) {
	case ModifierPercentOff:
		pct := spec.Value
		if pct > money.PercentBase {
			pct = money.PercentBase
		}
		return -(currentPrice * pct / money.PercentBase)
	case ModifierFixedOff:
		return -spec.Value
	default:
		return 0
	}
}

// toInt converts an interface{} to int, handling float64 from JSON.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	case int64:
		return int(n), true
	default:
		return 0, false
	}
}

// DefaultPipeline creates the standard 8-stage pricing pipeline.
func DefaultPipeline(tariff *TariffInput, rule *PricingRuleInput, promo *PromoCodeInput) *PricingPipeline {
	var modifiers []PricingRuleModifier
	var floorCents int64
	var charm string

	if rule != nil {
		modifiers = rule.Modifiers
		floorCents = rule.PriceFloorCents
		charm = rule.Rounding
	}
	if tariff != nil && tariff.PriceCharmStrategy != "" {
		charm = tariff.PriceCharmStrategy
	}

	return NewPricingPipeline(
		&basePriceStage{tariff: tariff},
		&geoAdjustStage{modifiers: modifiers},
		&promoGroupAdjustStage{},
		&loyaltyDiscountStage{},
		&bulkDiscountStage{modifiers: modifiers},
		&promoCodeStage{promoCode: promo},
		&priceFloorStage{floorCents: floorCents},
		&charmPriceStage{strategy: charm},
	)
}
