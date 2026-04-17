package tariff

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func baseTariff() *TariffInput {
	t := defaultTariffInput()
	t.Name = "Premium VPN"
	t.PriceAmount = 999
	t.PriceCurrency = "USD"
	t.DurationDays = 30
	return &t
}

func TestPipeline_BasePrice(t *testing.T) {
	tariff := baseTariff()
	pipeline := DefaultPipeline(tariff, nil, nil)

	ctx := &PriceContext{Currency: "USD"}
	require.NoError(t, pipeline.Calculate(ctx))

	assert.Equal(t, int64(999), ctx.BasePrice)
	assert.Equal(t, int64(999), ctx.FinalPrice)
	assert.Equal(t, int64(999), ctx.Breakdown["base"])
}

func TestPipeline_MultiCurrencyPrice(t *testing.T) {
	tariff := baseTariff()
	tariff.MultiCurrencyPrices = map[string]int64{"EUR": 899, "RUB": 79900}

	pipeline := DefaultPipeline(tariff, nil, nil)

	ctx := &PriceContext{Currency: "EUR"}
	require.NoError(t, pipeline.Calculate(ctx))

	assert.Equal(t, int64(899), ctx.BasePrice)
	assert.Equal(t, int64(899), ctx.FinalPrice)
}

func TestPipeline_FallbackToBasePriceWhenCurrencyNotInMap(t *testing.T) {
	tariff := baseTariff()
	tariff.MultiCurrencyPrices = map[string]int64{"EUR": 899}

	pipeline := DefaultPipeline(tariff, nil, nil)

	ctx := &PriceContext{Currency: "GBP"}
	require.NoError(t, pipeline.Calculate(ctx))

	assert.Equal(t, int64(999), ctx.FinalPrice)
}

func TestPipeline_GeoDiscount(t *testing.T) {
	tariff := baseTariff()
	rule := &PricingRuleInput{
		Modifiers: []PricingRuleModifier{
			{
				Type:       "geo",
				Priority:   10,
				Conditions: map[string]any{"countries": []any{"RU", "BY", "KZ"}},
				Action:     ModifierAction{Op: ModifierPercentOff, Value: 3000}, // 30%
			},
		},
	}

	pipeline := DefaultPipeline(tariff, rule, nil)

	ctx := &PriceContext{Currency: "USD", Country: "RU"}
	require.NoError(t, pipeline.Calculate(ctx))

	assert.Equal(t, int64(999), ctx.BasePrice)
	assert.Equal(t, int64(700), ctx.FinalPrice) // 999 - 999*3000/10000 = 999 - 299 = 700
	assert.Equal(t, int64(-299), ctx.Breakdown["geo"])
}

func TestPipeline_GeoNoMatch(t *testing.T) {
	tariff := baseTariff()
	rule := &PricingRuleInput{
		Modifiers: []PricingRuleModifier{
			{
				Type:       "geo",
				Priority:   10,
				Conditions: map[string]any{"countries": []any{"RU"}},
				Action:     ModifierAction{Op: ModifierPercentOff, Value: 3000},
			},
		},
	}

	pipeline := DefaultPipeline(tariff, rule, nil)

	ctx := &PriceContext{Currency: "USD", Country: "US"}
	require.NoError(t, pipeline.Calculate(ctx))

	assert.Equal(t, int64(999), ctx.FinalPrice)
}

func TestPipeline_BulkDurationDiscount(t *testing.T) {
	tariff := baseTariff()
	rule := &PricingRuleInput{
		Modifiers: []PricingRuleModifier{
			{
				Type:       "bulk_duration",
				Priority:   30,
				Conditions: map[string]any{"duration_days_min": float64(365)},
				Action:     ModifierAction{Op: ModifierPercentOff, Value: 2000}, // 20%
			},
		},
	}

	pipeline := DefaultPipeline(tariff, rule, nil)

	ctx := &PriceContext{Currency: "USD", Duration: 365}
	require.NoError(t, pipeline.Calculate(ctx))

	assert.Equal(t, int64(800), ctx.FinalPrice) // 999 - 999*2000/10000 = 999 - 199 = 800
}

func TestPipeline_BulkDurationNotMet(t *testing.T) {
	tariff := baseTariff()
	rule := &PricingRuleInput{
		Modifiers: []PricingRuleModifier{
			{
				Type:       "bulk_duration",
				Priority:   30,
				Conditions: map[string]any{"duration_days_min": float64(365)},
				Action:     ModifierAction{Op: ModifierPercentOff, Value: 2000},
			},
		},
	}

	pipeline := DefaultPipeline(tariff, rule, nil)

	ctx := &PriceContext{Currency: "USD", Duration: 30}
	require.NoError(t, pipeline.Calculate(ctx))

	assert.Equal(t, int64(999), ctx.FinalPrice)
}

