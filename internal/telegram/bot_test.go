package telegram

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
	msaggregate "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
)

func TestPlansKeyboard(t *testing.T) {
	plans := []*aggregate.Plan{
		{
			ID:        "plan-1",
			Name:      "Basic",
			BasePrice: vo.NewMoney(999, vo.CurrencyUSD),
			Interval:  vo.IntervalMonth,
		},
		{
			ID:        "plan-2",
			Name:      "Premium",
			BasePrice: vo.NewMoney(1999, vo.CurrencyUSD),
			Interval:  vo.IntervalMonth,
		},
	}

	kb := PlansKeyboard(plans)
	assert.NotNil(t, kb)
	assert.Len(t, kb.InlineKeyboard, 2)
	assert.Equal(t, "plan:plan-1", kb.InlineKeyboard[0][0].CallbackData)
	assert.Equal(t, "plan:plan-2", kb.InlineKeyboard[1][0].CallbackData)
}

func TestPlansKeyboard_Empty(t *testing.T) {
	kb := PlansKeyboard(nil)
	assert.NotNil(t, kb)
	assert.Empty(t, kb.InlineKeyboard)
}

func TestAddonsKeyboard(t *testing.T) {
	plan := &aggregate.Plan{
		ID:   "plan-1",
		Name: "Basic",
		AvailableAddons: []aggregate.Addon{
			{ID: "addon-1", Name: "Gaming", Price: vo.NewMoney(500, vo.CurrencyUSD)},
			{ID: "addon-2", Name: "Streaming", Price: vo.NewMoney(300, vo.CurrencyUSD)},
		},
	}

	kb := AddonsKeyboard(plan, []string{"addon-1"})
	assert.NotNil(t, kb)
	// 2 addons + 1 confirm button
	assert.Len(t, kb.InlineKeyboard, 3)
	// First addon should be selected
	assert.Contains(t, kb.InlineKeyboard[0][0].Text, "[x]")
	// Second addon should not be selected
	assert.NotContains(t, kb.InlineKeyboard[1][0].Text, "[x]")
	// Last row is the confirm button
	assert.Contains(t, kb.InlineKeyboard[2][0].CallbackData, CallbackPrefixConfirm)
}

func TestConfirmPurchaseKeyboard(t *testing.T) {
	kb := ConfirmPurchaseKeyboard("plan-1", []string{"addon-1", "addon-2"})
	assert.NotNil(t, kb)
	assert.Len(t, kb.InlineKeyboard, 1)
	assert.Len(t, kb.InlineKeyboard[0], 2)
	assert.Equal(t, "confirm:plan-1:addon-1,addon-2", kb.InlineKeyboard[0][0].CallbackData)
	assert.Equal(t, "cancel:checkout", kb.InlineKeyboard[0][1].CallbackData)
}

func TestSubscriptionKeyboard(t *testing.T) {
	sub := &aggregate.Subscription{ID: "sub-1"}
	bindings := []*msaggregate.RemnawaveBinding{
		{ID: "b-1", RemnawaveUsername: "user1", Purpose: msaggregate.PurposeBase},
	}
	kb := SubscriptionKeyboard(sub, bindings)
	assert.NotNil(t, kb)
	// 1 binding + 1 cancel button
	assert.Len(t, kb.InlineKeyboard, 2)
	assert.Equal(t, "cancel:sub-1", kb.InlineKeyboard[1][0].CallbackData)
}

func TestFormatPlanDetail(t *testing.T) {
	plan := &aggregate.Plan{
		Name:              "Basic VPN",
		Description:       "A basic plan",
		BasePrice:         vo.NewMoney(999, vo.CurrencyUSD),
		Interval:          vo.IntervalMonth,
		Tier:              aggregate.TierBasic,
		TrafficLimitBytes: 10 * 1024 * 1024 * 1024, // 10 GB
		DeviceLimit:       3,
		AllowedCountries:  []string{"US", "DE"},
		FamilyEnabled:     false,
	}

	text := FormatPlanDetail(plan)
	assert.Contains(t, text, "Basic VPN")
	assert.Contains(t, text, "A basic plan")
	assert.Contains(t, text, "9.99 usd")
	assert.Contains(t, text, "10.0 GB")
	assert.Contains(t, text, "US, DE")
}

func TestFormatTrafficUsage_Empty(t *testing.T) {
	text := FormatTrafficUsage(nil)
	assert.Equal(t, "No active bindings found.", text)
}

func TestFormatTrafficUsage_WithBindings(t *testing.T) {
	bindings := []*msaggregate.RemnawaveBinding{
		{
			RemnawaveUsername: "user1",
			Purpose:          msaggregate.PurposeBase,
			TrafficLimitBytes: 5 * 1024 * 1024 * 1024,
			Status:           msaggregate.BindingActive,
		},
	}
	text := FormatTrafficUsage(bindings)
	assert.Contains(t, text, "user1")
	assert.Contains(t, text, "5.0 GB")
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.expected, formatBytes(tc.bytes))
	}
}

func TestContainsAddon(t *testing.T) {
	addons := []string{"a", "b", "c"}
	assert.True(t, containsAddon(addons, "b"))
	assert.False(t, containsAddon(addons, "d"))
}

func TestRemoveAddon(t *testing.T) {
	addons := []string{"a", "b", "c"}
	result := removeAddon(addons, "b")
	assert.Equal(t, []string{"a", "c"}, result)

	// Removing non-existent addon returns all
	result2 := removeAddon(addons, "d")
	assert.Equal(t, []string{"a", "b", "c"}, result2)
}

func TestConstants(t *testing.T) {
	// Verify constants are defined as expected.
	assert.Equal(t, "start", CmdStart)
	assert.Equal(t, "plans", CmdPlans)
	assert.Equal(t, "subscribe", CmdSubscribe)
	assert.Equal(t, "my", CmdMy)
	assert.Equal(t, "traffic", CmdTraffic)
	assert.Equal(t, "support", CmdSupport)
	assert.Equal(t, "referral", CmdReferral)
	assert.Equal(t, "plan:", CallbackPrefixPlan)
	assert.Equal(t, "addon:", CallbackPrefixAddon)
	assert.Equal(t, "confirm:", CallbackPrefixConfirm)
	assert.Equal(t, "cancel:", CallbackPrefixCancel)
	assert.Equal(t, 4096, MaxMessageLength)
}
