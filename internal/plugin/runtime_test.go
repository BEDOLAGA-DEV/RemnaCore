package plugin

import (
	"context"
	"errors"
	"fmt"
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
	return func(_ string, wasmBytes []byte, config map[string]string, limits ManifestLimits) (WASMRunner, error) {
		return &mockRunner{callFn: callFn}, nil
	}
}

func testPlugin(slug string) *Plugin {
	m := &Manifest{
		Plugin: ManifestPlugin{ID: slug, Name: "Test", Version: "1.0.0", SDKVersion: CurrentSDKVersion},
		Hooks:  ManifestHooks{Sync: []string{"plugin.test"}},
	}
	p, _ := NewPlugin(m, []byte("fake-wasm"), time.Now())
	return p
}

func testPluginWithPoolSize(slug string, poolSize int) *Plugin {
	m := &Manifest{
		Plugin: ManifestPlugin{ID: slug, Name: "Test", Version: "1.0.0", SDKVersion: CurrentSDKVersion},
		Hooks:  ManifestHooks{Sync: []string{"plugin.test"}},
		Limits: ManifestLimits{PoolSize: poolSize},
	}
	p, _ := NewPlugin(m, []byte("fake-wasm"), time.Now())
	return p
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
	factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
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
	factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
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
	factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
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
	rp := NewRuntimePool(testErrorLogger(), mockFactory(nil))

	p := testPlugin("test-plugin")
	require.NoError(t, rp.LoadPlugin(p))
	require.NoError(t, rp.LoadPlugin(p))

	// After replacement, the old pool is retired (kept alive for the grace
	// period). The new pool is serving and the slug is still registered.
	assert.Len(t, rp.LoadedSlugs(), 1)
	assert.Equal(t, 1, rp.RetiredPoolCount(),
		"replaced pool should be retired, not drained immediately")
}

func TestRuntimePool_UnloadPlugin(t *testing.T) {
	var closedCount atomic.Int32
	factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
		return &trackingMockRunner{onClose: func() { closedCount.Add(1) }}, nil
	}

	rp := NewRuntimePool(testErrorLogger(), factory)

	p := testPlugin("test-plugin")
	require.NoError(t, rp.LoadPlugin(p))
	require.NoError(t, rp.UnloadPlugin("test-plugin"))

	// Drain runs in the background. Wait briefly for it to complete.
	assert.Eventually(t, func() bool {
		return closedCount.Load() == int32(DefaultPoolSize)
	}, time.Second, 5*time.Millisecond,
		"all pool instances should be closed via drain")
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

func TestRuntimePool_CallHook_RunnerError_NonCorruption(t *testing.T) {
	// A normal (non-WASM-corruption) error returns the runner to the pool.
	factory := mockFactory(func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
		return nil, errors.New("business logic error")
	})

	rp := NewRuntimePool(testErrorLogger(), factory)

	p := testPlugin("test-plugin")
	require.NoError(t, rp.LoadPlugin(p))

	_, err := rp.CallHook(context.Background(), "test-plugin", "hook.test", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "business logic error")

	// Runner should still be usable (returned to pool).
	_, err = rp.CallHook(context.Background(), "test-plugin", "hook.test", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "business logic error")
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
	factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
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

	factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
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

	factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
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

func TestPluginInstancePool_PartialFactoryError_GracefulDegradation(t *testing.T) {
	callCount := 0

	factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
		callCount++
		if callCount == 3 {
			return nil, errors.New("factory error on 3rd instance")
		}
		return &mockRunner{}, nil
	}

	pool, err := newPluginInstancePool("test", factory, []byte("wasm"), nil, nil, 4)
	require.NoError(t, err, "partial pool failure should not return error when at least 1 runner created")

	// 3 out of 4 instances should have been created synchronously
	// (the 4th may be filling in background).
	ctx := context.Background()
	r1, err := pool.Acquire(ctx)
	require.NoError(t, err)
	r2, err := pool.Acquire(ctx)
	require.NoError(t, err)
	r3, err := pool.Acquire(ctx)
	require.NoError(t, err)

	pool.Release(r1)
	pool.Release(r2)
	pool.Release(r3)
}

