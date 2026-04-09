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
)

// Built-in plugin metadata constants.
const (
	builtInRemnawaveVersion     = "1.0.0"
	builtInRemnawaveAuthor      = "RemnaCore"
	builtInRemnawaveDescription = "Built-in VPN provider integration with Remnawave panel. Manages connection configuration (URL, API token, webhook secret) through the admin plugin UI."
)

// Built-in config field keys.
const (
	RemnawaveConfigKeyURL           = "url"
	RemnawaveConfigKeyAPIToken      = "api_token"
	RemnawaveConfigKeyWebhookSecret = "webhook_secret"
)

// BuiltInPlugins returns the list of built-in plugin definitions that should
// be auto-seeded on startup if they don't already exist in the database.
// Each built-in plugin package provides its own Plugin() definition.
func BuiltInPlugins() []BuiltInPluginDef {
	return []BuiltInPluginDef{remnawaveProvider()}
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
		Routes: []ManifestRoute{
			{Method: "POST", Path: "/api/remnawave/test-connection", Function: "test_connection", Public: false},
		},
	}
}
