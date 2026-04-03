package infra

import "go.uber.org/fx"

// Module provides all infrastructure services to the Fx dependency graph.
var Module = fx.Module("infra",
	fx.Provide(NewNodeHealthCache),
	fx.Provide(NewHealthMonitor),
	fx.Provide(NewSmartRouter),
	fx.Provide(NewSpeedTestServer),
	fx.Provide(NewSubscriptionProxy),
)
