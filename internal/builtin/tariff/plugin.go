package tariff

import (
	"net/http"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
)

func init() {
	builtin.RegisterPlugin(Plugin())
}

// Plugin returns the built-in plugin definition for the tariff-manager.
func Plugin() plugin.BuiltInPluginDef {
	currencies := SupportedCurrencies()
	billingModels := []string{"fixed", "recurring", "lifetime", "payg", "credit", "hybrid"}
	audienceSegments := []string{"all", "b2c", "b2b", "reseller"}

	return plugin.BuiltInPluginDef{
		Slug:        PluginSlug,
		Name:        "Tariff Manager",
		Version:     "2.0.0",
		Description: "B2C + B2B + Reseller tariff management. Supports recurring, lifetime, PAYG, credit, and hybrid billing models with pricing engine, promo codes, A/B experiments, cohort pricing, and analytics.",
		Author:      "RemnaCore",
		ConfigFields: map[string]plugin.ManifestConfigField{
			"default_currency": {
				Type:     "select",
				Label:    "Default Currency",
				Required: false,
				Default:  "USD",
				Options:  currencies,
			},
			"eligibility_strict_mode": {
				Type:     "boolean",
				Label:    "Strict Eligibility Mode",
				Required: false,
				Default:  "false",
			},
		},
		Pages: []plugin.ManifestPage{
			// --- Page 1: Tariffs ---
			{
				Path:       "tariffs",
				Title:      "Tariffs",
				Icon:       "CreditCard",
				Menu:       plugin.PageMenuAdmin,
				Collection: CollectionName,
				Fields: []plugin.ManifestPageField{
					// Basic
					{Key: "name", Label: "Name", Type: "text", Required: true},
					{Key: "description", Label: "Description", Type: "textarea"},
					{Key: "billing_model", Label: "Billing Model", Type: "select", Required: true, Default: "fixed", Options: billingModels},
					{Key: "price_amount", Label: "Price (cents)", Type: "number", Required: true, Default: "0"},
					{Key: "price_currency", Label: "Currency", Type: "select", Required: true, Default: "USD", Options: currencies},
					{Key: "duration_days", Label: "Duration (days)", Type: "number", Default: "30"},
					{Key: "traffic_limit_gb", Label: "Traffic (GB, 0=unlimited)", Type: "number", Default: "0"},
					{Key: "device_limit", Label: "Devices (0=unlimited)", Type: "number", Default: "0"},
					{Key: "max_purchases_per_user", Label: "Max per user (0=unlimited)", Type: "number", Default: "0"},
					// VPN Panel
					{Key: "vpn_panel_id", Label: "VPN Panel", Type: "select", OptionsURL: "/api/tariffs/panels", OptionsValueKey: "id", OptionsLabelKey: "name"},
					// Squads (loaded from selected panel)
					{Key: "internal_squad_uuids", Label: "Internal Squads", Type: "multiselect", OptionsURL: "/api/tariffs/internal-squads", OptionsValueKey: "uuid", OptionsLabelKey: "name"},
					{Key: "external_squad_uuids", Label: "External Squads", Type: "multiselect", OptionsURL: "/api/tariffs/external-squads", OptionsValueKey: "uuid", OptionsLabelKey: "name"},
					{Key: "features", Label: "Features (one per line)", Type: "textarea"},
					// Pricing
					{Key: "pricing_rule_id", Label: "Pricing Rule", Type: "text"},
					{Key: "price_charm_strategy", Label: "Charm Pricing", Type: "select", Default: "none", Options: []string{"none", "99", "95"}},
					// Audience
					{Key: "audience_segment", Label: "Audience", Type: "select", Default: "all", Options: audienceSegments},
					{Key: "min_seats", Label: "Min Seats (B2B)", Type: "number", Default: "0"},
					{Key: "max_seats", Label: "Max Seats (B2B)", Type: "number", Default: "0"},
					{Key: "requires_kyc", Label: "Requires KYC", Type: "boolean", Default: "false"},
					// Billing details
					{Key: "auto_renewal", Label: "Auto Renewal", Type: "boolean", Default: "false"},
					{Key: "renewal_grace_days", Label: "Grace Period (days)", Type: "number", Default: "0"},
					{Key: "included_traffic_gb", Label: "Included Traffic GB (hybrid)", Type: "number", Default: "0"},
					{Key: "overage_per_gb_cents", Label: "Overage per GB (cents)", Type: "number", Default: "0"},
					{Key: "credit_cost_per_day", Label: "Credit Cost/Day (cents)", Type: "number", Default: "0"},
					{Key: "credit_cost_per_gb", Label: "Credit Cost/GB (cents)", Type: "number", Default: "0"},
					// Trial
					{Key: "trial_days", Label: "Trial Days", Type: "number", Default: "0"},
					{Key: "trial_requires_card", Label: "Trial Requires Card", Type: "boolean", Default: "false"},
					{Key: "trial_to_plan_id", Label: "Trial Target Plan ID", Type: "text"},
					// Lifecycle
					{Key: "upgrade_policy", Label: "Upgrade Policy", Type: "select", Default: "next_period", Options: []string{"immediate", "next_period", "prorated"}},
					{Key: "downgrade_policy", Label: "Downgrade Policy", Type: "select", Default: "next_period", Options: []string{"next_period", "immediate_refund"}},
					{Key: "cancellation_policy", Label: "Cancellation Policy", Type: "select", Default: "end_of_period", Options: []string{"immediate", "end_of_period"}},
					{Key: "refund_window_days", Label: "Refund Window (days)", Type: "number", Default: "0"},
					// Stock & Visibility
					{Key: "available_from", Label: "Available From (RFC3339)", Type: "text"},
					{Key: "available_until", Label: "Available Until (RFC3339)", Type: "text"},
					{Key: "stock_limit", Label: "Stock Limit (0=unlimited)", Type: "number", Default: "0"},
					{Key: "stock_remaining", Label: "Stock Remaining", Type: "number", Default: "0"},
					{Key: "requires_invite_code", Label: "Requires Invite Code", Type: "boolean", Default: "false"},
					{Key: "visible_in_public", Label: "Visible in Public", Type: "boolean", Default: "true"},
					{Key: "visible_in_telegram", Label: "Visible in Telegram", Type: "boolean", Default: "true"},
					{Key: "visible_in_cabinet", Label: "Visible in Cabinet", Type: "boolean", Default: "true"},
					{Key: "visible_in_reseller_portal", Label: "Visible in Reseller Portal", Type: "boolean", Default: "false"},
					// Reseller
					{Key: "reseller_markup_pct", Label: "Reseller Markup (bp)", Type: "number", Default: "0"},
					{Key: "reseller_min_price", Label: "Reseller Min Price (cents)", Type: "number", Default: "0"},
					{Key: "reseller_max_discount", Label: "Reseller Max Discount (bp)", Type: "number", Default: "0"},
					// Tax
					{Key: "tax_category", Label: "Tax Category", Type: "select", Default: "digital_service", Options: []string{"digital_service", "saas", "vpn"}},
					{Key: "tax_inclusive", Label: "Tax Inclusive", Type: "boolean", Default: "false"},
					// Metadata
					{Key: "badge_text", Label: "Badge Text", Type: "text"},
					{Key: "badge_color", Label: "Badge Color", Type: "text"},
					{Key: "is_active", Label: "Active", Type: "boolean", Default: "true"},
					{Key: "sort_order", Label: "Sort order", Type: "number", Default: "0"},
				},
			},
			// --- Page 2: Pricing Rules ---
			{
				Path:       "pricing-rules",
				Title:      "Pricing Rules",
				Icon:       "Calculator",
				Menu:       plugin.PageMenuAdmin,
				Collection: CollectionPricingRules,
				Fields: []plugin.ManifestPageField{
					{Key: "name", Label: "Name", Type: "text", Required: true},
					{Key: "stacking_policy", Label: "Stacking", Type: "select", Default: "multiplicative", Options: []string{"additive", "multiplicative", "best_only"}},
					{Key: "price_floor_cents", Label: "Price Floor (cents)", Type: "number", Default: "0"},
					{Key: "rounding", Label: "Rounding", Type: "select", Default: "none", Options: []string{"none", "charm_99", "charm_95"}},
				},
			},
			// --- Page 3: Promo Codes ---
			{
				Path:       "promo-codes",
				Title:      "Promo Codes",
				Icon:       "Tag",
				Menu:       plugin.PageMenuAdmin,
				Collection: CollectionPromoCodes,
				Fields: []plugin.ManifestPageField{
					{Key: "code", Label: "Code", Type: "text", Required: true},
					{Key: "is_active", Label: "Active", Type: "boolean", Default: "true"},
					{Key: "campaign_id", Label: "Campaign ID", Type: "text"},
					{Key: "source", Label: "Source", Type: "text"},
					{Key: "created_by", Label: "Created By", Type: "text"},
				},
			},
			// --- Page 4: A/B Tests ---
			{
				Path:       "ab-experiments",
				Title:      "A/B Tests",
				Icon:       "BarChart3",
				Menu:       plugin.PageMenuAdmin,
				Collection: CollectionABExperiments,
				Fields: []plugin.ManifestPageField{
					{Key: "name", Label: "Name", Type: "text", Required: true},
					{Key: "tariff_id", Label: "Tariff ID", Type: "text", Required: true},
					{Key: "status", Label: "Status", Type: "select", Default: "draft", Options: []string{"draft", "running", "paused", "completed", "archived"}},
					{Key: "assignment_strategy", Label: "Assignment", Type: "select", Default: "hash_user_id", Options: []string{"hash_user_id", "hash_tenant_id", "random"}},
					{Key: "stop_rule", Label: "Stop Rule", Type: "text"},
				},
			},
			// --- Page 5: Cohorts ---
			{
				Path:       "cohorts",
				Title:      "Cohorts",
				Icon:       "Users",
				Menu:       plugin.PageMenuAdmin,
				Collection: CollectionCohorts,
				Fields: []plugin.ManifestPageField{
					{Key: "name", Label: "Name", Type: "text", Required: true},
					{Key: "user_count", Label: "Users", Type: "number", Default: "0"},
				},
			},
		},
		Routes: tariffRoutes(),
	}
}

