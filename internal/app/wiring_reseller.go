package app

import (
	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
)

// resellerWiring provides all reseller-domain bindings: tenant and commission
// repository implementations.
var resellerWiring = fx.Options(
	// Reseller domain module
	reseller.Module,

	// Reseller repos -> interface bindings
	fx.Provide(postgres.NewResellerRepository),
	fx.Provide(func(repo *postgres.ResellerRepository) reseller.TenantRepository { return repo }),
	fx.Provide(func(repo *postgres.ResellerRepository) reseller.CommissionRepository { return repo }),
)
