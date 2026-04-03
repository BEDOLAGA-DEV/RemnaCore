package aggregate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
)

func validPlanParams() (string, string, vo.Money, vo.BillingInterval, int64, int, []string, []string, PlanTier, int, bool, int) {
	return "Premium VPN",
		"High-speed VPN with global coverage",
		vo.NewMoney(999, vo.CurrencyUSD),
		vo.IntervalMonth,
		int64(100 * 1024 * 1024 * 1024), // 100 GB
		5,
		[]string{"US", "DE", "JP"},
		[]string{"wireguard", "openvpn"},
		TierPremium,
		3,
		false,
		0
}

func TestNewPlan_Valid(t *testing.T) {
	name, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily := validPlanParams()

	plan, err := NewPlan(name, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily)

	require.NoError(t, err)
	assert.NotEmpty(t, plan.ID)
	assert.Equal(t, name, plan.Name)
	assert.Equal(t, desc, plan.Description)
	assert.Equal(t, price, plan.BasePrice)
	assert.Equal(t, interval, plan.Interval)
	assert.Equal(t, traffic, plan.TrafficLimitBytes)
	assert.Equal(t, devices, plan.DeviceLimit)
	assert.Equal(t, countries, plan.AllowedCountries)
	assert.Equal(t, protocols, plan.AllowedProtocols)
	assert.Equal(t, tier, plan.Tier)
	assert.Equal(t, maxBindings, plan.MaxRemnawaveBindings)
	assert.False(t, plan.FamilyEnabled)
	assert.Equal(t, 0, plan.MaxFamilyMembers)
	assert.True(t, plan.IsActive)
	assert.False(t, plan.CreatedAt.IsZero())
	assert.False(t, plan.UpdatedAt.IsZero())
}

func TestNewPlan_EmptyName(t *testing.T) {
	_, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily := validPlanParams()

	_, err := NewPlan("", desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily)

	require.Error(t, err)
	assert.ErrorContains(t, err, "name")
}

func TestNewPlan_ZeroPrice(t *testing.T) {
	name, desc, _, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily := validPlanParams()
	zeroPrice := vo.NewMoney(0, vo.CurrencyUSD)

	_, err := NewPlan(name, desc, zeroPrice, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily)

	require.Error(t, err)
	assert.ErrorContains(t, err, "price")
}

func TestNewPlan_NegativePrice(t *testing.T) {
	name, desc, _, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily := validPlanParams()
	negativePrice := vo.NewMoney(-100, vo.CurrencyUSD)

	_, err := NewPlan(name, desc, negativePrice, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily)

	require.Error(t, err)
	assert.ErrorContains(t, err, "price")
}

func TestNewPlan_NoCountries(t *testing.T) {
	name, desc, price, interval, traffic, devices, _, protocols, tier, maxBindings, familyEnabled, maxFamily := validPlanParams()

	_, err := NewPlan(name, desc, price, interval, traffic, devices, nil, protocols, tier, maxBindings, familyEnabled, maxFamily)

	require.Error(t, err)
	assert.ErrorContains(t, err, "country")
}

func TestNewPlan_EmptyCountries(t *testing.T) {
	name, desc, price, interval, traffic, devices, _, protocols, tier, maxBindings, familyEnabled, maxFamily := validPlanParams()

	_, err := NewPlan(name, desc, price, interval, traffic, devices, []string{}, protocols, tier, maxBindings, familyEnabled, maxFamily)

	require.Error(t, err)
	assert.ErrorContains(t, err, "country")
}

func TestNewPlan_FamilyDisabledButMaxFamilyPositive(t *testing.T) {
	name, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, _, _ := validPlanParams()

	_, err := NewPlan(name, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, false, 5)

	require.Error(t, err)
	assert.ErrorContains(t, err, "family")
}

func TestNewPlan_FamilyEnabled(t *testing.T) {
	name, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, _, _ := validPlanParams()

	plan, err := NewPlan(name, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, true, 5)

	require.NoError(t, err)
	assert.True(t, plan.FamilyEnabled)
	assert.Equal(t, 5, plan.MaxFamilyMembers)
}

