package plugin

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRunner implements WASMRunner for testing.
type mockRunner struct {
	callFn  func(ctx context.Context, funcName string, input []byte) ([]byte, error)
	closed  bool
}

func (m *mockRunner) Call(ctx context.Context, funcName string, input []byte) ([]byte, error) {
	if m.callFn != nil {
		return m.callFn(ctx, funcName, input)
	}
	return nil, nil
}

func (m *mockRunner) Close() error {
	m.closed = true
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func mockFactory(callFn func(ctx context.Context, funcName string, input []byte) ([]byte, error)) WASMRunnerFactory {
	return func(wasmBytes []byte, config map[string]string) (WASMRunner, error) {
		return &mockRunner{callFn: callFn}, nil
	}
}

func testPlugin(slug string) *Plugin {
	m := &Manifest{
		Plugin: ManifestPlugin{ID: slug, Name: "Test", Version: "1.0.0"},
		Hooks:  ManifestHooks{Sync: []string{"hook.test"}},
	}
	p, _ := NewPlugin(m, []byte("fake-wasm"))
	return p
}

func TestRuntimePool_LoadPlugin(t *testing.T) {
	rp := NewRuntimePool(testLogger(), mockFactory(nil))

	p := testPlugin("test-plugin")
	err := rp.LoadPlugin(p)
	require.NoError(t, err)

	slugs := rp.LoadedSlugs()
	assert.Contains(t, slugs, "test-plugin")
}

func TestRuntimePool_LoadPlugin_NilPlugin(t *testing.T) {
	rp := NewRuntimePool(testLogger(), mockFactory(nil))

	err := rp.LoadPlugin(nil)
	require.Error(t, err)
}

func TestRuntimePool_LoadPlugin_ReplacesExisting(t *testing.T) {
	runner1 := &mockRunner{}
	callCount := 0
	factory := func(wasmBytes []byte, config map[string]string) (WASMRunner, error) {
		callCount++
		if callCount == 1 {
			return runner1, nil
		}
		return &mockRunner{}, nil
	}

	rp := NewRuntimePool(testLogger(), factory)

	p := testPlugin("test-plugin")
	require.NoError(t, rp.LoadPlugin(p))
	require.NoError(t, rp.LoadPlugin(p))

	// The first runner should have been closed.
	assert.True(t, runner1.closed)
	assert.Len(t, rp.LoadedSlugs(), 1)
}

func TestRuntimePool_UnloadPlugin(t *testing.T) {
	runner := &mockRunner{}
	factory := func(wasmBytes []byte, config map[string]string) (WASMRunner, error) {
		return runner, nil
	}

	rp := NewRuntimePool(testLogger(), factory)

	p := testPlugin("test-plugin")
	require.NoError(t, rp.LoadPlugin(p))
	require.NoError(t, rp.UnloadPlugin("test-plugin"))

	assert.True(t, runner.closed)
	assert.Empty(t, rp.LoadedSlugs())
}

func TestRuntimePool_UnloadPlugin_NotFound(t *testing.T) {
	rp := NewRuntimePool(testLogger(), mockFactory(nil))

	err := rp.UnloadPlugin("nonexistent")
	require.ErrorIs(t, err, ErrPluginNotFound)
}

func TestRuntimePool_GetInstance(t *testing.T) {
	rp := NewRuntimePool(testLogger(), mockFactory(nil))

	p := testPlugin("test-plugin")
	require.NoError(t, rp.LoadPlugin(p))

	inst, err := rp.GetInstance("test-plugin")
	require.NoError(t, err)
	assert.Equal(t, "test-plugin", inst.Slug)
	assert.NotNil(t, inst.Runner)
}

func TestRuntimePool_GetInstance_NotFound(t *testing.T) {
	rp := NewRuntimePool(testLogger(), mockFactory(nil))

	_, err := rp.GetInstance("nonexistent")
	require.ErrorIs(t, err, ErrPluginNotFound)
}

func TestRuntimePool_CallHook(t *testing.T) {
	expected := []byte(`{"action":"continue"}`)
	factory := mockFactory(func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
		assert.Equal(t, "hook.test", funcName)
		return expected, nil
	})

	rp := NewRuntimePool(testLogger(), factory)

	p := testPlugin("test-plugin")
	require.NoError(t, rp.LoadPlugin(p))

	output, err := rp.CallHook(context.Background(), "test-plugin", "hook.test", []byte("input"))
	require.NoError(t, err)
	assert.Equal(t, expected, output)
}

func TestRuntimePool_CallHook_NotFound(t *testing.T) {
	rp := NewRuntimePool(testLogger(), mockFactory(nil))

	_, err := rp.CallHook(context.Background(), "nonexistent", "hook.test", nil)
	require.ErrorIs(t, err, ErrPluginNotFound)
}

func TestRuntimePool_CallHook_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	rp := NewRuntimePool(testLogger(), mockFactory(nil))

	p := testPlugin("test-plugin")
	require.NoError(t, rp.LoadPlugin(p))

	_, err := rp.CallHook(ctx, "test-plugin", "hook.test", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrHookTimeout)
}

func TestRuntimePool_CallHook_RunnerError(t *testing.T) {
	factory := mockFactory(func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
		return nil, errors.New("wasm trap")
	})

	rp := NewRuntimePool(testLogger(), factory)

	p := testPlugin("test-plugin")
	require.NoError(t, rp.LoadPlugin(p))

	_, err := rp.CallHook(context.Background(), "test-plugin", "hook.test", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wasm trap")
}

func TestRuntimePool_LoadPlugin_NilFactory(t *testing.T) {
	rp := NewRuntimePool(testLogger(), nil)

	p := testPlugin("test-plugin")
	err := rp.LoadPlugin(p)
	require.NoError(t, err)

	// Runner should be nil; CallHook should fail gracefully.
	_, err = rp.CallHook(context.Background(), "test-plugin", "hook.test", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no WASM runner")
}

func TestRuntimePool_LoadPlugin_FactoryError(t *testing.T) {
	factory := func(wasmBytes []byte, config map[string]string) (WASMRunner, error) {
		return nil, errors.New("compilation error")
	}

	rp := NewRuntimePool(testLogger(), factory)

	p := testPlugin("test-plugin")
	err := rp.LoadPlugin(p)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrWASMCompilationFailed)
}
