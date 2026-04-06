package tracing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestStartSpan_ReturnsValidSpan(t *testing.T) {
	ctx := context.Background()
	spanCtx, span := StartSpan(ctx, "test.operation")

	assert.NotNil(t, span)
	assert.NotNil(t, spanCtx)

	// Clean up.
	span.End()
}

func TestTracerName_IsSet(t *testing.T) {
	assert.Equal(t, "remnacore", TracerName)
}

func TestFormatTraceParent_NoActiveSpan(t *testing.T) {
	ctx := context.Background()
	tp := FormatTraceParent(ctx)

	assert.Empty(t, tp, "must return empty string when no valid span context")
}

func TestFormatTraceParent_WithActiveSpan(t *testing.T) {
	// Create a real tracer provider so we get valid span contexts.
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tr := tp.Tracer("test")
	ctx, span := tr.Start(context.Background(), "test.op")
	defer span.End()

	sc := span.SpanContext()
	require.True(t, sc.IsValid(), "span context must be valid with a real tracer provider")

	traceparent := FormatTraceParent(ctx)

	assert.NotEmpty(t, traceparent, "must return non-empty traceparent with valid span")
	assert.Contains(t, traceparent, sc.TraceID().String(), "must contain trace ID")
	assert.Contains(t, traceparent, sc.SpanID().String(), "must contain span ID")
	assert.True(t, len(traceparent) > 0 && traceparent[:2] == traceParentVersion,
		"must start with version 00")
}

func TestInjectExtract_RoundTrip(t *testing.T) {
	// Create a real tracer provider so we get valid span contexts.
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tr := tp.Tracer("test")
	ctx, span := tr.Start(context.Background(), "test.op")
	defer span.End()

	originalSC := span.SpanContext()
	require.True(t, originalSC.IsValid())

	// Use W3C propagator for the test (simulating production setup).
	prop := propagation.TraceContext{}

	// Inject into carrier.
	carrier := propagation.MapCarrier{}
	prop.Inject(ctx, carrier)

	// Verify the carrier has traceparent set.
	assert.NotEmpty(t, carrier.Get("traceparent"), "carrier must contain traceparent")

	// Extract from carrier into a fresh context.
	extractedCtx := prop.Extract(context.Background(), carrier)
	extractedSC := trace.SpanContextFromContext(extractedCtx)

	assert.Equal(t, originalSC.TraceID(), extractedSC.TraceID(),
		"trace ID must survive inject/extract round-trip")
	assert.Equal(t, originalSC.SpanID(), extractedSC.SpanID(),
		"span ID must survive inject/extract round-trip")
}
