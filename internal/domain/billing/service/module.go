package service

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"go.uber.org/fx"
)

// Module provides billing domain services to the Fx dependency graph.
var Module = fx.Module("billing",
	fx.Provide(NewProrateCalculator),
	fx.Provide(func(cfg *config.Config) *TrialManager {
		return NewTrialManagerWithClock(cfg.Billing.TrialDays, clock.NewReal())
	}),
	fx.Provide(NewBillingService),
	fx.Provide(NewCheckoutService),
)
