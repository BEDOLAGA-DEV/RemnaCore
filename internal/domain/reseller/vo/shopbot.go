package vo

import "github.com/BEDOLAGA-DEV/RemnaCore/pkg/secret"

// ShopBotConfig is a shop's Telegram bot configuration. Token is held as a
// masked secret.String in memory and is encrypted at rest by the adapter.
type ShopBotConfig struct {
	Token         secret.String
	WebhookSecret string
	CabinetURL    string
	BotUsername   string
	Enabled       bool
}

// ShopBotWithTenant pairs a config with its owning tenant (for the bot manager).
type ShopBotWithTenant struct {
	TenantID string
	Config   ShopBotConfig
}
