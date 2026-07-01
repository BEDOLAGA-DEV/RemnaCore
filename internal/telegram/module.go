package telegram

import (
	"context"
	"log/slog"

	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/cabinetbot"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	resellerservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// Module provides the Telegram bot and bot manager to the Fx dependency graph.
var Module = fx.Module("telegram",
	fx.Provide(NewBot),
	fx.Provide(NewBuiltinBotRegistry),
	fx.Provide(provideBotOps),
	fx.Provide(NewBotPluginValidator),
	fx.Invoke(registerCabinetBot),
	fx.Provide(provideBotManager),
)

// provideBotOps constructs a bothost.Registry pre-populated with the standard
// host operations (telegram.send_*, cabinet.open, user.register).
func provideBotOps() *bothost.Registry {
	r := bothost.NewRegistry()
	bothost.RegisterStdOps(r)
	return r
}

// botPluginValidator is the real BotPluginValidator backed by the BuiltinBotRegistry.
type botPluginValidator struct {
	reg *BuiltinBotRegistry
}

// IsValidBotPlugin reports whether slug names a registered built-in bot plugin.
func (v *botPluginValidator) IsValidBotPlugin(_ context.Context, slug string) (bool, error) {
	_, _, ok := v.reg.Lookup(slug)
	return ok, nil
}

// NewBotPluginValidator returns a resellerservice.BotPluginValidator backed by
// the BuiltinBotRegistry. The reseller domain only imports this interface; the
// telegram package imports the reseller service package for the type — not the
// other way around — so there is no import cycle.
func NewBotPluginValidator(reg *BuiltinBotRegistry) resellerservice.BotPluginValidator {
	return &botPluginValidator{reg: reg}
}

// registerCabinetBot seeds the BuiltinBotRegistry with the cabinet-bot handler.
func registerCabinetBot(reg *BuiltinBotRegistry) {
	reg.Register(cabinetbot.Slug, cabinetbot.RequiredPerms(), cabinetbot.Handler)
}

// provideBotManager adapts the concrete reseller service to ShopBotLister.
func provideBotManager(
	platform *Bot,
	identitySvc *identity.Service,
	resellerSvc *reseller.ResellerService,
	cfg *config.Config,
	botRegistry *BuiltinBotRegistry,
	botOps *bothost.Registry,
	txRunner txmanager.Runner,
	logger *slog.Logger,
) *BotManager {
	return NewBotManager(platform, identitySvc, resellerSvc, cfg, botRegistry, botOps, txRunner, logger)
}
