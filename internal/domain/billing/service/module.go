package service

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"go.uber.org/fx"
)

// Module provides billing domain services to the Fx dependency graph.
var Module = fx.Module("billing",
	fx.Provide(NewProrateCalculator),
	fx.Provide(func(cfg *config.Config) *TrialManager {
		return NewTrialManager(cfg.Billing.TrialDays)
	}),
	fx.Provide(NewBillingService),
	fx.Provide(NewCheckoutService),
)
