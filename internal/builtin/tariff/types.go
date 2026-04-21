package tariff

// BillingModel defines how a tariff charges the customer.
type BillingModel string

const (
	BillingModelFixed     BillingModel = "fixed"
	BillingModelRecurring BillingModel = "recurring"
	BillingModelLifetime  BillingModel = "lifetime"
	BillingModelPAYG      BillingModel = "payg"
	BillingModelCredit    BillingModel = "credit"
	BillingModelHybrid    BillingModel = "hybrid"
)

// IsValid returns true if b is one of the known billing models.
func (b BillingModel) IsValid() bool {
	switch b {
	case BillingModelFixed,
		BillingModelRecurring,
		BillingModelLifetime,
		BillingModelPAYG,
		BillingModelCredit,
		BillingModelHybrid:
		return true
	default:
		return false
	}
}

// --- Collection name constants ---

const (
	CollectionPricingRules    = "pricing_rules"
	CollectionPromoCodes      = "promo_codes"
	CollectionABExperiments   = "ab_experiments"
	CollectionCohorts         = "cohorts"
	CollectionAudienceRules   = "audience_rules"
	CollectionPurchaseHistory = "purchase_history"
)

// --- Audience segment constants ---

const (
	AudienceAll      = "all"
	AudienceB2C      = "b2c"
	AudienceB2B      = "b2b"
	AudienceReseller = "reseller"
)

// --- Policy constants ---

const (
	UpgradePolicyImmediate  = "immediate"
	UpgradePolicyNextPeriod = "next_period"
	UpgradePolicyProrated   = "prorated"

	DowngradePolicyNextPeriod      = "next_period"
	DowngradePolicyImmediateRefund = "immediate_refund"

	CancellationPolicyImmediate   = "immediate"
	CancellationPolicyEndOfPeriod = "end_of_period"

	CharmStrategyNone = "none"
	CharmStrategy99   = "99"
	CharmStrategy95   = "95"

	OverageRoundingGB    = "gb"
	OverageRoundingMB    = "mb"
	OverageRounding100MB = "100mb"
)

// EligibilityRules captures constraints that determine whether a user
// is eligible to purchase a tariff.
type EligibilityRules struct {
	RequirePromoGroups   []string `json:"require_promo_groups,omitempty"`
	RequireAnyOfGroups   []string `json:"require_any_of_groups,omitempty"`
	ExcludeGroups        []string `json:"exclude_groups,omitempty"`
	RequireTiers         []string `json:"require_tiers,omitempty"`
	MinLifetimeMonths    int      `json:"min_lifetime_months,omitempty"`
	RequireVerifiedEmail bool     `json:"require_verified_email,omitempty"`
	RequireKYC           bool     `json:"require_kyc,omitempty"`
	CustomRuleIDs        []string `json:"custom_rule_ids,omitempty"`
}

