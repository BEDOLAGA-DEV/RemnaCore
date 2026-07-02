package telegram

import (
	"context"
	"errors"
	"log/slog"

	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/cabinetbot"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	resellerservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// Module provides the Telegram bot and bot manager to the Fx dependency graph.
var Module = fx.Module("telegram",
	fx.Provide(NewBot),
	fx.Provide(NewBuiltinBotRegistry),
	fx.Provide(provideBotOps),
	fx.Provide(NewBotPluginValidator),
	// Adapt the full plugin repository down to the narrow PluginReader the WASM
	// bot dispatcher depends on.
	fx.Provide(func(r plugin.PluginRepository) PluginReader { return r }),
	fx.Provide(NewWASMBotDispatcher),
	// Adapt the full plugin repository down to the narrow PluginLister the
	// bot-plugin catalog depends on, then provide the catalog itself.
	fx.Provide(func(r plugin.PluginRepository) PluginLister { return r }),
	fx.Provide(NewBotPluginCatalog),
	fx.Invoke(registerCabinetBot),
	// Domain-reader adapters: billing-layer readers (from billing fx bindings).
	fx.Provide(NewSubscriptionReaderAdapter),
	fx.Provide(NewInvoiceReaderAdapter),
	fx.Provide(NewCheckoutStarterAdapter),
	// Bundle the five reader ports into one value for the bot stack.
	fx.Provide(newDomainReaders),
	fx.Provide(provideBotManager),
)

// provideBotOps constructs a bothost.Registry pre-populated with all host
// operations: the standard set (telegram.send_*, cabinet.open, user.register)
// and the seven domain ops (plans.*, subscriptions.*, invoices.*, balance.*,
// checkout.*).
func provideBotOps() *bothost.Registry {
	r := bothost.NewRegistry()
	bothost.RegisterStdOps(r)
	bothost.RegisterDomainOps(r)
	return r
}

// botPluginValidator is the real BotPluginValidator backed by both the
// BuiltinBotRegistry (for native Go bot plugins) and a PluginReader (for
// enabled WASM bot plugins whose manifest declares ProvidesBot=true).
type botPluginValidator struct {
	reg     *BuiltinBotRegistry
	plugins PluginReader
}

// IsValidBotPlugin reports whether slug names either a registered built-in bot
// plugin or an enabled WASM plugin whose manifest declares ProvidesBot=true.
//
// Resolution order:
//  1. Check the built-in registry; if found, return (true, nil) immediately
//     without consulting the plugin repo.
//  2. Consult the plugin repo. A not-found result (errors.Is ErrPluginNotFound)
//     is a clean rejection: (false, nil). Any other repo error is a real
//     infrastructure failure: (false, err). A found plugin is accepted only when
//     Status==StatusEnabled, Manifest!=nil, Manifest.Telegram!=nil, and
//     Manifest.Telegram.ProvidesBot==true.
func (v *botPluginValidator) IsValidBotPlugin(ctx context.Context, slug string) (bool, error) {
	if _, _, ok := v.reg.Lookup(slug); ok {
		return true, nil
	}

	p, err := v.plugins.GetBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, plugin.ErrPluginNotFound) {
			return false, nil
		}
		return false, err
	}
	if p == nil {
		return false, nil
	}
	if p.Status != plugin.StatusEnabled {
		return false, nil
	}
	if !isEnabledWASMBot(p) {
		return false, nil
	}
	return true, nil
}

// NewBotPluginValidator returns a resellerservice.BotPluginValidator backed by
// the BuiltinBotRegistry and the PluginReader. The reseller domain only imports
// this interface; the telegram package imports the reseller service package for
// the type — not the other way around — so there is no import cycle.
func NewBotPluginValidator(reg *BuiltinBotRegistry, plugins PluginReader) resellerservice.BotPluginValidator {
	return &botPluginValidator{reg: reg, plugins: plugins}
}

// registerCabinetBot seeds the BuiltinBotRegistry with the cabinet-bot handler.
func registerCabinetBot(reg *BuiltinBotRegistry) {
	reg.Register(cabinetbot.Slug, cabinetbot.RequiredPerms(), cabinetbot.Handler)
}

// newDomainReaders bundles the five domain reader/starter ports into a single
// DomainReaders value for injection into the bot stack. Each port is sourced
// from its own fx-provided adapter so they can be replaced or extended
// independently.
func newDomainReaders(
	tariffs bothost.TariffReader,
	subs bothost.SubscriptionReader,
	invoices bothost.InvoiceReader,
	bal bothost.BalanceReader,
	checkout bothost.CheckoutStarter,
) DomainReaders {
	return DomainReaders{
		Tariffs:  tariffs,
		Subs:     subs,
		Invoices: invoices,
		Balance:  bal,
		Checkout: checkout,
	}
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
	wasmBots WASMBotDispatcher,
	readers DomainReaders,
	logger *slog.Logger,
) *BotManager {
	return NewBotManager(platform, identitySvc, resellerSvc, cfg, botRegistry, botOps, txRunner, wasmBots, readers, logger)
}
