package observability

import (
	"log/slog"

	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
)

// Module provides structured logging and Prometheus metrics to the Fx dependency graph.
var Module = fx.Module("observability",
	fx.Provide(provideLogger),
	fx.Provide(NewMetrics),
)

func provideLogger(cfg *config.Config) *slog.Logger {
	logger := NewLogger(cfg.App.LogLevel, cfg.App.LogFormat)
	slog.SetDefault(logger)
	return logger
}
