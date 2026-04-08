package plugin

// BuiltInPluginDef describes a platform-provided plugin that is auto-seeded on
// startup if it does not already exist in the database. Built-in plugins have
// no WASM binary — their behaviour is implemented natively in Go.
type BuiltInPluginDef struct {
	Slug         string
	Name         string
	Version      string
	Description  string
	Author       string
	ConfigFields map[string]ManifestConfigField
	Pages        []ManifestPage
	Routes       []ManifestRoute
}

// Built-in plugin slugs.
const (
	BuiltInSlugRemnawaveProvider = "remnawave-provider"
	BuiltInSlugTariffManager     = "tariff-manager"
)

// Built-in plugin metadata constants.
const (
	builtInRemnawaveVersion     = "1.0.0"
	builtInRemnawaveAuthor      = "RemnaCore"
	builtInRemnawaveDescription = "Built-in VPN provider integration with Remnawave panel. Manages connection configuration (URL, API token, webhook secret) through the admin plugin UI."

	builtInTariffVersion     = "1.0.0"
	builtInTariffAuthor      = "RemnaCore"
	builtInTariffDescription = "Tariff management with flexible conditions. Create subscription plans with traffic limits, device limits, squad assignments, and custom pricing rules."
)

// Built-in config field keys.
const (
	RemnawaveConfigKeyURL           = "url"
	RemnawaveConfigKeyAPIToken      = "api_token"
	RemnawaveConfigKeyWebhookSecret = "webhook_secret"

	TariffConfigKeyDefaultCurrency = "default_currency"
)

// BuiltInPlugins returns the list of built-in plugin definitions that should
// be auto-seeded on startup if they don't already exist in the database.
func BuiltInPlugins() []BuiltInPluginDef {
	return []BuiltInPluginDef{remnawaveProvider(), tariffManager()}
}

func tariffManager() BuiltInPluginDef {
	return BuiltInPluginDef{
		Slug:        BuiltInSlugTariffManager,
		Name:        "Tariff Manager",
		Version:     builtInTariffVersion,
		Description: builtInTariffDescription,
		Author:      builtInTariffAuthor,
		ConfigFields: map[string]ManifestConfigField{
			TariffConfigKeyDefaultCurrency: {
				Type:     "select",
				Label:    "Default Currency",
				Required: false,
				Default:  "USD",
				Options:  []string{"USD", "EUR", "RUB", "GBP", "TRY", "UAH", "KZT"},
			},
		},
		Pages: []ManifestPage{
			{
				Path:  "tariffs",
				Title: "Tariffs",
				Icon:  "CreditCard",
				Menu:  PageMenuAdmin,
			},
		},
		Routes: []ManifestRoute{
			{Method: "GET", Path: "/api/tariffs", Function: "list_tariffs", Public: true},
			{Method: "GET", Path: "/api/tariffs/{tariffID}", Function: "get_tariff", Public: true},
			{Method: "POST", Path: "/api/tariffs", Function: "create_tariff", Public: false},
			{Method: "PUT", Path: "/api/tariffs/{tariffID}", Function: "update_tariff", Public: false},
			{Method: "DELETE", Path: "/api/tariffs/{tariffID}", Function: "delete_tariff", Public: false},
		},
	}
}

func remnawaveProvider() BuiltInPluginDef {
	return BuiltInPluginDef{
		Slug:        BuiltInSlugRemnawaveProvider,
		Name:        "Remnawave Provider",
		Version:     builtInRemnawaveVersion,
		Description: builtInRemnawaveDescription,
		Author:      builtInRemnawaveAuthor,
		ConfigFields: map[string]ManifestConfigField{
			RemnawaveConfigKeyURL: {
				Type:     "string",
				Label:    "Panel URL",
				Required: true,
				Default:  "",
			},
			RemnawaveConfigKeyAPIToken: {
				Type:     configFieldTypeSecret,
				Label:    "API Token",
				Required: true,
				Default:  "",
			},
			RemnawaveConfigKeyWebhookSecret: {
				Type:     configFieldTypeSecret,
				Label:    "Webhook Secret",
				Required: false,
				Default:  "",
			},
		},
	}
}