func TestPipeline_QuantityDiscount(t *testing.T) {
	tariff := baseTariff()
	rule := &PricingRuleInput{
		Modifiers: []PricingRuleModifier{
			{
				Type:       "quantity",
				Priority:   40,
				Conditions: map[string]any{"min_quantity": float64(10)},
				Action:     ModifierAction{Op: ModifierPercentOff, Value: 1500}, // 15%
			},
		},
	}

	pipeline := DefaultPipeline(tariff, rule, nil)

	ctx := &PriceContext{Currency: "USD", Quantity: 15}
	require.NoError(t, pipeline.Calculate(ctx))

	assert.Equal(t, int64(850), ctx.FinalPrice) // 999 - 999*1500/10000 = 999 - 149 = 850
}

func TestPipeline_PromoCodePercentOff(t *testing.T) {
	tariff := baseTariff()
	promo := &PromoCodeInput{
		Code:     "WELCOME20",
		IsActive: true,
		DiscountSpec: DiscountSpec{
			Type:  "percent_off",
			Value: 2000, // 20%
		},
	}

	pipeline := DefaultPipeline(tariff, nil, promo)

	ctx := &PriceContext{Currency: "USD", PromoCode: "WELCOME20"}
	require.NoError(t, pipeline.Calculate(ctx))

	assert.Equal(t, int64(800), ctx.FinalPrice) // 999 - 999*2000/10000 = 800
}

func TestPipeline_PromoCodeFixedOff(t *testing.T) {
	tariff := baseTariff()
	promo := &PromoCodeInput{
		Code:     "SAVE100",
		IsActive: true,
		DiscountSpec: DiscountSpec{
			Type:  "fixed_off",
			Value: 100,
		},
	}

	pipeline := DefaultPipeline(tariff, nil, promo)

	ctx := &PriceContext{Currency: "USD", PromoCode: "SAVE100"}
	require.NoError(t, pipeline.Calculate(ctx))

	assert.Equal(t, int64(899), ctx.FinalPrice)
}

func TestPipeline_PromoCodeWithMaxDiscount(t *testing.T) {
	tariff := baseTariff()
	tariff.PriceAmount = 10000 // $100
	promo := &PromoCodeInput{
		Code:     "HALF",
		IsActive: true,
		DiscountSpec: DiscountSpec{
			Type:             "percent_off",
			Value:            5000, // 50%
			MaxDiscountCents: 3000, // max $30 off
		},
	}

	pipeline := DefaultPipeline(tariff, nil, promo)

	ctx := &PriceContext{Currency: "USD", PromoCode: "HALF"}
	require.NoError(t, pipeline.Calculate(ctx))

	assert.Equal(t, int64(7000), ctx.FinalPrice) // 10000 - 3000 = 7000 (capped)
}

func TestPipeline_PromoCodeNoCodeProvided(t *testing.T) {
	tariff := baseTariff()
	promo := &PromoCodeInput{
		Code:     "CODE",
		IsActive: true,
		DiscountSpec: DiscountSpec{
			Type:  "percent_off",
			Value: 5000,
		},
	}

	pipeline := DefaultPipeline(tariff, nil, promo)

	ctx := &PriceContext{Currency: "USD"} // no PromoCode set
	require.NoError(t, pipeline.Calculate(ctx))

	assert.Equal(t, int64(999), ctx.FinalPrice) // no discount
}

func TestPipeline_PriceFloor(t *testing.T) {
	tariff := baseTariff()
	tariff.PriceAmount = 200
	promo := &PromoCodeInput{
		Code:     "BIG",
		IsActive: true,
		DiscountSpec: DiscountSpec{
			Type:  "fixed_off",
			Value: 300, // more than price
		},
	}
	rule := &PricingRuleInput{
		PriceFloorCents: 100,
	}

	pipeline := DefaultPipeline(tariff, rule, promo)

	ctx := &PriceContext{Currency: "USD", PromoCode: "BIG"}
	require.NoError(t, pipeline.Calculate(ctx))

	assert.Equal(t, int64(100), ctx.FinalPrice) // floored at 100
}

