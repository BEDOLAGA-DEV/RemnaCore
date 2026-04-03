package service_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/service"
)

func newTestPlanSnapshot() multisub.PlanSnapshot {
	return multisub.PlanSnapshot{
		ID:                   "plan-premium",
		TrafficLimitBytes:    100_000_000_000, // 100 GB
		MaxRemnawaveBindings: 4,
	}
}

func TestCalculate_BaseOnly(t *testing.T) {
	calc := service.NewBindingCalculator()
	plan := newTestPlanSnapshot()

	specs := calc.Calculate(plan, nil, nil)

	require.Len(t, specs, 1)
	assert.Equal(t, aggregate.PurposeBase, specs[0].Purpose)
	assert.Equal(t, int64(100_000_000_000), specs[0].TrafficLimit)
}

func TestCalculate_WithGamingAddon(t *testing.T) {
	calc := service.NewBindingCalculator()
	plan := newTestPlanSnapshot()
	plan.Addons = []multisub.AddonSnapshot{
		{
			ID:                "addon-gaming",
			Name:              "gaming",
			Type:              multisub.AddonSnapshotNodes,
			ExtraTrafficBytes: 50_000_000_000,
			ExtraNodes:        []string{"node-gaming-us", "node-gaming-eu"},
		},
	}

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
	plan := newTestPlanSnapshot()

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
	plan := newTestPlanSnapshot()
	plan.Addons = []multisub.AddonSnapshot{
		{
			ID:                "addon-extra-traffic",
			Name:              "extra-traffic",
			Type:              multisub.AddonSnapshotTraffic,
			ExtraTrafficBytes: 50_000_000_000,
		},
	}

	specs := calc.Calculate(plan, []string{"addon-extra-traffic"}, nil)

	require.Len(t, specs, 1) // only base, with increased traffic
	assert.Equal(t, aggregate.PurposeBase, specs[0].Purpose)
	assert.Equal(t, int64(150_000_000_000), specs[0].TrafficLimit) // 100 + 50
}

func TestCalculate_EmptyPlan(t *testing.T) {
	calc := service.NewBindingCalculator()
	plan := newTestPlanSnapshot()
	plan.TrafficLimitBytes = 0 // unlimited

	specs := calc.Calculate(plan, nil, nil)

	require.Len(t, specs, 1)
	assert.Equal(t, aggregate.PurposeBase, specs[0].Purpose)
	assert.Equal(t, int64(0), specs[0].TrafficLimit)
}

func TestCalculate_UnknownAddonIgnored(t *testing.T) {
	calc := service.NewBindingCalculator()
	plan := newTestPlanSnapshot()

	specs := calc.Calculate(plan, []string{"nonexistent-addon"}, nil)

	require.Len(t, specs, 1) // only base
	assert.Equal(t, aggregate.PurposeBase, specs[0].Purpose)
}

func TestCalculate_CombinedScenario(t *testing.T) {
	calc := service.NewBindingCalculator()
	plan := newTestPlanSnapshot()
	plan.Addons = []multisub.AddonSnapshot{
		{
			ID:                "addon-gaming",
			Name:              "gaming",
			Type:              multisub.AddonSnapshotNodes,
			ExtraTrafficBytes: 50_000_000_000,
			ExtraNodes:        []string{"node-gaming-us"},
		},
		{
			ID:                "addon-streaming",
			Name:              "streaming",
			Type:              multisub.AddonSnapshotNodes,
			ExtraTrafficBytes: 80_000_000_000,
			ExtraNodes:        []string{"node-stream-us"},
		},
		{
			ID:                "addon-extra-traffic",
			Name:              "extra-traffic",
			Type:              multisub.AddonSnapshotTraffic,
			ExtraTrafficBytes: 25_000_000_000,
		},
	}

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