func TestPluginInstancePool_TotalFactoryFailure_ReturnsError(t *testing.T) {
	var closedCount atomic.Int32

	factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
		return nil, errors.New("every factory call fails")
	}

	_, err := newPluginInstancePool("test", factory, []byte("wasm"), nil, nil, 4)
	require.Error(t, err, "pool with zero created runners should fail")
	assert.Contains(t, err.Error(), "every factory call fails")

	// No instances to close since none were created.
	assert.Equal(t, int32(0), closedCount.Load())
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

// --- Drain Tests ---

func TestPluginInstancePool_DrainWaitsForActive(t *testing.T) {
	factory := mockFactory(nil)
	pool, err := newPluginInstancePool("drain-test", factory, []byte("wasm"), nil, nil, 2)
	require.NoError(t, err)

	ctx := context.Background()

	// Acquire an instance (simulate an in-flight request).
	runner, err := pool.Acquire(ctx)
	require.NoError(t, err)

	// Start drain in background.
	drainDone := make(chan struct{})
	go func() {
		_ = pool.Drain(context.Background())
		close(drainDone)
	}()

	// Verify drain is blocking (not completed yet).
	select {
	case <-drainDone:
		t.Fatal("drain completed before in-flight runner was released")
	case <-time.After(50 * time.Millisecond):
		// Expected: drain should be waiting.
	}

	// Release the runner — drain should complete.
	pool.Release(runner)

	select {
	case <-drainDone:
		// Expected: drain completed.
	case <-time.After(time.Second):
		t.Fatal("drain did not complete after all runners were released")
	}
}

func TestPluginInstancePool_DrainRejectsNewAcquire(t *testing.T) {
	factory := mockFactory(nil)
	pool, err := newPluginInstancePool("drain-reject", factory, []byte("wasm"), nil, nil, 2)
	require.NoError(t, err)

	// Start drain (no active runners, so it completes quickly but marks as draining).
	_ = pool.Drain(context.Background())

	// Try to acquire — should fail because pool is draining.
	_, err = pool.Acquire(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPluginDraining)
}

func TestPluginInstancePool_DrainIdempotent(t *testing.T) {
	factory := mockFactory(nil)
	pool, err := newPluginInstancePool("drain-idempotent", factory, []byte("wasm"), nil, nil, 2)
	require.NoError(t, err)

	// Drain twice should not panic.
	err = pool.Drain(context.Background())
	require.NoError(t, err)

	err = pool.Drain(context.Background())
	require.NoError(t, err)
}

func TestPluginInstancePool_DrainClosesIdleRunners(t *testing.T) {
	var closedCount atomic.Int32
	factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
		return &trackingMockRunner{onClose: func() { closedCount.Add(1) }}, nil
	}

	const poolSize = 3
	pool, err := newPluginInstancePool("drain-idle", factory, []byte("wasm"), nil, nil, poolSize)
	require.NoError(t, err)

	// Drain without any active runners — all idle runners should be closed.
	err = pool.Drain(context.Background())
	require.NoError(t, err)

	assert.Equal(t, int32(poolSize), closedCount.Load())
}

func TestLoadPlugin_GracefulDrainOnReplace(t *testing.T) {
	// When a plugin is replaced, the old pool is retired with a grace period
	// so flow-pinned callers can complete. The new pool serves immediately.
	var oldClosedCount atomic.Int32

	factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
		return &trackingMockRunner{onClose: func() { oldClosedCount.Add(1) }}, nil
	}

	rp := NewRuntimePool(testErrorLogger(), factory)

	p := testPlugin("drain-replace")
	require.NoError(t, rp.LoadPlugin(p))

	// Acquire a runner from the first pool (simulate in-flight request).
	rp.mu.RLock()
	oldPool := rp.plugins["drain-replace"]
	rp.mu.RUnlock()

	runner, err := oldPool.Acquire(context.Background())
	require.NoError(t, err)

	// Load the plugin again — old pool is retired with a grace period.
	require.NoError(t, rp.LoadPlugin(p))

	// Old pool is in the grace period — no runners should be closed yet.
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, int32(0), oldClosedCount.Load(),
		"no runners should be closed during grace period")

	// Release the in-flight runner — it returns to the retired pool normally
	// since drain hasn't started yet.
	oldPool.Release(runner)

	// New pool should be fully operational.
	output, err := rp.CallHook(context.Background(), "drain-replace", "hook.test", nil)
	require.NoError(t, err)
	assert.Nil(t, output) // trackingMockRunner returns nil

	// Verify retired pool still exists (grace period is 30s).
	assert.Equal(t, 1, rp.RetiredPoolCount(),
		"retired pool should still exist during grace period")
}

