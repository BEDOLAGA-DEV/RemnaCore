package gateway

import (
	"testing"

	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/handler"
)

// TestTelegramAuthHandlerGraphResolves validates the fx dependency chain for the
// Telegram auth handler after the single-context refactor: the handler depends
// on the ShopBotConfigResolver port (not the reseller service directly), and
// provideShopBotConfigResolver adapts *reseller.ResellerService to that port.
// fx.ValidateApp builds the graph without running constructors or lifecycle, so
// a missing or ambiguous provider surfaces here — which -short unit tests, that
// never boot the fx graph, would otherwise miss.
func TestTelegramAuthHandlerGraphResolves(t *testing.T) {
	err := fx.ValidateApp(
		fx.Provide(func() *identity.Service { return nil }),
		fx.Provide(func() *reseller.ResellerService { return nil }),
		fx.Provide(provideShopBotConfigResolver),
		fx.Provide(handler.NewTelegramAuthHandler),
		fx.Invoke(func(*handler.TelegramAuthHandler) {}),
	)
	if err != nil {
		t.Fatalf("telegram auth handler fx graph does not resolve: %v", err)
	}
}
