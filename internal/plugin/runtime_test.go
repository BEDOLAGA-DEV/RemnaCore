package plugin

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRunner implements WASMRunner for testing.
type mockRunner struct {
	callFn func(ctx context.Context, funcName string, input []byte) ([]byte, error)
	closed bool
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

func mockFactory(callFn func(ctx context.Context, funcName string, input []byte) ([]byte, error)) WASMRunnerFactory {
	return func(wasmBytes []byte, config map[string]string) (WASMRunner, error) {
		return &mockRunner{callFn: callFn}, nil
	}
}

func testPlugin(slug string) *Plugin {
	m := &Manifest{
		Plugin: ManifestPlugin{ID: slug, Name: "Test", Version: "1.0.0", SDKVersion: CurrentSDKVersion},
		Hooks:  ManifestHooks{Sync: []string{"hook.test"}},
	}
	p, _ := NewPlugin(m, []byte("fake-wasm"), time.Now())
	return p
}

func testPluginWithPoolSize(slug string, poolSize int) *Plugin {
	m := &Manifest{
		Plugin: ManifestPlugin{ID: slug, Name: "Test", Version: "1.0.0", SDKVersion: CurrentSDKVersion},
		Hooks:  ManifestHooks{Sync: []string{"hook.test"}},
		Limits: ManifestLimits{PoolSize: poolSize},
	}
	p, _ := NewPlugin(m, []byte("fake-wasm"), time.Now())
	return p
}

func TestRuntimePool_LoadPlugin(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), mockFactory(nil))

	p := testPlugin("test-plugin")
	err := rp.LoadPlugin(p)
	require.NoError(t, err)

	slugs := rp.LoadedSlugs()
	assert.Contains(t, slugs, "test-plugin")
}

func TestRuntimePool_LoadPlugin_CreatesPool(t *testing.T) {
	var instanceCount atomic.Int32
	factory := func(wasmBytes []byte, config map[string]string) (WASMRunner, error) {
		instanceCount.Add(1)
		return &mockRunner{}, nil
	}

	rp := NewRuntimePool(testErrorLogger(), factory)

	p := testPlugin("test-plugin")
	err := rp.LoadPlugin(p)
	require.NoError(t, err)

	// Default pool size should create DefaultPoolSize instances.
	assert.Equal(t, int32(DefaultPoolSize), instanceCount.Load())
}

func TestRuntimePool_LoadPlugin_CustomPoolSize(t *testing.T) {
	var instanceCount atomic.Int32
	factory := func(wasmBytes []byte, config map[string]string) (WASMRunner, error) {
		instanceCount.Add(1)
		return &mockRunner{}, nil
	}

	rp := NewRuntimePool(testErrorLogger(), factory)

	p := testPluginWithPoolSize("test-plugin", 8)
	err := rp.LoadPlugin(p)
	require.NoError(t, err)

	assert.Equal(t, int32(8), instanceCount.Load())
}

func TestRuntimePool_LoadPlugin_PoolSizeCappedAtMax(t *testing.T) {
	var instanceCount atomic.Int32
	factory := func(wasmBytes []byte, config map[string]string) (WASMRunner, error) {
		instanceCount.Add(1)
		return &mockRunner{}, nil
	}

	rp := NewRuntimePool(testErrorLogger(), factory)

	// Request more than MaxPoolSize.
	p := testPluginWithPoolSize("test-plugin", MaxPoolSize+10)
	err := rp.LoadPlugin(p)
	require.NoError(t, err)

	assert.Equal(t, int32(MaxPoolSize), instanceCount.Load())
}

func TestRuntimePool_LoadPlugin_NilPlugin(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), mockFactory(nil))

	err := rp.LoadPlugin(nil)
	require.Error(t, err)
}

func TestRuntimePool_LoadPlugin_ReplacesExisting(t *testing.T) {
	var closedCount atomic.Int32
	factory := func(wasmBytes []byte, config map[string]string) (WASMRunner, error) {
		return &mockRunner{
			callFn: func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
				return nil, nil
			},
			// Track closes via the atomic counter instead of the struct field
			// since we create multiple instances.
		}, nil
	}

	// We'll track closes by wrapping. Use a simpler approach: count instances
	// created for each Load call.
	var firstBatchCount, secondBatchCount atomic.Int32
	loadCount := 0
	factoryWithTracking := func(wasmBytes []byte, config map[string]string) (WASMRunner, error) {
		runner := &trackingMockRunner{onClose: func() { closedCount.Add(1) }}
		if loadCount == 0 {
			firstBatchCount.Add(1)
		} else {
			secondBatchCount.Add(1)
		}
		_ = factory // suppress unused
		return runner, nil
	}

	rp := NewRuntimePool(testErrorLogger(), factoryWithTracking)

	p := testPlugin("test-plugin")
	require.NoError(t, rp.LoadPlugin(p))
	loadCount = 1
	require.NoError(t, rp.LoadPlugin(p))

	// All first-batch runners should have been closed.
	assert.Equal(t, firstBatchCount.Load(), closedCount.Load())
	assert.Len(t, rp.LoadedSlugs(), 1)
}

