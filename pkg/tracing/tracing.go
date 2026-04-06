// Package tracing is a shared kernel package providing a thin wrapper around
// the OpenTelemetry tracing API so that domain and infrastructure packages can
// create spans without coupling to the OTel SDK initialisation code in
// internal/observability. It lives in pkg/ (rather than internal/observability)
// because domain packages must be able to import it without violating the
// domain isolation rule.
package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
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

// W3C traceparent format version.
const traceParentVersion = "00"

// FormatTraceParent extracts the active span from ctx and returns a W3C
// traceparent string (e.g. "00-<traceID>-<spanID>-<flags>"). Returns an empty
// string when no valid span context exists in ctx. This is used by the outbox
// publisher to embed trace context into domain events for cross-process
// propagation through the outbox → NATS → consumer pipeline.
func FormatTraceParent(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() {
		return ""
	}
	return fmt.Sprintf("%s-%s-%s-%s",
		traceParentVersion,
		sc.TraceID().String(),
		sc.SpanID().String(),
		formatTraceFlags(sc.TraceFlags()),
	)
}

// formatTraceFlags returns the trace flags as a 2-character hex string per the
// W3C Trace Context specification.
func formatTraceFlags(flags trace.TraceFlags) string {
	return fmt.Sprintf("%02x", byte(flags))
}

// InjectTraceContext injects the trace context from ctx into the carrier using
// the global text map propagator. This is the counterpart of
// ExtractTraceContext and is used to propagate trace context across process
// boundaries via map-based carriers (e.g. NATS message metadata).
func InjectTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) {
	otel.GetTextMapPropagator().Inject(ctx, carrier)
}

// ExtractTraceContext extracts trace context from the carrier using the global
// text map propagator and returns a new context enriched with the extracted
// span context. Consumers use this to link their processing spans to the
// originating business operation.
func ExtractTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}
