package payment

import "go.uber.org/fx"

// Module provides payment domain services to the Fx dependency graph.
var Module = fx.Module("payment",
	fx.Provide(NewPaymentFacade),
)
