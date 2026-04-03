package reseller

import (
	"log/slog"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"go.uber.org/fx"
)

// Module provides the reseller domain Service to the Fx dependency graph.
var Module = fx.Module("reseller",
	fx.Provide(func(
		tenants TenantRepository,
		commissions CommissionRepository,
		pub domainevent.Publisher,
		logger *slog.Logger,
	) *ResellerService {
		return NewResellerService(tenants, commissions, pub, logger)
	}),
)
