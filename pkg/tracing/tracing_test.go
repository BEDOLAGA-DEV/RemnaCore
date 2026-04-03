package tracing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
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
