package infra

import (
	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/infra/health"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/infra/proxy"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/infra/routing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/infra/speedtest"
)

// Module provides all infrastructure services to the Fx dependency graph.
var Module = fx.Module("infra",
	fx.Provide(health.NewNodeHealthCache),
	fx.Provide(health.NewHealthMonitor),
	fx.Provide(routing.NewSmartRouter),
	fx.Provide(speedtest.NewSpeedTestServer),
	fx.Provide(proxy.NewSubscriptionProxy),
)
