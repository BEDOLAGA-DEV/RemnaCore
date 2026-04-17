package tariff

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validFixedTariff() TariffInput {
	input := defaultTariffInput()
	input.Name = "Basic VPN"
	input.PriceAmount = 999
	input.PriceCurrency = "USD"
	input.DurationDays = 30
	return input
}

func TestValidateByBillingModel_Fixed(t *testing.T) {
	input := validFixedTariff()
	assert.NoError(t, validateTariffInputV2(&input))
}

func TestValidateByBillingModel_FixedMissingDuration(t *testing.T) {
	input := validFixedTariff()
	input.DurationDays = 0
	assert.ErrorContains(t, validateTariffInputV2(&input), "duration_days required")
}

func TestValidateByBillingModel_Recurring(t *testing.T) {
	input := validFixedTariff()
	input.BillingModel = string(BillingModelRecurring)
	input.AutoRenewal = true
	assert.NoError(t, validateTariffInputV2(&input))
}

func TestValidateByBillingModel_Lifetime(t *testing.T) {
	input := validFixedTariff()
	input.BillingModel = string(BillingModelLifetime)
	input.DurationDays = 0
	assert.NoError(t, validateTariffInputV2(&input))
}

func TestValidateByBillingModel_LifetimeWithDuration(t *testing.T) {
	input := validFixedTariff()
	input.BillingModel = string(BillingModelLifetime)
	assert.ErrorContains(t, validateTariffInputV2(&input), "lifetime must have duration_days = 0")
}

func TestValidateByBillingModel_LifetimeWithAutoRenewal(t *testing.T) {
	input := validFixedTariff()
	input.BillingModel = string(BillingModelLifetime)
	input.DurationDays = 0
	input.AutoRenewal = true
	assert.ErrorContains(t, validateTariffInputV2(&input), "lifetime cannot auto-renew")
}

func TestValidateByBillingModel_PAYG(t *testing.T) {
	input := validFixedTariff()
	input.BillingModel = string(BillingModelPAYG)
	input.DurationDays = 0
	input.OveragePerGBCents = 100
	assert.NoError(t, validateTariffInputV2(&input))
}

func TestValidateByBillingModel_PAYGMissingOverage(t *testing.T) {
	input := validFixedTariff()
	input.BillingModel = string(BillingModelPAYG)
	input.DurationDays = 0
	assert.ErrorContains(t, validateTariffInputV2(&input), "payg requires overage_per_gb_cents > 0")
}

func TestValidateByBillingModel_Credit(t *testing.T) {
	input := validFixedTariff()
	input.BillingModel = string(BillingModelCredit)
	input.DurationDays = 0
	input.CreditCostPerDay = 50
	assert.NoError(t, validateTariffInputV2(&input))
}

func TestValidateByBillingModel_CreditPerGB(t *testing.T) {
	input := validFixedTariff()
	input.BillingModel = string(BillingModelCredit)
	input.DurationDays = 0
	input.CreditCostPerGB = 100
	assert.NoError(t, validateTariffInputV2(&input))
}

func TestValidateByBillingModel_CreditMissingCost(t *testing.T) {
	input := validFixedTariff()
	input.BillingModel = string(BillingModelCredit)
	input.DurationDays = 0
	assert.ErrorContains(t, validateTariffInputV2(&input), "credit model requires")
}

func TestValidateByBillingModel_Hybrid(t *testing.T) {
	input := validFixedTariff()
	input.BillingModel = string(BillingModelHybrid)
	input.IncludedTrafficGB = 50
	input.OveragePerGBCents = 100
	assert.NoError(t, validateTariffInputV2(&input))
}

func TestValidateByBillingModel_HybridMissingTraffic(t *testing.T) {
	input := validFixedTariff()
	input.BillingModel = string(BillingModelHybrid)
	input.OveragePerGBCents = 100
	assert.ErrorContains(t, validateTariffInputV2(&input), "hybrid requires included_traffic_gb > 0")
}

func TestValidateByBillingModel_HybridMissingOverage(t *testing.T) {
	input := validFixedTariff()
	input.BillingModel = string(BillingModelHybrid)
	input.IncludedTrafficGB = 50
	assert.ErrorContains(t, validateTariffInputV2(&input), "hybrid requires overage_per_gb_cents > 0")
}

func TestValidateByBillingModel_UnknownModel(t *testing.T) {
	input := validFixedTariff()
	input.BillingModel = "subscription"
	assert.ErrorContains(t, validateTariffInputV2(&input), "unsupported billing_model")
}

// --- Basic field validation ---

func TestValidateTariffInput_EmptyName(t *testing.T) {
	input := validFixedTariff()
	input.Name = ""
	assert.ErrorContains(t, validateTariffInputV2(&input), "name is required")
}

func TestValidateTariffInput_NegativePrice(t *testing.T) {
	input := validFixedTariff()
	input.PriceAmount = -1
	assert.ErrorContains(t, validateTariffInputV2(&input), "price_amount must be >= 0")
}

func TestValidateTariffInput_ZeroPrice(t *testing.T) {
	input := validFixedTariff()
	input.PriceAmount = 0
	assert.NoError(t, validateTariffInputV2(&input))
}

func TestValidateTariffInput_InvalidCurrency(t *testing.T) {
	input := validFixedTariff()
	input.PriceCurrency = "XYZ"
	assert.ErrorContains(t, validateTariffInputV2(&input), "unsupported price_currency")
}

