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

func TestPluginInstancePool_FactoryError_CleansUp(t *testing.T) {
	var closedCount atomic.Int32
	callCount := 0

	factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
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
			name:      "memory limit exceeded",
			err:       fmt.Errorf("memory limit exceeded"),
			corrupted: true,
		},
		{
			name:      "wasm trap",
			err:       fmt.Errorf("wasm trap: integer overflow"),
			corrupted: true,
		},
		{
			name:      "panic in wasm",
			err:       fmt.Errorf("panic: runtime error"),
			corrupted: true,
		},
		{
			name:      "trap instruction",
			err:       fmt.Errorf("trap: call stack exhausted"),
			corrupted: true,
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
