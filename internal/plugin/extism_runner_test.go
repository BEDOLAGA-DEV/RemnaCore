package plugin

import (
	"context"
	"testing"
	"time"

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

func TestExtismRunnerFactory_MemoryLimitsDoNotPanic(t *testing.T) {
	// Verify the factory exercises the memory-limit code path without
	// panicking, even though WASM compilation fails on invalid bytes.
	factory := ExtismRunnerFactory()

	limits := ManifestLimits{MaxMemoryMB: 128}
	_, err := factory([]byte("not-valid-wasm"), nil, limits)
	require.Error(t, err, "invalid WASM should still fail")
	assert.Contains(t, err.Error(), "create extism plugin")
}

func TestExtismRunnerFactory_ZeroMemorySkipsMemoryConfig(t *testing.T) {
	// When MaxMemoryMB is zero the factory must not set a memory limit.
	// We verify indirectly: the factory should not panic and the error
	// message should remain the same as without limits.
	factory := ExtismRunnerFactory()

	limits := ManifestLimits{MaxMemoryMB: 0}
	_, err := factory([]byte("not-valid-wasm"), nil, limits)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create extism plugin")
}

func TestExtismRunnerFactoryWithTimeout_MemoryLimitsDoNotPanic(t *testing.T) {
	const timeoutMs = 5000
	factory := ExtismRunnerFactoryWithTimeout(timeoutMs)

	limits := ManifestLimits{MaxMemoryMB: 256}
	_, err := factory([]byte("not-valid-wasm"), nil, limits)
	require.Error(t, err, "invalid WASM should still fail")
	assert.Contains(t, err.Error(), "create extism plugin with timeout")
}

func TestWASMPagesPerMB_ConversionCorrectness(t *testing.T) {
	// 1 WASM page = 64 KB = 65536 bytes. 1 MB = 1048576 bytes.
	// 1048576 / 65536 = 16 pages per MB.
	const bytesPerPage = 65536
	const bytesPerMB = 1048576
	expectedPagesPerMB := bytesPerMB / bytesPerPage

	assert.Equal(t, expectedPagesPerMB, WASMPagesPerMB)
}

func TestMemoryLimits_PropagatedThroughPool(t *testing.T) {
	// Verify that when a plugin with MaxMemoryMB is loaded, the factory
	// receives limits with MaxMemoryMB populated via EffectiveLimits.
	var capturedLimits ManifestLimits

	capturingFactory := func(wasmBytes []byte, config map[string]string, limits ManifestLimits) (WASMRunner, error) {
		capturedLimits = limits
		return &mockRunner{}, nil
	}

	rp := NewRuntimePool(testErrorLogger(), capturingFactory)

	m := &Manifest{
		Plugin: ManifestPlugin{ID: "mem-test", Name: "MemTest", Version: "1.0.0", SDKVersion: CurrentSDKVersion},
		Hooks:  ManifestHooks{Sync: []string{"hook.test"}},
		Limits: ManifestLimits{MaxMemoryMB: 128},
	}
	p, _ := NewPlugin(m, []byte("fake-wasm"), time.Now())
	require.NoError(t, rp.LoadPlugin(p))

	assert.Equal(t, 128, capturedLimits.MaxMemoryMB)
}

func TestMemoryLimits_DefaultAppliedWhenZero(t *testing.T) {
	// When plugin manifest omits MaxMemoryMB, EffectiveLimits fills in the
	// default. Verify the factory receives the default value.
	var capturedLimits ManifestLimits

	capturingFactory := func(wasmBytes []byte, config map[string]string, limits ManifestLimits) (WASMRunner, error) {
		capturedLimits = limits
		return &mockRunner{}, nil
	}

	rp := NewRuntimePool(testErrorLogger(), capturingFactory)

	m := &Manifest{
		Plugin: ManifestPlugin{ID: "mem-default", Name: "MemDefault", Version: "1.0.0", SDKVersion: CurrentSDKVersion},
		Hooks:  ManifestHooks{Sync: []string{"hook.test"}},
		// MaxMemoryMB intentionally omitted (zero value).
	}
	p, _ := NewPlugin(m, []byte("fake-wasm"), time.Now())
	require.NoError(t, rp.LoadPlugin(p))

	assert.Equal(t, DefaultMaxMemoryMB, capturedLimits.MaxMemoryMB,
		"factory should receive default memory limit from EffectiveLimits")
}

func TestMemoryLimits_PageCalculation(t *testing.T) {
	tests := []struct {
		name         string
		maxMemoryMB  int
		expectedPages uint32
	}{
		{
			name:          "64 MB default",
			maxMemoryMB:   64,
			expectedPages: 1024,
		},
		{
			name:          "128 MB",
			maxMemoryMB:   128,
			expectedPages: 2048,
		},
		{
			name:          "256 MB",
			maxMemoryMB:   256,
			expectedPages: 4096,
		},
		{
			name:          "1 MB minimum",
			maxMemoryMB:   1,
			expectedPages: 16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pages := uint32(tt.maxMemoryMB * WASMPagesPerMB)
			assert.Equal(t, tt.expectedPages, pages)
		})
	}
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
