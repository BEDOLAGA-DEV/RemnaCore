package service_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	billingaggregate "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/service"
)

func newTestPlan(t *testing.T) *billingaggregate.Plan {
	t.Helper()
	plan, err := billingaggregate.NewPlan(
		"Premium VPN",
		"Premium plan",
		vo.Money{Amount: 999, Currency: "USD"},
		vo.IntervalMonth,
		100_000_000_000, // 100 GB
		5,
		[]string{"US", "DE"},
		[]string{"wireguard"},
		billingaggregate.TierPremium,
		4,
		true,
		3,
	)
	require.NoError(t, err)
	return plan
}

func TestCalculate_BaseOnly(t *testing.T) {
	calc := service.NewBindingCalculator()
	plan := newTestPlan(t)

	specs := calc.Calculate(plan, nil, nil)

	require.Len(t, specs, 1)
	assert.Equal(t, aggregate.PurposeBase, specs[0].Purpose)
	assert.Equal(t, int64(100_000_000_000), specs[0].TrafficLimit)
}

func TestCalculate_WithGamingAddon(t *testing.T) {
	calc := service.NewBindingCalculator()
	plan := newTestPlan(t)
	_ = plan.AddAddon(billingaggregate.Addon{
		ID:                "addon-gaming",
		Name:              "gaming",
		Price:             vo.Money{Amount: 499, Currency: "USD"},
		Type:              billingaggregate.AddonNodes,
		ExtraTrafficBytes: 50_000_000_000,
		ExtraNodes:        []string{"node-gaming-us", "node-gaming-eu"},
	})

	specs := calc.Calculate(plan, []string{"addon-gaming"}, nil)

	require.Len(t, specs, 2)

	assert.Equal(t, aggregate.PurposeBase, specs[0].Purpose)
	assert.Equal(t, int64(100_000_000_000), specs[0].TrafficLimit)

	assert.Equal(t, aggregate.PurposeGaming, specs[1].Purpose)
	assert.Equal(t, int64(50_000_000_000), specs[1].TrafficLimit)
	assert.Equal(t, []string{"node-gaming-us", "node-gaming-eu"}, specs[1].AllowedNodes)
}

func TestCalculate_WithFamilyMembers(t *testing.T) {
	calc := service.NewBindingCalculator()
	plan := newTestPlan(t)

	specs := calc.Calculate(plan, nil, []string{"family-1", "family-2"})

	require.Len(t, specs, 3) // base + 2 family
	assert.Equal(t, aggregate.PurposeBase, specs[0].Purpose)
	assert.Equal(t, aggregate.PurposeFamilyMember, specs[1].Purpose)
	assert.Equal(t, aggregate.PurposeFamilyMember, specs[2].Purpose)
	assert.Equal(t, int64(100_000_000_000), specs[1].TrafficLimit)
	assert.Equal(t, int64(100_000_000_000), specs[2].TrafficLimit)
}

func TestCalculate_WithTrafficAddon(t *testing.T) {
	calc := service.NewBindingCalculator()
	plan := newTestPlan(t)
	_ = plan.AddAddon(billingaggregate.Addon{
		ID:                "addon-extra-traffic",
		Name:              "extra-traffic",
		Price:             vo.Money{Amount: 299, Currency: "USD"},
		Type:              billingaggregate.AddonTraffic,
		ExtraTrafficBytes: 50_000_000_000,
	})

	specs := calc.Calculate(plan, []string{"addon-extra-traffic"}, nil)

	require.Len(t, specs, 1) // only base, with increased traffic
	assert.Equal(t, aggregate.PurposeBase, specs[0].Purpose)
	assert.Equal(t, int64(150_000_000_000), specs[0].TrafficLimit) // 100 + 50
}

func TestCalculate_EmptyPlan(t *testing.T) {
	calc := service.NewBindingCalculator()
	plan := newTestPlan(t)
	plan.TrafficLimitBytes = 0 // unlimited

	specs := calc.Calculate(plan, nil, nil)

	require.Len(t, specs, 1)
	assert.Equal(t, aggregate.PurposeBase, specs[0].Purpose)
	assert.Equal(t, int64(0), specs[0].TrafficLimit)
}

func TestCalculate_UnknownAddonIgnored(t *testing.T) {
	calc := service.NewBindingCalculator()
	plan := newTestPlan(t)

	specs := calc.Calculate(plan, []string{"nonexistent-addon"}, nil)

	require.Len(t, specs, 1) // only base
	assert.Equal(t, aggregate.PurposeBase, specs[0].Purpose)
}

func TestCalculate_CombinedScenario(t *testing.T) {
	calc := service.NewBindingCalculator()
	plan := newTestPlan(t)
	_ = plan.AddAddon(billingaggregate.Addon{
		ID:                "addon-gaming",
		Name:              "gaming",
		Price:             vo.Money{Amount: 499, Currency: "USD"},
		Type:              billingaggregate.AddonNodes,
		ExtraTrafficBytes: 50_000_000_000,
		ExtraNodes:        []string{"node-gaming-us"},
	})
	_ = plan.AddAddon(billingaggregate.Addon{
		ID:                "addon-streaming",
		Name:              "streaming",
		Price:             vo.Money{Amount: 399, Currency: "USD"},
		Type:              billingaggregate.AddonNodes,
		ExtraTrafficBytes: 80_000_000_000,
		ExtraNodes:        []string{"node-stream-us"},
	})
	_ = plan.AddAddon(billingaggregate.Addon{
		ID:                "addon-extra-traffic",
		Name:              "extra-traffic",
		Price:             vo.Money{Amount: 299, Currency: "USD"},
		Type:              billingaggregate.AddonTraffic,
		ExtraTrafficBytes: 25_000_000_000,
	})

	specs := calc.Calculate(
		plan,
		[]string{"addon-gaming", "addon-streaming", "addon-extra-traffic"},
		[]string{"family-1"},
	)

	// base (traffic boosted) + gaming + streaming + 1 family = 4
	require.Len(t, specs, 4)
	assert.Equal(t, aggregate.PurposeBase, specs[0].Purpose)
	assert.Equal(t, int64(125_000_000_000), specs[0].TrafficLimit) // 100 + 25
	assert.Equal(t, aggregate.PurposeGaming, specs[1].Purpose)
	assert.Equal(t, aggregate.PurposeStreaming, specs[2].Purpose)
	assert.Equal(t, aggregate.PurposeFamilyMember, specs[3].Purpose)
}