// trackingMockRunner is a mock runner that calls onClose when Close is called.
type trackingMockRunner struct {
	onClose func()
}

func (m *trackingMockRunner) Call(ctx context.Context, funcName string, input []byte) ([]byte, error) {
	return nil, nil
}

func (m *trackingMockRunner) Close() error {
	if m.onClose != nil {
		m.onClose()
	}
	return nil
}

func TestRuntimePool_UnloadPlugin(t *testing.T) {
	var closedCount atomic.Int32
	factory := func(wasmBytes []byte, config map[string]string) (WASMRunner, error) {
		return &trackingMockRunner{onClose: func() { closedCount.Add(1) }}, nil
	}

	rp := NewRuntimePool(testErrorLogger(), factory)

	p := testPlugin("test-plugin")
	require.NoError(t, rp.LoadPlugin(p))
	require.NoError(t, rp.UnloadPlugin("test-plugin"))

	// All pool instances should have been closed.
	assert.Equal(t, int32(DefaultPoolSize), closedCount.Load())
	assert.Empty(t, rp.LoadedSlugs())
}

func TestRuntimePool_UnloadPlugin_NotFound(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), mockFactory(nil))

	err := rp.UnloadPlugin("nonexistent")
	require.ErrorIs(t, err, ErrPluginNotFound)
}

func TestRuntimePool_GetInstance(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), mockFactory(nil))

	p := testPlugin("test-plugin")
	require.NoError(t, rp.LoadPlugin(p))

	inst, err := rp.GetInstance("test-plugin")
	require.NoError(t, err)
	assert.Equal(t, "test-plugin", inst.Slug)
	assert.NotNil(t, inst.Manifest)
}

func TestRuntimePool_GetInstance_NotFound(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), mockFactory(nil))

	_, err := rp.GetInstance("nonexistent")
	require.ErrorIs(t, err, ErrPluginNotFound)
}

func TestRuntimePool_CallHook(t *testing.T) {
	expected := []byte(`{"action":"continue"}`)
	factory := mockFactory(func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
		assert.Equal(t, "hook.test", funcName)
		return expected, nil
	})

	rp := NewRuntimePool(testErrorLogger(), factory)

	p := testPlugin("test-plugin")
	require.NoError(t, rp.LoadPlugin(p))

	output, err := rp.CallHook(context.Background(), "test-plugin", "hook.test", []byte("input"))
	require.NoError(t, err)
	assert.Equal(t, expected, output)
}

func TestRuntimePool_CallHook_NotFound(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), mockFactory(nil))

	_, err := rp.CallHook(context.Background(), "nonexistent", "hook.test", nil)
	require.ErrorIs(t, err, ErrPluginNotFound)
}

func TestRuntimePool_CallHook_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	rp := NewRuntimePool(testErrorLogger(), mockFactory(nil))

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

	rp := NewRuntimePool(testErrorLogger(), factory)

	p := testPlugin("test-plugin")
	require.NoError(t, rp.LoadPlugin(p))

	_, err := rp.CallHook(context.Background(), "test-plugin", "hook.test", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wasm trap")
}

func TestRuntimePool_LoadPlugin_NilFactory(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), nil)

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

	rp := NewRuntimePool(testErrorLogger(), factory)

	p := testPlugin("test-plugin")
	err := rp.LoadPlugin(p)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrWASMCompilationFailed)
}

