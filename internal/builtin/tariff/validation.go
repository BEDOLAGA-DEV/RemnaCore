package tariff

import (
	"fmt"
	"time"
)

// validCurrencies is the set of supported ISO 4217 currency codes.
// This is the single source of truth — plugin.go derives its UI options from SupportedCurrencies().
var validCurrencies = map[string]bool{
	"USD": true, "EUR": true, "RUB": true, "GBP": true,
	"TRY": true, "UAH": true, "KZT": true,
}

// SupportedCurrencies returns the list of supported currency codes.
func SupportedCurrencies() []string {
	out := make([]string, 0, len(validCurrencies))
	for c := range validCurrencies {
		out = append(out, c)
	}
	return out
}

// validateTariffInputV2 performs expanded validation that replaces the old
// simple validateTariffInput. It covers basic required fields, nil-slice
// defaults, per-billing-model rules, and cross-field invariants.
func validateTariffInputV2(input *TariffInput) error {
	// Basic required fields.
	if input.Name == "" {
		return fmt.Errorf("name is required")
	}
	if input.PriceAmount < 0 {
		return fmt.Errorf("price_amount must be >= 0")
	}
	if len(input.PriceCurrency) != 3 {
		return fmt.Errorf("price_currency must be 3 characters")
	}
	if !validCurrencies[input.PriceCurrency] {
		return fmt.Errorf("unsupported price_currency: %s", input.PriceCurrency)
	}

	// Nil slice defaults.
	if input.InternalSquadUUIDs == nil {
		input.InternalSquadUUIDs = []string{}
	}
	if input.ExternalSquadUUIDs == nil {
		input.ExternalSquadUUIDs = []string{}
	}
	if input.Features == nil {
		input.Features = []string{}
	}
	if input.Tags == nil {
		input.Tags = []string{}
	}

	// Default billing model to "fixed" for backward compat.
	if input.BillingModel == "" {
		input.BillingModel = string(BillingModelFixed)
	}

	// Per-model validation.
	if err := validateByBillingModel(input); err != nil {
		return err
	}

	// Cross-field validation.
	if err := validateCrossFields(input); err != nil {
		return err
	}

	return nil
}

// validateByBillingModel enforces rules specific to each billing model.
func validateByBillingModel(input *TariffInput) error {
	model := BillingModel(input.BillingModel)
	if !model.IsValid() {
		return fmt.Errorf("unsupported billing_model: %s", input.BillingModel)
	}

	switch model {
	case BillingModelFixed, BillingModelRecurring:
		if input.DurationDays <= 0 {
			return fmt.Errorf("duration_days required for %s", input.BillingModel)
		}
	case BillingModelLifetime:
		if input.DurationDays != 0 {
			return fmt.Errorf("lifetime must have duration_days = 0")
		}
		if input.AutoRenewal {
			return fmt.Errorf("lifetime cannot auto-renew")
		}
	case BillingModelPAYG:
		if input.OveragePerGBCents <= 0 {
			return fmt.Errorf("payg requires overage_per_gb_cents > 0")
		}
	case BillingModelCredit:
		if input.CreditCostPerDay <= 0 && input.CreditCostPerGB <= 0 {
			return fmt.Errorf("credit model requires credit_cost_per_day or credit_cost_per_gb")
		}
	case BillingModelHybrid:
		if input.IncludedTrafficGB <= 0 {
			return fmt.Errorf("hybrid requires included_traffic_gb > 0")
		}
		if input.OveragePerGBCents <= 0 {
			return fmt.Errorf("hybrid requires overage_per_gb_cents > 0")
		}
	}
	return nil
}

// validateCrossFields enforces invariants that span multiple fields.
func validateCrossFields(input *TariffInput) error {
	// Trial validation.
	if input.TrialDays > 0 && input.TrialToPlanID == "" {
		return fmt.Errorf("trial_to_plan_id required when trial_days > 0")
	}

	// Stock validation.
	if input.StockLimit > 0 && input.StockRemaining > input.StockLimit {
		return fmt.Errorf("stock_remaining cannot exceed stock_limit")
	}

	// Multi-currency prices validation.
	for currency := range input.MultiCurrencyPrices {
		if !validCurrencies[currency] {
			return fmt.Errorf("unsupported currency in multi_currency_prices: %s", currency)
		}
	}

	// Date validation.
	if input.AvailableFrom != "" && input.AvailableUntil != "" {
		from, errFrom := time.Parse(time.RFC3339, input.AvailableFrom)
		until, errUntil := time.Parse(time.RFC3339, input.AvailableUntil)
		if errFrom == nil && errUntil == nil && until.Before(from) {
			return fmt.Errorf("available_until must be after available_from")
		}
	}

	// Seats validation (B2B).
	if input.MinSeats > 0 && input.MaxSeats > 0 && input.MinSeats > input.MaxSeats {
		return fmt.Errorf("min_seats cannot exceed max_seats")
	}

	// Audience segment validation.
	if input.AudienceSegment != "" {
		validSegments := map[string]bool{
			AudienceAll: true, AudienceB2C: true, AudienceB2B: true, AudienceReseller: true,
		}
		if !validSegments[input.AudienceSegment] {
			return fmt.Errorf("unsupported audience_segment: %s", input.AudienceSegment)
		}
	}

	// Enum-like field validation.
	if input.UpgradePolicy != "" {
		valid := map[string]bool{UpgradePolicyImmediate: true, UpgradePolicyNextPeriod: true, UpgradePolicyProrated: true}
		if !valid[input.UpgradePolicy] {
			return fmt.Errorf("unsupported upgrade_policy: %s", input.UpgradePolicy)
		}
	}
	if input.DowngradePolicy != "" {
		valid := map[string]bool{DowngradePolicyNextPeriod: true, DowngradePolicyImmediateRefund: true}
		if !valid[input.DowngradePolicy] {
			return fmt.Errorf("unsupported downgrade_policy: %s", input.DowngradePolicy)
		}
	}
	if input.CancellationPolicy != "" {
		valid := map[string]bool{CancellationPolicyImmediate: true, CancellationPolicyEndOfPeriod: true}
		if !valid[input.CancellationPolicy] {
			return fmt.Errorf("unsupported cancellation_policy: %s", input.CancellationPolicy)
		}
	}
	if input.PriceCharmStrategy != "" {
		valid := map[string]bool{CharmStrategyNone: true, CharmStrategy99: true, CharmStrategy95: true}
		if !valid[input.PriceCharmStrategy] {
			return fmt.Errorf("unsupported price_charm_strategy: %s", input.PriceCharmStrategy)
		}
	}
	if input.OverageRounding != "" {
		valid := map[string]bool{OverageRoundingGB: true, OverageRoundingMB: true, OverageRounding100MB: true}
		if !valid[input.OverageRounding] {
			return fmt.Errorf("unsupported overage_rounding: %s", input.OverageRounding)
		}
	}
	if input.TrafficResetStrategy != "" {
		valid := map[string]bool{
			TrafficResetNoReset: true, TrafficResetDay: true, TrafficResetWeek: true,
			TrafficResetMonth: true, TrafficResetMonthRolling: true,
		}
		if !valid[input.TrafficResetStrategy] {
			return fmt.Errorf("unsupported traffic_reset_strategy: %s", input.TrafficResetStrategy)
		}
	}

	return nil
}
