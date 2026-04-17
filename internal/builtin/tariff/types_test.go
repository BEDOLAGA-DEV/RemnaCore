package tariff

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBillingModel_IsValid(t *testing.T) {
	tests := []struct {
		model BillingModel
		valid bool
	}{
		{BillingModelFixed, true},
		{BillingModelRecurring, true},
		{BillingModelLifetime, true},
		{BillingModelPAYG, true},
		{BillingModelCredit, true},
		{BillingModelHybrid, true},
		{"unknown", false},
		{"", false},
		{"FIXED", false}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.model), func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.model.IsValid())
		})
	}
}

func TestDefaultTariffInput(t *testing.T) {
	d := defaultTariffInput()

	assert.Equal(t, string(BillingModelFixed), d.BillingModel)
	assert.Equal(t, AudienceAll, d.AudienceSegment)
	assert.Equal(t, UpgradePolicyNextPeriod, d.UpgradePolicy)
	assert.Equal(t, DowngradePolicyNextPeriod, d.DowngradePolicy)
	assert.Equal(t, CancellationPolicyEndOfPeriod, d.CancellationPolicy)
	assert.Equal(t, CharmStrategyNone, d.PriceCharmStrategy)
	assert.Equal(t, OverageRoundingGB, d.OverageRounding)
	assert.True(t, d.VisibleInPublic)
	assert.True(t, d.VisibleInTelegram)
	assert.True(t, d.VisibleInCabinet)
	assert.False(t, d.VisibleInResellerPortal)
}