func TestPipeline_PriceNeverNegative(t *testing.T) {
	tariff := baseTariff()
	tariff.PriceAmount = 100
	promo := &PromoCodeInput{
		Code:     "FREE",
		IsActive: true,
		DiscountSpec: DiscountSpec{
			Type:  "fixed_off",
			Value: 500,
		},
	}

	pipeline := DefaultPipeline(tariff, nil, promo)

	ctx := &PriceContext{Currency: "USD", PromoCode: "FREE"}
	require.NoError(t, pipeline.Calculate(ctx))

	assert.True(t, ctx.FinalPrice >= 0)
}

func TestPipeline_CharmPrice99(t *testing.T) {
	tariff := baseTariff()
	tariff.PriceAmount = 1000
	tariff.PriceCharmStrategy = CharmStrategy99

	pipeline := DefaultPipeline(tariff, nil, nil)

	ctx := &PriceContext{Currency: "USD"}
	require.NoError(t, pipeline.Calculate(ctx))

	assert.Equal(t, int64(999), ctx.FinalPrice) // 1000 -> 999
}

func TestPipeline_CharmPrice95(t *testing.T) {
	tariff := baseTariff()
	tariff.PriceAmount = 1000
	tariff.PriceCharmStrategy = CharmStrategy95

	pipeline := DefaultPipeline(tariff, nil, nil)

	ctx := &PriceContext{Currency: "USD"}
	require.NoError(t, pipeline.Calculate(ctx))

	assert.Equal(t, int64(995), ctx.FinalPrice) // 1000 -> 995
}

func TestPipeline_CharmPrice_SubDollar_NoChange(t *testing.T) {
	tariff := baseTariff()
	tariff.PriceAmount = 50 // 50 cents
	tariff.PriceCharmStrategy = CharmStrategy99

	pipeline := DefaultPipeline(tariff, nil, nil)

	ctx := &PriceContext{Currency: "USD"}
	require.NoError(t, pipeline.Calculate(ctx))

	assert.Equal(t, int64(50), ctx.FinalPrice) // sub-$1 — no charm applied
}

func TestPipeline_CharmPrice_Already99(t *testing.T) {
	tariff := baseTariff()
	tariff.PriceAmount = 999
	tariff.PriceCharmStrategy = CharmStrategy99

	pipeline := DefaultPipeline(tariff, nil, nil)

	ctx := &PriceContext{Currency: "USD"}
	require.NoError(t, pipeline.Calculate(ctx))

	assert.Equal(t, int64(999), ctx.FinalPrice) // already .99, no change
}

func TestPipeline_CharmPrice_ZeroPrice(t *testing.T) {
	tariff := baseTariff()
	tariff.PriceAmount = 0
	tariff.PriceCharmStrategy = CharmStrategy99
	tariff.BillingModel = string(BillingModelPAYG)
	tariff.OveragePerGBCents = 1
	tariff.DurationDays = 0

	pipeline := DefaultPipeline(tariff, nil, nil)

	ctx := &PriceContext{Currency: "USD"}
	require.NoError(t, pipeline.Calculate(ctx))

	assert.Equal(t, int64(0), ctx.FinalPrice) // zero stays zero
}

func TestPipeline_BulkBreakdownAccumulates(t *testing.T) {
	tariff := baseTariff()
	tariff.PriceAmount = 10000 // $100
	rule := &PricingRuleInput{
		Modifiers: []PricingRuleModifier{
			{
				Type:       "bulk_duration",
				Priority:   30,
				Conditions: map[string]any{"duration_days_min": float64(365)},
				Action:     ModifierAction{Op: ModifierPercentOff, Value: 1000}, // 10%
			},
			{
				Type:       "quantity",
				Priority:   40,
				Conditions: map[string]any{"min_quantity": float64(5)},
				Action:     ModifierAction{Op: ModifierPercentOff, Value: 500}, // 5%
			},
		},
	}

	pipeline := DefaultPipeline(tariff, rule, nil)

	ctx := &PriceContext{Currency: "USD", Duration: 365, Quantity: 10}
	require.NoError(t, pipeline.Calculate(ctx))

	// Both bulk modifiers should fire; breakdown["bulk"] should be cumulative
	assert.Len(t, ctx.Modifiers, 2)
	assert.Equal(t, ctx.Breakdown["bulk"], ctx.Modifiers[0].AmountCents+ctx.Modifiers[1].AmountCents)
}

