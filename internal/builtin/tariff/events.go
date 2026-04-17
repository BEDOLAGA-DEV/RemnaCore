package tariff

// Tariff lifecycle events.
const (
	EventTariffCreated       = "tariff.created"
	EventTariffUpdated       = "tariff.updated"
	EventTariffArchived      = "tariff.archived"
	EventTariffStockDepleted = "tariff.stock_depleted"
	EventTariffPublished     = "tariff.published"
)

// Pricing events.
const (
	EventPricingRuleApplied   = "pricing.rule_applied"
	EventPricingModifierFired = "pricing.modifier_fired"
)

// Promo code events.
const (
	EventPromoCodeCreated     = "promo.code_created"
	EventPromoCodeUsed        = "promo.code_used"
	EventPromoCodeExhausted   = "promo.code_exhausted"
	EventPromoCampaignStarted = "promo.campaign_started"
	EventPromoCampaignEnded   = "promo.campaign_ended"
)

// A/B experiment events.
const (
	EventABExperimentStarted   = "ab.experiment_started"
	EventABVariantAssigned     = "ab.variant_assigned"
	EventABExperimentConcluded = "ab.experiment_concluded"
)

// Cohort events.
const (
	EventCohortUserAdded    = "cohort.user_added"
	EventCohortUserRemoved  = "cohort.user_removed"
	EventCohortRecalculated = "cohort.recalculated"
)

// Sync hook names — for use with hook dispatcher.
const (
	HookCalculatePrice      = "tariff.calculate_price"
	HookValidateEligibility = "tariff.validate_eligibility"
	HookListVisible         = "tariff.list_visible"
	HookPromoValidate       = "promo.validate"
	HookPromoApply          = "promo.apply"
	HookUserResolveGroups   = "user.resolve_groups"
	HookUserResolveTier     = "user.resolve_tier"
)

// Async hook names.
const (
	HookTariffPurchased   = "tariff.purchased"
	HookTariffExpired     = "tariff.expired"
	HookPromoCodeUsed     = "promo.code_used"
	HookCohortUserAdded   = "cohort.user_added"
	HookABVariantAssigned = "ab.variant_assigned"
)

// TariffPublishedPayload is emitted when a tariff is created or updated,
// to sync with the billing domain's Plan aggregate.
type TariffPublishedPayload struct {
	TariffID      string            `json:"tariff_id"`
	BillingModel  string            `json:"billing_model"`
	PriceAmount   int64             `json:"price_amount"`
	Currency      string            `json:"currency"`
	DurationDays  int               `json:"duration_days"`
	Version       int               `json:"version"`
	PricingRuleID string            `json:"pricing_rule_id,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// TariffArchivedPayload is emitted when a tariff is archived (soft-deleted).
type TariffArchivedPayload struct {
	TariffID   string `json:"tariff_id"`
	ArchivedAt string `json:"archived_at"` // RFC3339
	Reason     string `json:"reason"`      // "admin_action" | "stock_depleted" | "sunset"
}

// TariffPurchasedPayload is emitted asynchronously when a tariff is purchased.
type TariffPurchasedPayload struct {
	TariffID       string           `json:"tariff_id"`
	UserID         string           `json:"user_id"`
	PriceBreakdown map[string]int64 `json:"price_breakdown"`
	AppliedRules   []string         `json:"applied_rules,omitempty"`
	ABExperiment   string           `json:"ab_experiment,omitempty"`
	ABVariant      string           `json:"ab_variant,omitempty"`
	Cohort         string           `json:"cohort,omitempty"`
	PromoCode      string           `json:"promo_code,omitempty"`
}

// PromoCodeUsedPayload is emitted when a promo code is successfully used.
type PromoCodeUsedPayload struct {
	PromoCodeID   string `json:"promo_code_id"`
	Code          string `json:"code"`
	UserID        string `json:"user_id"`
	TariffID      string `json:"tariff_id"`
	DiscountCents int64  `json:"discount_cents"`
}

// ABVariantAssignedPayload is emitted for observability when a user is
// assigned to an experiment variant.
type ABVariantAssignedPayload struct {
	ExperimentID string `json:"experiment_id"`
	UserID       string `json:"user_id"`
	VariantKey   string `json:"variant_key"`
	PriceCents   int64  `json:"price_cents"`
}