// TariffInput is the full set of fields accepted when creating or
// updating a tariff. The first 13 fields match the original schema;
// the remaining fields extend it with billing, pricing, audience,
// trial, lifecycle, visibility, reseller, localisation, tax, and
// metadata capabilities.
type TariffInput struct {
	// --- Core (original 13 fields) ---
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	PriceAmount         int64    `json:"price_amount"`
	PriceCurrency       string   `json:"price_currency"`
	DurationDays        int      `json:"duration_days"`
	TrafficLimitGB      float64  `json:"traffic_limit_gb"`
	DeviceLimit         int      `json:"device_limit"`
	MaxPurchasesPerUser int      `json:"max_purchases_per_user"`
	VPNPanelID          string   `json:"vpn_panel_id,omitempty"`
	InternalSquadUUIDs  []string `json:"internal_squad_uuids"`
	ExternalSquadUUIDs  []string `json:"external_squad_uuids"`
	Features            []string `json:"features"`
	IsActive            bool     `json:"is_active"`
	SortOrder           int      `json:"sort_order"`

	// --- Billing model ---
	BillingModel     string `json:"billing_model"`
	AutoRenewal      bool   `json:"auto_renewal"`
	RenewalGraceDays int    `json:"renewal_grace_days"`

	// --- Pricing ---
	PricingRuleID       string           `json:"pricing_rule_id"`
	MultiCurrencyPrices map[string]int64 `json:"multi_currency_prices,omitempty"`
	PriceCharmStrategy  string           `json:"price_charm_strategy"`

	// --- PAYG / Hybrid ---
	IncludedTrafficGB float64 `json:"included_traffic_gb"`
	OveragePerGBCents int64   `json:"overage_per_gb_cents"`
	OverageRounding   string  `json:"overage_rounding"`

	// --- Credit model ---
	CreditCostPerDay int64 `json:"credit_cost_per_day"`
	CreditCostPerGB  int64 `json:"credit_cost_per_gb"`

	// --- Audience ---
	AudienceSegment string `json:"audience_segment"`
	MinSeats        int    `json:"min_seats"`
	MaxSeats        int    `json:"max_seats"`
	RequiresKYC     bool   `json:"requires_kyc"`

	// --- Trial & conversion ---
	TrialDays         int    `json:"trial_days"`
	TrialRequiresCard bool   `json:"trial_requires_card"`
	TrialToPlanID     string `json:"trial_to_plan_id"`

	// --- Lifecycle rules ---
	UpgradePolicy      string `json:"upgrade_policy"`
	DowngradePolicy    string `json:"downgrade_policy"`
	CancellationPolicy string `json:"cancellation_policy"`
	RefundWindowDays   int    `json:"refund_window_days"`

	// --- Purchase constraints ---
	AvailableFrom      string           `json:"available_from,omitempty"`
	AvailableUntil     string           `json:"available_until,omitempty"`
	StockLimit         int              `json:"stock_limit"`
	StockRemaining     int              `json:"stock_remaining"`
	RequiresInviteCode bool             `json:"requires_invite_code"`
	EligibilityRules   EligibilityRules `json:"eligibility_rules"`

	// --- Visibility ---
	VisibleInPublic         bool `json:"visible_in_public"`
	VisibleInTelegram       bool `json:"visible_in_telegram"`
	VisibleInCabinet        bool `json:"visible_in_cabinet"`
	VisibleInResellerPortal bool `json:"visible_in_reseller_portal"`

	// --- Reseller ---
	ResellerMarkupPct   int64 `json:"reseller_markup_pct"`
	ResellerMinPrice    int64 `json:"reseller_min_price"`
	ResellerMaxDiscount int64 `json:"reseller_max_discount"`

	// --- Localization ---
	LocalizedNames        map[string]string `json:"localized_names,omitempty"`
	LocalizedDescriptions map[string]string `json:"localized_descriptions,omitempty"`

	// --- Tax ---
	TaxCategory  string `json:"tax_category"`
	TaxInclusive bool   `json:"tax_inclusive"`

	// --- Metadata ---
	Tags         []string          `json:"tags,omitempty"`
	BadgeText    string            `json:"badge_text"`
	BadgeColor   string            `json:"badge_color"`
	ExternalRefs map[string]string `json:"external_refs,omitempty"`
}

// TariffResponse wraps TariffInput with server-assigned identity and
// timestamps.
type TariffResponse struct {
	ID string `json:"id"`
	TariffInput
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// defaultTariffInput returns a TariffInput populated with sensible
// defaults so that callers upgrading from the original 13-field schema
// get backward-compatible behaviour without setting every new field.
func defaultTariffInput() TariffInput {
	return TariffInput{
		BillingModel:       string(BillingModelFixed),
		AudienceSegment:    AudienceAll,
		UpgradePolicy:      UpgradePolicyNextPeriod,
		DowngradePolicy:    DowngradePolicyNextPeriod,
		CancellationPolicy: CancellationPolicyEndOfPeriod,
		PriceCharmStrategy: CharmStrategyNone,
		OverageRounding:    OverageRoundingGB,
		VisibleInPublic:    true,
		VisibleInTelegram:  true,
		VisibleInCabinet:   true,
	}
}