// tariffRoutes returns all HTTP routes for the tariff-manager plugin.
// Static paths MUST come before parameterized paths to avoid chi routing conflicts.
func tariffRoutes() []plugin.ManifestRoute {
	return []plugin.ManifestRoute{
		// --- Remnawave lookups (accept ?panel_id= for multi-panel) ---
		{Method: "GET", Path: "/api/tariffs/panels", Function: "list_panels_for_tariff", Public: false},
		{Method: "GET", Path: "/api/tariffs/internal-squads", Function: "list_internal_squads", Public: false},
		{Method: "GET", Path: "/api/tariffs/external-squads", Function: "list_external_squads", Public: false},
		{Method: "GET", Path: "/api/tariffs/nodes", Function: "list_nodes", Public: false},

		// --- Tariff CRUD + extended ---
		{Method: "GET", Path: "/api/tariffs/catalog", Function: "list_tariff_catalog", Public: true},
		{Method: "GET", Path: "/api/tariffs/export", Function: "export_tariffs", Public: false},
		{Method: "POST", Path: "/api/tariffs/import", Function: "import_tariffs", Public: false},
		{Method: "GET", Path: "/api/tariffs", Function: "list_tariffs", Public: true},
		{Method: "POST", Path: "/api/tariffs", Function: "create_tariff", Public: false},
		{Method: "GET", Path: "/api/tariffs/{tariffID}/price", Function: "get_tariff_price", Public: true},
		{Method: "GET", Path: "/api/tariffs/{tariffID}/stats", Function: "get_tariff_stats", Public: false},
		{Method: "GET", Path: "/api/tariffs/{tariffID}/version-history", Function: "get_tariff_version_history", Public: false},
		{Method: "POST", Path: "/api/tariffs/{tariffID}/clone", Function: "clone_tariff", Public: false},
		{Method: "POST", Path: "/api/tariffs/{tariffID}/archive", Function: "archive_tariff", Public: false},
		{Method: "GET", Path: "/api/tariffs/{tariffID}", Function: "get_tariff", Public: true},
		{Method: "PUT", Path: "/api/tariffs/{tariffID}", Function: "update_tariff", Public: false},
		{Method: "DELETE", Path: "/api/tariffs/{tariffID}", Function: "delete_tariff", Public: false},

		// --- Pricing rules ---
		{Method: "GET", Path: "/api/tariffs/pricing-rules", Function: "list_pricing_rules", Public: false},
		{Method: "POST", Path: "/api/tariffs/pricing-rules", Function: "create_pricing_rule", Public: false},
		{Method: "POST", Path: "/api/tariffs/pricing-rules/{pricingRuleID}/simulate", Function: "simulate_pricing_rule", Public: false},
		{Method: "GET", Path: "/api/tariffs/pricing-rules/{pricingRuleID}", Function: "get_pricing_rule", Public: false},
		{Method: "PUT", Path: "/api/tariffs/pricing-rules/{pricingRuleID}", Function: "update_pricing_rule", Public: false},
		{Method: "DELETE", Path: "/api/tariffs/pricing-rules/{pricingRuleID}", Function: "delete_pricing_rule", Public: false},

		// --- Promo codes ---
		{Method: "POST", Path: "/api/tariffs/promo-codes/validate", Function: "validate_promo_code", Public: true},
		{Method: "POST", Path: "/api/tariffs/promo-codes/bulk-generate", Function: "bulk_generate_promo_codes", Public: false},
		{Method: "GET", Path: "/api/tariffs/promo-codes", Function: "list_promo_codes", Public: false},
		{Method: "POST", Path: "/api/tariffs/promo-codes", Function: "create_promo_code", Public: false},
		{Method: "GET", Path: "/api/tariffs/promo-codes/{promoCodeID}/stats", Function: "get_promo_code_stats", Public: false},
		{Method: "GET", Path: "/api/tariffs/promo-codes/{promoCodeID}", Function: "get_promo_code", Public: false},
		{Method: "PUT", Path: "/api/tariffs/promo-codes/{promoCodeID}", Function: "update_promo_code", Public: false},
		{Method: "DELETE", Path: "/api/tariffs/promo-codes/{promoCodeID}", Function: "delete_promo_code", Public: false},

		// --- A/B experiments ---
		{Method: "GET", Path: "/api/tariffs/ab-experiments", Function: "list_ab_experiments", Public: false},
		{Method: "POST", Path: "/api/tariffs/ab-experiments", Function: "create_ab_experiment", Public: false},
		{Method: "PUT", Path: "/api/tariffs/ab-experiments/{experimentID}/start", Function: "start_ab_experiment", Public: false},
		{Method: "PUT", Path: "/api/tariffs/ab-experiments/{experimentID}/pause", Function: "pause_ab_experiment", Public: false},
		{Method: "PUT", Path: "/api/tariffs/ab-experiments/{experimentID}/conclude", Function: "conclude_ab_experiment", Public: false},
		{Method: "GET", Path: "/api/tariffs/ab-experiments/{experimentID}/results", Function: "get_ab_experiment_results", Public: false},

		// --- Cohorts ---
		{Method: "GET", Path: "/api/tariffs/cohorts", Function: "list_cohorts", Public: false},
		{Method: "POST", Path: "/api/tariffs/cohorts", Function: "create_cohort", Public: false},
		{Method: "GET", Path: "/api/tariffs/cohorts/{cohortID}/users", Function: "get_cohort_users", Public: false},
		{Method: "GET", Path: "/api/tariffs/cohorts/{cohortID}", Function: "get_cohort", Public: false},
		{Method: "PUT", Path: "/api/tariffs/cohorts/{cohortID}", Function: "update_cohort", Public: false},
		{Method: "DELETE", Path: "/api/tariffs/cohorts/{cohortID}", Function: "delete_cohort", Public: false},

		// --- Analytics ---
		{Method: "GET", Path: "/api/tariffs/analytics/mrr-by-tariff", Function: "get_mrr_by_tariff", Public: false},
		{Method: "GET", Path: "/api/tariffs/analytics/conversion-funnel", Function: "get_conversion_funnel", Public: false},

		// --- Reseller ---
		{Method: "GET", Path: "/api/tariffs/reseller/catalog", Function: "get_reseller_catalog", Public: false},
		{Method: "POST", Path: "/api/tariffs/reseller/customize", Function: "customize_reseller", Public: false},
	}
}