// --- WASM Health Check Tests ---

func TestCallHook_CorruptedRunnerDiscarded(t *testing.T) {
	callCount := 0
	var replacedCount atomic.Int32

	factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
		callCount++
		return &mockRunner{
			callFn: func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
				// First call returns a corruption error.
				return nil, fmt.Errorf("wasm: unreachable instruction")
			},
		}, nil
	}

	rp := NewRuntimePool(testErrorLogger(), factory)

	p := testPluginWithPoolSize("corrupt-test", 1)
	require.NoError(t, rp.LoadPlugin(p))

	// Patch factory to track replacements after initial load.
	rp.mu.Lock()
	pool := rp.plugins["corrupt-test"]
	originalFactory := pool.factory
	pool.factory = func(slug string, wasmBytes []byte, config map[string]string, limits ManifestLimits) (WASMRunner, error) {
		replacedCount.Add(1)
		return originalFactory(slug, wasmBytes, config, limits)
	}
	rp.mu.Unlock()

	_, err := rp.CallHook(context.Background(), "corrupt-test", "hook.test", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "corrupted")

	// Replacement should be created asynchronously.
	assert.Eventually(t, func() bool {
		return replacedCount.Load() >= 1
	}, time.Second, 5*time.Millisecond,
		"a replacement runner should be created after corruption")
}

func TestCallHook_NonCorruptionErrorReturnsRunner(t *testing.T) {
	var closedCount atomic.Int32
	callAttempts := atomic.Int32{}

	factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
		return &mockRunner{
			callFn: func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
				callAttempts.Add(1)
				return nil, errors.New("validation failed")
			},
		}, nil
	}

	rp := NewRuntimePool(testErrorLogger(), factory)

	p := testPluginWithPoolSize("non-corrupt", 1)
	require.NoError(t, rp.LoadPlugin(p))

	// First call — error but not corruption.
	_, err := rp.CallHook(context.Background(), "non-corrupt", "hook.test", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")

	// Second call — runner should still be available (was returned to pool).
	_, err = rp.CallHook(context.Background(), "non-corrupt", "hook.test", nil)
	require.Error(t, err)
	assert.Equal(t, int32(2), callAttempts.Load(), "runner should be reused")
	assert.Equal(t, int32(0), closedCount.Load(), "runner should not have been closed")
}

func TestIsRunnerCorrupted(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		corrupted bool
	}{
		{
			name:      "wasm unreachable",
			err:       fmt.Errorf("wasm: unreachable"),
			corrupted: true,
		},
		{
			name:      "out of fuel",
			err:       fmt.Errorf("out of fuel"),
			corrupted: true,
		},
		{
			name:      "memory.grow failure",
			err:       fmt.Errorf("memory.grow failed: limit exceeded"),
			corrupted: true,
		},
		{
			name:      "wasm trap",
			err:       fmt.Errorf("wasm trap: integer overflow"),
			corrupted: true,
		},
		{
			name:      "extism runtime error",
			err:       fmt.Errorf("extism: runtime error"),
			corrupted: true,
		},
		{
			name:      "call stack exhausted",
			err:       fmt.Errorf("call stack exhausted"),
			corrupted: true,
		},
		{
			name:      "memory limit (non-wasm)",
			err:       fmt.Errorf("memory limit exceeded"),
			corrupted: false,
		},
		{
			name:      "panic (non-wasm)",
			err:       fmt.Errorf("panic: runtime error"),
			corrupted: false,
		},
		{
			name:      "context deadline exceeded",
			err:       context.DeadlineExceeded,
			corrupted: false,
		},
		{
			name:      "context canceled",
			err:       context.Canceled,
			corrupted: false,
		},
		{
			name:      "wrapped context deadline",
			err:       fmt.Errorf("call failed: %w", context.DeadlineExceeded),
			corrupted: false,
		},
		{
			name:      "wrapped context canceled",
			err:       fmt.Errorf("call failed: %w", context.Canceled),
			corrupted: false,
		},
		{
			name:      "normal business error",
			err:       fmt.Errorf("validation failed"),
			corrupted: false,
		},
		{
			name:      "network error",
			err:       fmt.Errorf("connection refused"),
			corrupted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRunnerCorrupted(tt.err)
			assert.Equal(t, tt.corrupted, result)
		})
	}
}

