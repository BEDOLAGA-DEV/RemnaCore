package app

import (
	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	resellerservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/service"
)

// resellerWiring provides all reseller-domain bindings: tenant and commission
// repository implementations.
var resellerWiring = fx.Options(
	// Reseller domain service
	fx.Provide(resellerservice.NewResellerService),

	// Reseller repos -> interface bindings (service package types for Fx matching)
	fx.Provide(postgres.NewResellerRepository),
	fx.Provide(func(repo *postgres.ResellerRepository) resellerservice.TenantRepository { return repo }),
	fx.Provide(func(repo *postgres.ResellerRepository) resellerservice.CommissionRepository { return repo }),
)