func TestPlan_AddAddon(t *testing.T) {
	name, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily := validPlanParams()
	plan, err := NewPlan(name, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily)
	require.NoError(t, err)

	addon := Addon{
		ID:                "addon-1",
		Name:              "Extra Traffic",
		Price:             vo.NewMoney(299, vo.CurrencyUSD),
		Type:              AddonTraffic,
		ExtraTrafficBytes: 50 * 1024 * 1024 * 1024,
	}

	err = plan.AddAddon(addon)

	require.NoError(t, err)
	assert.Len(t, plan.AvailableAddons, 1)
	assert.Equal(t, "addon-1", plan.AvailableAddons[0].ID)
}

func TestPlan_AddAddon_Duplicate(t *testing.T) {
	name, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily := validPlanParams()
	plan, err := NewPlan(name, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily)
	require.NoError(t, err)

	addon := Addon{
		ID:    "addon-1",
		Name:  "Extra Traffic",
		Price: vo.NewMoney(299, vo.CurrencyUSD),
		Type:  AddonTraffic,
	}

	err = plan.AddAddon(addon)
	require.NoError(t, err)

	err = plan.AddAddon(addon)
	require.Error(t, err)
	assert.ErrorContains(t, err, "already exists")
}

func TestPlan_HasAddon(t *testing.T) {
	name, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily := validPlanParams()
	plan, err := NewPlan(name, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily)
	require.NoError(t, err)

	addon := Addon{
		ID:    "addon-1",
		Name:  "Extra Traffic",
		Price: vo.NewMoney(299, vo.CurrencyUSD),
		Type:  AddonTraffic,
	}

	assert.False(t, plan.HasAddon("addon-1"))

	err = plan.AddAddon(addon)
	require.NoError(t, err)

	assert.True(t, plan.HasAddon("addon-1"))
	assert.False(t, plan.HasAddon("addon-999"))
}

func TestPlan_CalculateTotal_BaseOnly(t *testing.T) {
	name, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily := validPlanParams()
	plan, err := NewPlan(name, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily)
	require.NoError(t, err)

	total, err := plan.CalculateTotal(nil)

	require.NoError(t, err)
	assert.Equal(t, price.Amount, total.Amount)
	assert.Equal(t, price.Currency, total.Currency)
}

func TestPlan_CalculateTotal_WithAddons(t *testing.T) {
	name, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily := validPlanParams()
	plan, err := NewPlan(name, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily)
	require.NoError(t, err)

	addon1 := Addon{
		ID:    "addon-1",
		Name:  "Extra Traffic",
		Price: vo.NewMoney(299, vo.CurrencyUSD),
		Type:  AddonTraffic,
	}
	addon2 := Addon{
		ID:    "addon-2",
		Name:  "Extra Nodes",
		Price: vo.NewMoney(499, vo.CurrencyUSD),
		Type:  AddonNodes,
	}

	require.NoError(t, plan.AddAddon(addon1))
	require.NoError(t, plan.AddAddon(addon2))

	total, err := plan.CalculateTotal([]string{"addon-1", "addon-2"})

	require.NoError(t, err)
	// 999 + 299 + 499 = 1797
	assert.Equal(t, int64(1797), total.Amount)
}

func TestPlan_CalculateTotal_AddonNotFound(t *testing.T) {
	name, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily := validPlanParams()
	plan, err := NewPlan(name, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily)
	require.NoError(t, err)

	_, err = plan.CalculateTotal([]string{"nonexistent"})

	require.Error(t, err)
	assert.ErrorContains(t, err, "addon")
}

func TestPlan_CalculateTotal_PartialAddons(t *testing.T) {
	name, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily := validPlanParams()
	plan, err := NewPlan(name, desc, price, interval, traffic, devices, countries, protocols, tier, maxBindings, familyEnabled, maxFamily)
	require.NoError(t, err)

	addon1 := Addon{
		ID:    "addon-1",
		Name:  "Extra Traffic",
		Price: vo.NewMoney(299, vo.CurrencyUSD),
		Type:  AddonTraffic,
	}
	addon2 := Addon{
		ID:    "addon-2",
		Name:  "Extra Nodes",
		Price: vo.NewMoney(499, vo.CurrencyUSD),
		Type:  AddonNodes,
	}

	require.NoError(t, plan.AddAddon(addon1))
	require.NoError(t, plan.AddAddon(addon2))

	total, err := plan.CalculateTotal([]string{"addon-1"})

	require.NoError(t, err)
	// 999 + 299 = 1298
	assert.Equal(t, int64(1298), total.Amount)
}