func TestIsRunnerCorrupted_NoFalsePositives(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		corrupted bool
	}{
		// These should NOT be detected as WASM corruption.
		{
			name:      "Go memory allocation error",
			err:       errors.New("insufficient memory for allocation"),
			corrupted: false,
		},
		{
			name:      "Go runtime panic",
			err:       errors.New("panic: runtime error: index out of range"),
			corrupted: false,
		},
		{
			name:      "Go out of memory",
			err:       errors.New("out of memory"),
			corrupted: false,
		},
		{
			name:      "network error",
			err:       fmt.Errorf("connection refused"),
			corrupted: false,
		},
		{
			name:      "generic trap word in business error",
			err:       errors.New("trap occurred in request processing"),
			corrupted: false,
		},
		// These SHOULD be detected as WASM corruption.
		{
			name:      "wasm trap unreachable",
			err:       errors.New("wasm trap: unreachable"),
			corrupted: true,
		},
		{
			name:      "wazero memory.grow",
			err:       errors.New("wazero: memory.grow failed"),
			corrupted: true,
		},
		{
			name:      "extism plugin call",
			err:       errors.New("extism: plugin call failed"),
			corrupted: true,
		},
		{
			name:      "fuel exhaustion",
			err:       errors.New("out of fuel"),
			corrupted: true,
		},
		{
			name:      "call stack exhausted",
			err:       errors.New("call stack exhausted"),
			corrupted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.corrupted, isRunnerCorrupted(tt.err),
				"isRunnerCorrupted(%q) = %v, want %v", tt.err.Error(), isRunnerCorrupted(tt.err), tt.corrupted)
		})
	}
}

// --- Corruption-During-Drain Tests ---

func TestPool_DrainCompletesWhenCorruptedRunnerReleased(t *testing.T) {
	factory := mockFactory(nil)
	pool, err := newPluginInstancePool("drain-corrupt", factory, []byte("wasm"), nil, nil, 1)
	require.NoError(t, err)

	ctx := context.Background()

	// Acquire the single instance.
	runner, err := pool.Acquire(ctx)
	require.NoError(t, err)

	// Start drain in background — it should block because one runner is active.
	drainDone := make(chan error, 1)
	go func() { drainDone <- pool.Drain(context.Background()) }()

	// Verify drain is blocked.
	select {
	case <-drainDone:
		t.Fatal("drain completed before corrupted runner was handled")
	case <-time.After(50 * time.Millisecond):
		// Expected: drain is waiting.
	}

	// Simulate the corruption path: close runner, decrement active, signal drain.
	_ = runner.Close()
	remaining := atomic.AddInt32(&pool.active, -1)
	pool.signalDrainIfNeeded(remaining)

	// Drain should complete promptly.
	select {
	case <-drainDone:
		// Success.
	case <-time.After(time.Second):
		t.Fatal("drain should have completed after corrupted runner was handled")
	}
}

func TestPool_SignalDrainIfNeeded_NotDraining(t *testing.T) {
	factory := mockFactory(nil)
	pool, err := newPluginInstancePool("no-drain", factory, []byte("wasm"), nil, nil, 1)
	require.NoError(t, err)
	defer pool.Close()

	// Should not panic when pool is not draining.
	pool.signalDrainIfNeeded(0)
}

func TestPool_SignalDrainIfNeeded_AlreadySignaled(t *testing.T) {
	factory := mockFactory(nil)
	pool, err := newPluginInstancePool("double-signal", factory, []byte("wasm"), nil, nil, 1)
	require.NoError(t, err)

	// Drain with no active runners — drained channel already closed.
	_ = pool.Drain(context.Background())

	// Calling signalDrainIfNeeded again should not panic (double-close guard).
	pool.signalDrainIfNeeded(0)
}