func TestRuntimePool_ConcurrentCalls(t *testing.T) {
	// Verify that multiple goroutines can call the same plugin concurrently
	// without serialization. Each call takes a small duration; if they were
	// serialized, total time would be N * duration. With pooling, it should
	// be much less.
	const concurrency = 8
	const callDuration = 50 * time.Millisecond

	var activeCount atomic.Int32
	var maxActive atomic.Int32

	factory := func(wasmBytes []byte, config map[string]string) (WASMRunner, error) {
		return &mockRunner{
			callFn: func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
				current := activeCount.Add(1)
				// Track the maximum concurrent active calls.
				for {
					old := maxActive.Load()
					if current <= old || maxActive.CompareAndSwap(old, current) {
						break
					}
				}
				time.Sleep(callDuration)
				activeCount.Add(-1)
				return []byte(`{"action":"continue"}`), nil
			},
		}, nil
	}

	rp := NewRuntimePool(testErrorLogger(), factory)

	// Create a plugin with pool size matching our concurrency.
	p := testPluginWithPoolSize("concurrent-plugin", concurrency)
	require.NoError(t, rp.LoadPlugin(p))

	var wg sync.WaitGroup
	start := time.Now()

	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := rp.CallHook(context.Background(), "concurrent-plugin", "hook.test", nil)
			assert.NoError(t, err)
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	// With full concurrency, all calls should complete in roughly callDuration
	// (not concurrency * callDuration). Allow 2x tolerance for CI flakiness.
	maxExpected := callDuration * 2
	assert.Less(t, elapsed, maxExpected,
		"concurrent calls took %v, expected < %v (serialization detected)", elapsed, maxExpected)

	// At least 2 runners should have been active simultaneously.
	assert.Greater(t, maxActive.Load(), int32(1),
		"max concurrent active runners was %d, expected > 1", maxActive.Load())
}

func TestRuntimePool_ConcurrentCalls_PoolExhaustion(t *testing.T) {
	// When more goroutines than pool size try to call, excess callers should
	// block until a runner is released — not error immediately.
	const poolSize = 2
	const totalCallers = 4

	var callsCompleted atomic.Int32

	factory := func(wasmBytes []byte, config map[string]string) (WASMRunner, error) {
		return &mockRunner{
			callFn: func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
				time.Sleep(20 * time.Millisecond)
				callsCompleted.Add(1)
				return []byte(`{"action":"continue"}`), nil
			},
		}, nil
	}

	rp := NewRuntimePool(testErrorLogger(), factory)

	p := testPluginWithPoolSize("small-pool-plugin", poolSize)
	require.NoError(t, rp.LoadPlugin(p))

	var wg sync.WaitGroup
	for range totalCallers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err := rp.CallHook(ctx, "small-pool-plugin", "hook.test", nil)
			assert.NoError(t, err)
		}()
	}

	wg.Wait()
	assert.Equal(t, int32(totalCallers), callsCompleted.Load())
}

func TestPluginInstancePool_AcquireReleaseCycle(t *testing.T) {
	factory := mockFactory(nil)
	pool, err := newPluginInstancePool("test", factory, []byte("wasm"), nil, nil, 2)
	require.NoError(t, err)
	defer pool.Close()

	ctx := context.Background()

	// Acquire both instances.
	r1, err := pool.Acquire(ctx)
	require.NoError(t, err)

	r2, err := pool.Acquire(ctx)
	require.NoError(t, err)

	// Pool should be empty now. A third acquire with a short timeout should fail.
	shortCtx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()

	_, err = pool.Acquire(shortCtx)
	assert.Error(t, err, "should fail when pool exhausted")

	// Release one back.
	pool.Release(r1)

	// Now we should be able to acquire again.
	r3, err := pool.Acquire(ctx)
	require.NoError(t, err)
	assert.NotNil(t, r3)

	pool.Release(r2)
	pool.Release(r3)
}

func TestPluginInstancePool_FactoryError_CleansUp(t *testing.T) {
	var closedCount atomic.Int32
	callCount := 0

	factory := func(wasmBytes []byte, config map[string]string) (WASMRunner, error) {
		callCount++
		if callCount == 3 {
			return nil, errors.New("factory error on 3rd instance")
		}
		return &trackingMockRunner{onClose: func() { closedCount.Add(1) }}, nil
	}

	_, err := newPluginInstancePool("test", factory, []byte("wasm"), nil, nil, 4)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create instance 2")

	// The 2 successfully created instances should have been closed.
	assert.Equal(t, int32(2), closedCount.Load())
}

func TestRuntimePool_SetRunnerForTest(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), mockFactory(nil))

	p := testPlugin("test-plugin")
	require.NoError(t, rp.LoadPlugin(p))

	expected := []byte("test-output")
	rp.SetRunnerForTest("test-plugin", &mockRunner{
		callFn: func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return expected, nil
		},
	})

	output, err := rp.CallHook(context.Background(), "test-plugin", "hook.test", nil)
	require.NoError(t, err)
	assert.Equal(t, expected, output)
}
