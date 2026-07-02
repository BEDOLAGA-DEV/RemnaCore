package gateway

import (
	"context"
	"errors"
	"log/slog"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/handler"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/health"
	"go.uber.org/fx"
)

// shopBotConfigResolver adapts the reseller service to the narrow
// handler.ShopBotConfigResolver port, translating the reseller domain's
// not-found error into the port-level handler.ErrShopBotNotConfigured so the
// handler can classify not-found vs infrastructure failures without importing
// the reseller service context (gateway single-context architecture rule).
type shopBotConfigResolver struct {
	svc *reseller.ResellerService
}

func (r shopBotConfigResolver) GetShopBot(ctx context.Context, tenantID string) (*vo.ShopBotConfig, error) {
	cfg, err := r.svc.GetShopBot(ctx, tenantID)
	if err != nil {
		if errors.Is(err, reseller.ErrShopBotNotFound) {
			return nil, handler.ErrShopBotNotConfigured
		}
		return nil, err
	}
	return cfg, nil
}

// provideShopBotConfigResolver wires the adapter above into the fx graph.
func provideShopBotConfigResolver(s *reseller.ResellerService) handler.ShopBotConfigResolver {
	return shopBotConfigResolver{svc: s}
}

// provideBotPluginLister adapts the concrete telegram bot-plugin catalog to the
// narrow BotPluginLister port the bot-catalog handler depends on. Keeping the
// adapter here lets the handler stay decoupled from the telegram concrete type.
func provideBotPluginLister(c *telegram.BotPluginCatalog) handler.BotPluginLister {
	return c
}

// shopNameGetterAdapter adapts *reseller.ResellerService to the narrow
// handler.ShopNameGetter port. The reseller.tenants table carries no RLS, so
// no platform-scope wrapping is required; a direct GetTenant call suffices.
type shopNameGetterAdapter struct {
	svc *reseller.ResellerService
}

// GetTenantName implements handler.ShopNameGetter.
func (a *shopNameGetterAdapter) GetTenantName(ctx context.Context, id string) (string, error) {
	t, err := a.svc.GetTenant(ctx, id)
	if err != nil {
		return "", err
	}
	return t.Name, nil
}

// provideMeShopsHandler wires the MeShopsHandler by bridging rbac.Repository
// and *reseller.ResellerService to the two narrow ports the handler depends on.
func provideMeShopsHandler(
	bindings rbac.Repository,
	svc *reseller.ResellerService,
	log *slog.Logger,
) *handler.MeShopsHandler {
	return handler.NewMeShopsHandler(bindings, &shopNameGetterAdapter{svc: svc}, log)
}

// Module provides the HTTP gateway layer (handlers + router) to the Fx
// dependency graph.
var Module = fx.Module("gateway",
	fx.Provide(
		fx.Annotate(
			func(checkers []health.Checker) *handler.HealthHandler {
				return handler.NewHealthHandler(checkers)
			},
			fx.ParamTags(`group:"health.checkers"`),
		),
	),
	fx.Provide(handler.NewIdentityHandler),
	fx.Provide(handler.NewSetupHandler),
	fx.Provide(handler.NewBillingHandler),
	fx.Provide(handler.NewMultiSubHandler),
	fx.Provide(handler.NewPluginHandler),
	fx.Provide(handler.NewCheckoutHandler),
	fx.Provide(handler.NewPaymentWebhookHandler),
	fx.Provide(handler.NewFamilyHandler),
	fx.Provide(handler.NewAdminHandler),
	fx.Provide(handler.NewResellerHandler),
	fx.Provide(handler.NewRoutingHandler),
	fx.Provide(handler.NewSettingsHandler),
	fx.Provide(handler.NewStatsHandler),
	fx.Provide(handler.NewActivityHandler),
	fx.Provide(handler.NewPluginRPCHandler),
	fx.Provide(handler.NewPluginRouteHandler),
	fx.Provide(handler.NewIAMHandler),
	fx.Provide(provideShopBotConfigResolver),
	fx.Provide(handler.NewTelegramAuthHandler),
	fx.Provide(provideBotPluginLister),
	fx.Provide(handler.NewBotCatalogHandler),
	fx.Provide(provideMeShopsHandler),
	fx.Provide(NewRouter),
)
