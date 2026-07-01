package app

import (
	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	resellerservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/service"
)

// resellerWiring provides all reseller-domain bindings: tenant, commission,
// customer, and shop-bot repository implementations.
var resellerWiring = fx.Options(
	// Reseller domain service
	fx.Provide(resellerservice.NewResellerService),

	// Reseller repos -> interface bindings (service package types for Fx matching)
	fx.Provide(postgres.NewResellerRepository),
	fx.Provide(func(repo *postgres.ResellerRepository) resellerservice.TenantRepository { return repo }),
	fx.Provide(func(repo *postgres.ResellerRepository) resellerservice.CommissionRepository { return repo }),
	fx.Provide(func(repo *postgres.ResellerRepository) resellerservice.CustomerRepository { return repo }),

	// Shop-bot repository (encrypted at rest via secretbox.Box from identity wiring)
	fx.Provide(postgres.NewShopBotRepository),
	fx.Provide(func(repo *postgres.ShopBotRepository) resellerservice.ShopBotRepository { return repo }),

	// STOPGAP (Task 8): AllowAllBotPlugins accepts every plugin slug without
	// real validation. Task 8 must replace this provider with the real
	// telegram-backed BotPluginValidator once the telegram registry is wired.
	fx.Provide(func() resellerservice.BotPluginValidator { return resellerservice.AllowAllBotPlugins{} }),
)
