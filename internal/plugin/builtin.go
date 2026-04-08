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
		Pages: []ManifestPage{
			{
				Path:  "tariffs",
				Title: "Tariffs",
				Icon:  "CreditCard",
				Menu:  PageMenuAdmin,
			},
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
