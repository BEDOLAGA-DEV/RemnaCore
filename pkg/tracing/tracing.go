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

// tracer is the package-level tracer instance. It resolves to whatever
// TracerProvider has been set globally via otel.SetTracerProvider (which
// happens in internal/observability.InitTracer at startup).
var tracer = otel.Tracer(TracerName)

// StartSpan creates a new span from the given context using the global tracer.
// Callers should defer span.End() immediately after calling StartSpan.
func StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return tracer.Start(ctx, name)
}