func TestCallHook_DrainingReturnsCorrectError(t *testing.T) {
	factory := mockFactory(nil)
	rp := NewRuntimePool(testErrorLogger(), factory)

	p := testPluginWithPoolSize("draining-hook", 1)
	require.NoError(t, rp.LoadPlugin(p))

	// Drain the pool so it rejects new acquires.
	rp.mu.RLock()
	pool := rp.plugins["draining-hook"]
	rp.mu.RUnlock()
	_ = pool.Drain(context.Background())

	_, err := rp.CallHook(context.Background(), "draining-hook", "hook.test", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPluginDraining)
	// Must NOT be wrapped as ErrHookTimeout.
	assert.False(t, errors.Is(err, ErrHookTimeout), "draining error should not be wrapped as ErrHookTimeout")
}

func TestCallHook_CorruptionDuringDrainSignalsDrain(t *testing.T) {
	// Verify that when a runner returns a corruption error while the pool is
	// being drained, the drain completes (doesn't hang for 30s).
	corruptionErr := fmt.Errorf("wasm: unreachable instruction")

	factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
		return &mockRunner{
			callFn: func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
				return nil, corruptionErr
			},
		}, nil
	}

	rp := NewRuntimePool(testErrorLogger(), factory)

	p := testPluginWithPoolSize("corrupt-drain", 1)
	require.NoError(t, rp.LoadPlugin(p))

	// Acquire the runner manually so we can start drain while CallHook is
	// doing its work. We use SetRunnerForTest to install a fresh mock, then
	// trigger drain and CallHook in sequence.

	// Step 1: Get the pool, acquire the runner to block drain.
	rp.mu.RLock()
	pool := rp.plugins["corrupt-drain"]
	rp.mu.RUnlock()

	// We need the CallHook path to exercise corruption. The trick is:
	// use a runner whose Call blocks until we start drain, then returns corruption.
	callStarted := make(chan struct{})
	callProceed := make(chan struct{})

	blockingFactory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
		return &mockRunner{
			callFn: func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
				close(callStarted)
				<-callProceed
				return nil, corruptionErr
			},
		}, nil
	}

	// Replace the pool with one using our blocking factory.
	blockingPool, err := newPluginInstancePool("corrupt-drain", blockingFactory, []byte("wasm"), nil, nil, 1)
	require.NoError(t, err)
	rp.mu.Lock()
	rp.plugins["corrupt-drain"] = blockingPool
	pool = blockingPool
	rp.mu.Unlock()

	// Step 2: Start CallHook in background — it will acquire the runner and
	// block in Call until we signal callProceed.
	hookDone := make(chan error, 1)
	go func() {
		_, err := rp.CallHook(context.Background(), "corrupt-drain", "hook.test", nil)
		hookDone <- err
	}()

	// Wait for the runner's Call to start (runner is now acquired/active).
	<-callStarted

	// Step 3: Start drain — it should block because the runner is active.
	drainDone := make(chan error, 1)
	go func() { drainDone <- pool.Drain(context.Background()) }()

	// Give drain a moment to set draining=true.
	time.Sleep(20 * time.Millisecond)

	// Step 4: Let the Call return corruption error. CallHook should handle
	// corruption: close runner, decrement active, signal drain.
	close(callProceed)

	// Step 5: Both CallHook and Drain should complete promptly.
	select {
	case err := <-hookDone:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "corrupted")
	case <-time.After(2 * time.Second):
		t.Fatal("CallHook did not complete in time")
	}

	select {
	case <-drainDone:
		// Drain completed — corruption path correctly signaled drain.
	case <-time.After(2 * time.Second):
		t.Fatal("Drain should have completed after corruption handling, but it hung")
	}
}

// --- Max-Uses Counter Tests ---

func TestPooledRunner_ImplementsWASMRunner(t *testing.T) {
	// Verify that pooledRunner satisfies the WASMRunner interface at compile
	// time and delegates correctly.
	expected := []byte("output")
	inner := &mockRunner{
		callFn: func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return expected, nil
		},
	}

	var runner WASMRunner = &pooledRunner{runner: inner}

	output, err := runner.Call(context.Background(), "hook.test", nil)
	require.NoError(t, err)
	assert.Equal(t, expected, output)

	require.NoError(t, runner.Close())
	assert.True(t, inner.closed, "Close should delegate to inner runner")
}

