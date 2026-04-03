package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
)

func TestInitTracer_NoopWhenEndpointEmpty(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{
			Version: "test",
		},
		Tracing: config.TracingConfig{
			Endpoint: "",
		},
	}

	logger := NewLogger("debug", FormatConsole)

	shutdown, err := InitTracer(context.Background(), cfg, logger)
	require.NoError(t, err)
	assert.NotNil(t, shutdown)

	// Calling shutdown on noop should be a no-op with no error.
	err = shutdown(context.Background())
	require.NoError(t, err)
}
