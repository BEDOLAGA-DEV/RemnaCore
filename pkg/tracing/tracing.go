// Package tracing provides a thin wrapper around the OpenTelemetry tracing API
// so that domain and infrastructure packages can create spans without coupling
// to the OTel SDK initialisation code in internal/observability.
package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// TracerName is the instrumentation library name used for all manual spans.
const TracerName = "remnacore"

// tracer uses OTel's global tracer provider which resolves lazily.
// Even though this is initialized before otel.SetTracerProvider is called,
// the global provider delegates to whatever provider is set at call time,
// not at tracer-creation time. This is safe and documented OTel behavior.
var tracer = otel.Tracer(TracerName)

// StartSpan creates a new span from the given context using the global tracer.
// Callers should defer span.End() immediately after calling StartSpan.
func StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return tracer.Start(ctx, name)
}