func TestValidateTariffInput_CurrencyLength(t *testing.T) {
	input := validFixedTariff()
	input.PriceCurrency = "US"
	assert.ErrorContains(t, validateTariffInputV2(&input), "price_currency must be 3 characters")
}

func TestValidateTariffInput_DefaultsBillingModel(t *testing.T) {
	input := validFixedTariff()
	input.BillingModel = ""
	require.NoError(t, validateTariffInputV2(&input))
	assert.Equal(t, string(BillingModelFixed), input.BillingModel)
}

func TestValidateTariffInput_NilSliceDefaults(t *testing.T) {
	input := validFixedTariff()
	input.InternalSquadUUIDs = nil
	input.ExternalSquadUUIDs = nil
	input.Features = nil
	input.Tags = nil
	require.NoError(t, validateTariffInputV2(&input))
	assert.NotNil(t, input.InternalSquadUUIDs)
	assert.NotNil(t, input.ExternalSquadUUIDs)
	assert.NotNil(t, input.Features)
	assert.NotNil(t, input.Tags)
}

// --- Cross-field validation ---

func TestCrossField_TrialWithoutTarget(t *testing.T) {
	input := validFixedTariff()
	input.TrialDays = 7
	assert.ErrorContains(t, validateTariffInputV2(&input), "trial_to_plan_id required")
}

func TestCrossField_TrialWithTarget(t *testing.T) {
	input := validFixedTariff()
	input.TrialDays = 7
	input.TrialToPlanID = "plan_premium"
	assert.NoError(t, validateTariffInputV2(&input))
}

func TestCrossField_StockRemainingExceedsLimit(t *testing.T) {
	input := validFixedTariff()
	input.StockLimit = 100
	input.StockRemaining = 200
	assert.ErrorContains(t, validateTariffInputV2(&input), "stock_remaining cannot exceed stock_limit")
}

func TestCrossField_StockValid(t *testing.T) {
	input := validFixedTariff()
	input.StockLimit = 100
	input.StockRemaining = 50
	assert.NoError(t, validateTariffInputV2(&input))
}

func TestCrossField_StockUnlimited(t *testing.T) {
	input := validFixedTariff()
	input.StockLimit = 0
	input.StockRemaining = 999
	assert.NoError(t, validateTariffInputV2(&input))
}

func TestCrossField_InvalidCurrencyInMultiPrices(t *testing.T) {
	input := validFixedTariff()
	input.MultiCurrencyPrices = map[string]int64{"USD": 999, "ABC": 500}
	assert.ErrorContains(t, validateTariffInputV2(&input), "unsupported currency in multi_currency_prices")
}

func TestCrossField_ValidMultiPrices(t *testing.T) {
	input := validFixedTariff()
	input.MultiCurrencyPrices = map[string]int64{"USD": 999, "EUR": 899}
	assert.NoError(t, validateTariffInputV2(&input))
}

func TestCrossField_AvailableUntilBeforeFrom(t *testing.T) {
	input := validFixedTariff()
	input.AvailableFrom = "2026-06-01T00:00:00Z"
	input.AvailableUntil = "2026-01-01T00:00:00Z"
	assert.ErrorContains(t, validateTariffInputV2(&input), "available_until must be after available_from")
}

func TestCrossField_ValidDateRange(t *testing.T) {
	input := validFixedTariff()
	input.AvailableFrom = "2026-01-01T00:00:00Z"
	input.AvailableUntil = "2026-12-31T23:59:59Z"
	assert.NoError(t, validateTariffInputV2(&input))
}

func TestCrossField_MinSeatsExceedsMax(t *testing.T) {
	input := validFixedTariff()
	input.MinSeats = 10
	input.MaxSeats = 5
	assert.ErrorContains(t, validateTariffInputV2(&input), "min_seats cannot exceed max_seats")
}

func TestCrossField_ValidSeats(t *testing.T) {
	input := validFixedTariff()
	input.MinSeats = 5
	input.MaxSeats = 100
	assert.NoError(t, validateTariffInputV2(&input))
}

func TestCrossField_UnlimitedMaxSeats(t *testing.T) {
	input := validFixedTariff()
	input.MinSeats = 5
	input.MaxSeats = 0 // unlimited
	assert.NoError(t, validateTariffInputV2(&input))
}

func TestCrossField_InvalidAudienceSegment(t *testing.T) {
	input := validFixedTariff()
	input.AudienceSegment = "enterprise"
	assert.ErrorContains(t, validateTariffInputV2(&input), "unsupported audience_segment")
}

func TestCrossField_ValidAudienceSegments(t *testing.T) {
	for _, seg := range []string{AudienceAll, AudienceB2C, AudienceB2B, AudienceReseller} {
		t.Run(seg, func(t *testing.T) {
			input := validFixedTariff()
			input.AudienceSegment = seg
			assert.NoError(t, validateTariffInputV2(&input))
		})
	}
}

// --- All currencies ---

func TestAllValidCurrencies(t *testing.T) {
	for currency := range validCurrencies {
		t.Run(currency, func(t *testing.T) {
			input := validFixedTariff()
			input.PriceCurrency = currency
			assert.NoError(t, validateTariffInputV2(&input))
		})
	}
}