func TestAcquire_IncrementsUseCount(t *testing.T) {
	factory := mockFactory(nil)
	pool, err := newPluginInstancePool("use-count", factory, []byte("wasm"), nil, nil, 1)
	require.NoError(t, err)
	defer pool.Close()

	ctx := context.Background()

	// Acquire-release cycle should increment use count.
	r1, err := pool.Acquire(ctx)
	require.NoError(t, err)

	pr, ok := r1.(*pooledRunner)
	require.True(t, ok, "Acquire should return a *pooledRunner")
	assert.Equal(t, int64(1), pr.useCount.Load())

	pool.Release(r1)

	// Second acquire of same runner — use count should be 2.
	r2, err := pool.Acquire(ctx)
	require.NoError(t, err)

	pr2, ok := r2.(*pooledRunner)
	require.True(t, ok)
	assert.Equal(t, int64(2), pr2.useCount.Load())

	pool.Release(r2)
}

func TestAcquire_ReplacesExhaustedRunner(t *testing.T) {
	var factoryCalls atomic.Int32
	factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
		factoryCalls.Add(1)
		return &mockRunner{}, nil
	}

	pool, err := newPluginInstancePool("exhaust", factory, []byte("wasm"), nil, nil, 1)
	require.NoError(t, err)
	defer pool.Close()

	ctx := context.Background()

	// Force the runner to the exhaustion threshold.
	r, err := pool.Acquire(ctx)
	require.NoError(t, err)

	pr := r.(*pooledRunner)
	// Set use count to MaxRunnerUses - 1 (Acquire already added 1).
	pr.useCount.Store(MaxRunnerUses - 1)
	pool.Release(r)

	// Initial pool creation used 1 factory call.
	initialCalls := factoryCalls.Load()

	// Next acquire: use count is MaxRunnerUses-1, Acquire increments to
	// MaxRunnerUses. This acquire gets the runner right at the threshold.
	r2, err := pool.Acquire(ctx)
	require.NoError(t, err)
	pool.Release(r2)

	// The release should detect exhaustion and trigger replacement.
	// Wait for the async replacement goroutine.
	assert.Eventually(t, func() bool {
		return factoryCalls.Load() > initialCalls
	}, time.Second, 5*time.Millisecond,
		"factory should be called to replace exhausted runner")

	// Next acquire should get a fresh runner with reset use count.
	r3, err := pool.Acquire(ctx)
	require.NoError(t, err)

	pr3 := r3.(*pooledRunner)
	assert.Equal(t, int64(1), pr3.useCount.Load(),
		"fresh runner should have use count 1 after acquire")
	pool.Release(r3)
}

func TestAcquire_ReplacesRunnerAtThreshold(t *testing.T) {
	// When a runner has exactly MaxRunnerUses, Acquire should replace it
	// inline (not return the exhausted runner).
	var factoryCalls atomic.Int32
	factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
		factoryCalls.Add(1)
		return &mockRunner{}, nil
	}

	pool, err := newPluginInstancePool("threshold", factory, []byte("wasm"), nil, nil, 1)
	require.NoError(t, err)
	defer pool.Close()

	ctx := context.Background()

	// Get the runner and set it to exactly MaxRunnerUses.
	r, err := pool.Acquire(ctx)
	require.NoError(t, err)
	pr := r.(*pooledRunner)
	pr.useCount.Store(MaxRunnerUses)
	pool.Release(r)

	// Release at MaxRunnerUses triggers async replacement via replaceInstance.
	// Wait for the replacement.
	assert.Eventually(t, func() bool {
		// 1 from pool init + 1 from replacement
		return factoryCalls.Load() >= 2
	}, time.Second, 5*time.Millisecond)

	initialCalls := factoryCalls.Load()

	// Next acquire should get the fresh runner from replaceInstance.
	// That runner will have useCount 0, and Acquire checks >= MaxRunnerUses
	// which is false, so it proceeds normally.
	r2, err := pool.Acquire(ctx)
	require.NoError(t, err)

	pr2 := r2.(*pooledRunner)
	assert.Equal(t, int64(1), pr2.useCount.Load(),
		"fresh runner should have use count 1")

	// No additional factory call — the replacement was already created.
	assert.Equal(t, initialCalls, factoryCalls.Load(),
		"no additional factory call expected")
	pool.Release(r2)
}