// RegisterRoutes maps the plugin's manifest route functions to native Go handlers.
func RegisterRoutes(registry *gateway.BuiltinRouteRegistry, h *Handler) {
	r := func(fn string, handler func(http.ResponseWriter, *http.Request)) {
		registry.Register(PluginSlug, fn, handler)
	}

	// Remnawave lookups
	r("list_panels_for_tariff", h.ListPanelsForTariff)
	r("list_internal_squads", h.ListInternalSquads)
	r("list_external_squads", h.ListExternalSquads)
	r("list_nodes", h.ListNodes)

	// Tariff CRUD + extended
	r("list_tariffs", h.ListTariffs)
	r("get_tariff", h.GetTariff)
	r("create_tariff", h.CreateTariff)
	r("update_tariff", h.UpdateTariff)
	r("delete_tariff", h.DeleteTariff)
	r("get_tariff_price", h.GetTariffPrice)
	r("clone_tariff", h.CloneTariff)
	r("archive_tariff", h.ArchiveTariff)
	r("get_tariff_stats", h.GetTariffStats)
	r("get_tariff_version_history", h.GetTariffVersionHistory)
	r("list_tariff_catalog", h.ListTariffCatalog)
	r("export_tariffs", h.ExportTariffs)
	r("import_tariffs", h.ImportTariffs)

	// Pricing rules
	r("list_pricing_rules", h.ListPricingRules)
	r("get_pricing_rule", h.GetPricingRule)
	r("create_pricing_rule", h.CreatePricingRule)
	r("update_pricing_rule", h.UpdatePricingRule)
	r("delete_pricing_rule", h.DeletePricingRule)
	r("simulate_pricing_rule", h.SimulatePricingRule)

	// Promo codes
	r("list_promo_codes", h.ListPromoCodes)
	r("get_promo_code", h.GetPromoCode)
	r("create_promo_code", h.CreatePromoCode)
	r("update_promo_code", h.UpdatePromoCode)
	r("delete_promo_code", h.DeletePromoCode)
	r("validate_promo_code", h.ValidatePromoCode)
	r("bulk_generate_promo_codes", h.BulkGeneratePromoCodes)
	r("get_promo_code_stats", h.GetPromoCodeStats)

	// A/B experiments
	r("list_ab_experiments", h.ListABExperiments)
	r("create_ab_experiment", h.CreateABExperiment)
	r("start_ab_experiment", h.StartABExperiment)
	r("pause_ab_experiment", h.PauseABExperiment)
	r("conclude_ab_experiment", h.ConcludeABExperiment)
	r("get_ab_experiment_results", h.GetABExperimentResults)

	// Cohorts
	r("list_cohorts", h.ListCohorts)
	r("get_cohort", h.GetCohort)
	r("create_cohort", h.CreateCohort)
	r("update_cohort", h.UpdateCohort)
	r("delete_cohort", h.DeleteCohort)
	r("get_cohort_users", h.GetCohortUsers)

	// Analytics
	r("get_mrr_by_tariff", h.GetMRRByTariff)
	r("get_conversion_funnel", h.GetConversionFunnel)

	// Reseller
	r("get_reseller_catalog", h.GetResellerCatalog)
	r("customize_reseller", h.CustomizeReseller)
}
