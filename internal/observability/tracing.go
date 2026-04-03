package observability

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tracing"
)

const (
	// ServiceName is the OpenTelemetry service name used in traces and
	// exported to the OTel resource.
	ServiceName = "remnacore"

	// TracerShutdownTimeout is the maximum time allowed for the tracer
	// provider to flush pending spans during graceful shutdown.
	TracerShutdownTimeout = 5 * time.Second
)

// TracerShutdownFunc is the function signature returned by InitTracer for
// graceful shutdown of the tracer provider.
type TracerShutdownFunc func(context.Context) error

// InitTracer sets up OpenTelemetry tracing with an OTLP HTTP exporter.
// It returns a shutdown function that must be called on application stop.
// If the tracing endpoint is not configured, it installs a noop tracer
// provider and returns a no-op shutdown function.
func InitTracer(ctx context.Context, cfg *config.Config, logger *slog.Logger) (TracerShutdownFunc, error) {
	if cfg.Tracing.Endpoint == "" {
		logger.Info("tracing disabled (no TRACING_ENDPOINT configured)")
		return func(_ context.Context) error { return nil }, nil
	}

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.Tracing.Endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create otlp exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(ServiceName),
			semconv.ServiceVersionKey.String(cfg.App.Version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create otel resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)

	logger.Info("tracing enabled",
		slog.String("endpoint", cfg.Tracing.Endpoint),
		slog.String("service", ServiceName),
		slog.String("tracer_name", tracing.TracerName),
	)

	return tp.Shutdown, nil
}