func TestRelease_ExhaustedRunnerClosedAndReplaced(t *testing.T) {
	var closedCount atomic.Int32
	var factoryCalls atomic.Int32

	factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
		factoryCalls.Add(1)
		return &trackingMockRunner{onClose: func() { closedCount.Add(1) }}, nil
	}

	pool, err := newPluginInstancePool("release-exhaust", factory, []byte("wasm"), nil, nil, 1)
	require.NoError(t, err)

	ctx := context.Background()
	r, err := pool.Acquire(ctx)
	require.NoError(t, err)

	pr := r.(*pooledRunner)
	pr.useCount.Store(MaxRunnerUses)

	// Release should close the exhausted runner and trigger async replacement.
	pool.Release(r)

	// The exhausted runner should be closed.
	assert.Equal(t, int32(1), closedCount.Load(),
		"exhausted runner should be closed on release")

	// Wait for the async replacement goroutine to finish before closing the pool.
	assert.Eventually(t, func() bool {
		// 1 from pool init + 1 replacement
		return factoryCalls.Load() >= 2
	}, time.Second, 5*time.Millisecond,
		"factory should be called to replace exhausted runner")

	// Close after replacement goroutine has completed to avoid data race.
	pool.Close()
}

func TestCallHook_RunnerReplacedAfterMaxUses(t *testing.T) {
	// End-to-end test: make MaxRunnerUses calls through CallHook and verify
	// the runner is replaced transparently.
	var factoryCalls atomic.Int32
	factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
		factoryCalls.Add(1)
		return &mockRunner{
			callFn: func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
				return []byte(`{"action":"continue"}`), nil
			},
		}, nil
	}

	rp := NewRuntimePool(testErrorLogger(), factory)

	p := testPluginWithPoolSize("max-uses-e2e", 1)
	require.NoError(t, rp.LoadPlugin(p))

	ctx := context.Background()

	// Directly set the runner's use count near the threshold to avoid
	// making 10000 real calls.
	rp.mu.RLock()
	pool := rp.plugins["max-uses-e2e"]
	rp.mu.RUnlock()

	r, err := pool.Acquire(ctx)
	require.NoError(t, err)
	pr := r.(*pooledRunner)
	pr.useCount.Store(MaxRunnerUses - 2) // Next acquire will set it to MaxRunnerUses - 1
	pool.Release(r)

	initialFactoryCalls := factoryCalls.Load()

	// Make two calls to cross the threshold.
	for range 2 {
		output, err := rp.CallHook(ctx, "max-uses-e2e", "hook.test", nil)
		require.NoError(t, err)
		assert.Equal(t, []byte(`{"action":"continue"}`), output)
	}

	// The runner should have been replaced by now (either inline in Acquire
	// or via Release triggering replaceInstance).
	assert.Eventually(t, func() bool {
		return factoryCalls.Load() > initialFactoryCalls
	}, time.Second, 5*time.Millisecond,
		"factory should be called to replace exhausted runner during CallHook")
}

func TestRelease_RawRunnerWrappedAsPooledRunner(t *testing.T) {
	// When Release receives a raw WASMRunner (not *pooledRunner), it should
	// wrap it and return it to the pool without panicking.
	factory := mockFactory(nil)
	pool, err := newPluginInstancePool("raw-wrap", factory, []byte("wasm"), nil, nil, 2)
	require.NoError(t, err)
	defer pool.Close()

	// Drain one runner to make room, then release a raw mockRunner.
	ctx := context.Background()
	r, err := pool.Acquire(ctx)
	require.NoError(t, err)

	// Release a raw runner (simulating the SetRunnerForTest path).
	raw := &mockRunner{}
	// Manually increment active to match Release's decrement.
	atomic.AddInt32(&pool.active, 1)
	pool.Release(raw)

	pool.Release(r)
}
