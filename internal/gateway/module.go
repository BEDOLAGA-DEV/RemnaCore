package gateway

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/handler"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/health"
	"go.uber.org/fx"
)

// provideShopBotConfigResolver adapts the reseller service to the narrow
// ShopBotConfigResolver port the Telegram auth handler depends on. Keeping this
// adapter in the module (not the handler) lets the handler stay within a single
// domain service context per the gateway single-context architecture rule.
func provideShopBotConfigResolver(s *reseller.ResellerService) handler.ShopBotConfigResolver {
	return s
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
	fx.Provide(NewRouter),
)
