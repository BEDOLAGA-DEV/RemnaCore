package app

import (
	"context"
	"fmt"
	"log/slog"

	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/observability"
)

// tracingWiring provides the OpenTelemetry tracing lifecycle: initialises the
// tracer provider on start and flushes pending spans on stop.
var tracingWiring = fx.Options(
	fx.Invoke(startTracing),
)

// startTracing initialises the OpenTelemetry tracer provider on start and
// flushes pending spans on stop. When no TRACING_ENDPOINT is configured a noop
// provider is used and the shutdown function is a harmless no-op.
func startTracing(lc fx.Lifecycle, cfg *config.Config, logger *slog.Logger) {
	var shutdown observability.TracerShutdownFunc

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			var err error
			shutdown, err = observability.InitTracer(ctx, cfg, logger)
			if err != nil {
				return fmt.Errorf("init tracer: %w", err)
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if shutdown == nil {
				return nil
			}
			logger.Info("tracer provider shutting down")
			shutdownCtx, cancel := context.WithTimeout(ctx, observability.TracerShutdownTimeout)
			defer cancel()
			return shutdown(shutdownCtx)
		},
	})
}
