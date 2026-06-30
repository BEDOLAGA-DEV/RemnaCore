package telegram

import (
	"log/slog"

	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
)

// Module provides the Telegram bot and bot manager to the Fx dependency graph.
var Module = fx.Module("telegram",
	fx.Provide(NewBot),
	fx.Provide(provideBotManager),
)

// provideBotManager adapts the concrete reseller service to ShopBotLister.
func provideBotManager(
	platform *Bot,
	identitySvc *identity.Service,
	resellerSvc *reseller.ResellerService,
	cfg *config.Config,
	logger *slog.Logger,
) *BotManager {
	return NewBotManager(platform, identitySvc, resellerSvc, cfg, logger)
}