func TestPipeline_FullStack_GeoAndPromo(t *testing.T) {
	tariff := baseTariff()
	tariff.PriceAmount = 999
	rule := &PricingRuleInput{
		Modifiers: []PricingRuleModifier{
			{
				Type:       "geo",
				Priority:   10,
				Conditions: map[string]any{"countries": []any{"RU"}},
				Action:     ModifierAction{Op: ModifierPercentOff, Value: 3000},
			},
		},
		PriceFloorCents: 100,
	}
	promo := &PromoCodeInput{
		Code:     "WELCOME20",
		IsActive: true,
		DiscountSpec: DiscountSpec{
			Type:  "percent_off",
			Value: 2000,
		},
	}

	pipeline := DefaultPipeline(tariff, rule, promo)

	ctx := &PriceContext{Currency: "USD", Country: "RU", PromoCode: "WELCOME20"}
	require.NoError(t, pipeline.Calculate(ctx))

	// Base: 999 -> geo -299 (999*3000/10000): 700 -> promo -140 (700*2000/10000): 560
	assert.Equal(t, int64(999), ctx.BasePrice)
	assert.Equal(t, int64(560), ctx.FinalPrice)
	assert.Len(t, ctx.Modifiers, 2)
}

func TestPipeline_AuditTrail(t *testing.T) {
	tariff := baseTariff()
	tariff.PriceAmount = 1000
	promo := &PromoCodeInput{
		Code:     "TEST",
		IsActive: true,
		DiscountSpec: DiscountSpec{
			Type:  "fixed_off",
			Value: 100,
		},
	}

	pipeline := DefaultPipeline(tariff, nil, promo)

	ctx := &PriceContext{Currency: "USD", PromoCode: "TEST"}
	require.NoError(t, pipeline.Calculate(ctx))

	require.Len(t, ctx.Modifiers, 1)
	assert.Equal(t, "promo_code", ctx.Modifiers[0].Step)
	assert.Equal(t, int64(-100), ctx.Modifiers[0].AmountCents)
	assert.Equal(t, "TEST", ctx.Modifiers[0].RuleID)
}

// --- applyModifierAction tests ---

func TestApplyModifierAction_PercentOff(t *testing.T) {
	delta := applyModifierAction(1000, ModifierAction{Op: ModifierPercentOff, Value: 1000}) // 10%
	assert.Equal(t, int64(-100), delta)
}

func TestApplyModifierAction_PercentOffClampAt100(t *testing.T) {
	delta := applyModifierAction(1000, ModifierAction{Op: ModifierPercentOff, Value: 15000}) // >100%
	assert.Equal(t, int64(-1000), delta) // clamped at 100%
}

func TestApplyModifierAction_PercentUp(t *testing.T) {
	delta := applyModifierAction(1000, ModifierAction{Op: ModifierPercentUp, Value: 2000}) // 20%
	assert.Equal(t, int64(200), delta)
}

func TestApplyModifierAction_FixedOff(t *testing.T) {
	delta := applyModifierAction(1000, ModifierAction{Op: ModifierFixedOff, Value: 300})
	assert.Equal(t, int64(-300), delta)
}

func TestApplyModifierAction_FixedUp(t *testing.T) {
	delta := applyModifierAction(1000, ModifierAction{Op: ModifierFixedUp, Value: 200})
	assert.Equal(t, int64(200), delta)
}

func TestApplyModifierAction_ReplacePrice(t *testing.T) {
	delta := applyModifierAction(1000, ModifierAction{Op: ModifierReplacePrice, Value: 500})
	assert.Equal(t, int64(-500), delta) // 500 - 1000 = -500
}

func TestApplyModifierAction_Multiply(t *testing.T) {
	delta := applyModifierAction(1000, ModifierAction{Op: ModifierMultiply, Value: 7000}) // 0.7x
	assert.Equal(t, int64(-300), delta) // 1000*0.7 - 1000 = -300
}

func TestApplyModifierAction_UnknownOp(t *testing.T) {
	delta := applyModifierAction(1000, ModifierAction{Op: "unknown", Value: 500})
	assert.Equal(t, int64(0), delta)
}

// --- toInt tests ---

func TestToInt(t *testing.T) {
	tests := []struct {
		input    any
		expected int
		ok       bool
	}{
		{42, 42, true},
		{float64(42), 42, true},
		{int64(42), 42, true},
		{"42", 0, false},
		{nil, 0, false},
	}

	for _, tt := range tests {
		val, ok := toInt(tt.input)
		assert.Equal(t, tt.expected, val)
		assert.Equal(t, tt.ok, ok)
	}
}
