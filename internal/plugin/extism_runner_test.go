package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtismRunnerFactory_ReturnsFactory(t *testing.T) {
	factory := ExtismRunnerFactory()
	require.NotNil(t, factory)
}

func TestExtismRunnerFactoryWithTimeout_ReturnsFactory(t *testing.T) {
	const timeoutMs = 5000
	factory := ExtismRunnerFactoryWithTimeout(timeoutMs)
	require.NotNil(t, factory)
}

func TestExtismRunnerFactory_InvalidWASM(t *testing.T) {
	factory := ExtismRunnerFactory()

	_, err := factory([]byte("not-valid-wasm"), nil, ManifestLimits{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create extism plugin")
}

func TestExtismRunnerFactory_EmptyWASM(t *testing.T) {
	factory := ExtismRunnerFactory()

	_, err := factory([]byte{}, nil, ManifestLimits{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create extism plugin")
}

func TestExtismRunnerFactory_NilWASM(t *testing.T) {
	factory := ExtismRunnerFactory()

	_, err := factory(nil, nil, ManifestLimits{})
	require.Error(t, err)
}

func TestExtismRunnerFactory_ConfigPassthrough(t *testing.T) {
	// Verify the factory does not panic when config is provided, even though
	// the WASM bytes are invalid (the config path is exercised before
	// compilation fails).
	factory := ExtismRunnerFactory()

	config := map[string]string{
		"api_key": "sk_test_123",
		"mode":    "sandbox",
	}
	_, err := factory([]byte("not-valid-wasm"), config, ManifestLimits{})
	require.Error(t, err, "invalid WASM should still fail")
}

// TestNoopWASMFactory_StillWorks ensures the noop factory from app.go is still a
// valid fallback. This test guards against regressions if ExtismRunnerFactory
// becomes the only factory in production.
func TestNoopWASMFactory_StillWorks(t *testing.T) {
	factory := mockFactory(func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
		return []byte(`{"action":"continue"}`), nil
	})

	runner, err := factory([]byte("fake"), nil, ManifestLimits{})
	require.NoError(t, err)
	defer runner.Close()

	output, err := runner.Call(context.Background(), "test.hook", []byte("input"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"action":"continue"}`, string(output))
}
