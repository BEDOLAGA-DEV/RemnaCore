package telegram

import "go.uber.org/fx"

// Module provides the Telegram bot to the Fx dependency graph.
var Module = fx.Module("telegram",
	fx.Provide(NewBot),
)
