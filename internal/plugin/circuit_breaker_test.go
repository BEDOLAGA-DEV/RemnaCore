package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
)

// --- hookBreaker unit tests ---

func TestHookBreaker_AllowWhenClosed(t *testing.T) {
	b := &hookBreaker{}
	assert.True(t, b.allow(), "closed breaker should allow calls")
}

func TestHookBreaker_OpensAfterThreshold(t *testing.T) {
	b := &hookBreaker{}

	for i := int64(0); i < HookBreakerFailureThreshold; i++ {
		b.recordFailure()
	}

	assert.True(t, b.isOpen(), "breaker should be open after reaching failure threshold")
	assert.False(t, b.allow(), "open breaker should reject calls")
}

func TestHookBreaker_DoesNotOpenBelowThreshold(t *testing.T) {
	b := &hookBreaker{}

	for i := int64(0); i < HookBreakerFailureThreshold-1; i++ {
		b.recordFailure()
	}

	assert.False(t, b.isOpen(), "breaker should remain closed below threshold")
	assert.True(t, b.allow(), "breaker below threshold should allow calls")
}

func TestHookBreaker_SuccessResetsFailureCount(t *testing.T) {
	b := &hookBreaker{}

	// Record failures up to threshold-1.
	for i := int64(0); i < HookBreakerFailureThreshold-1; i++ {
		b.recordFailure()
	}

	// A success resets the counter.
	b.recordSuccess()

	// One more failure should not open the breaker.
	b.recordFailure()
	assert.False(t, b.isOpen(), "breaker should not open after success reset")
}

func TestHookBreaker_TransitionsToHalfOpenAfterTimeout(t *testing.T) {
	b := &hookBreaker{}

	// Open the breaker.
	for i := int64(0); i < HookBreakerFailureThreshold; i++ {
		b.recordFailure()
	}
	require.True(t, b.isOpen())

	// Simulate time passing beyond the reset timeout by backdating lastFailure.
	pastTime := time.Now().Add(-HookBreakerResetTimeout - time.Second)
	b.lastFailure.Store(pastTime.UnixNano())

	// Next allow() call should transition to half-open and permit a test call.
	assert.True(t, b.allow(), "breaker should transition to half-open after timeout")
	assert.Equal(t, int32(hookBreakerHalfOpen), b.state.Load())
}

func TestHookBreaker_HalfOpen_SuccessCloses(t *testing.T) {
	b := &hookBreaker{}
	b.state.Store(int32(hookBreakerHalfOpen))

	b.recordSuccess()

	assert.Equal(t, int32(hookBreakerClosed), b.state.Load(),
		"successful test call should close the breaker")
	assert.True(t, b.allow())
}

func TestHookBreaker_HalfOpen_FailureReopens(t *testing.T) {
	b := &hookBreaker{}
	b.state.Store(int32(hookBreakerHalfOpen))

	b.recordFailure()

	assert.Equal(t, int32(hookBreakerOpen), b.state.Load(),
		"failed test call should reopen the breaker")
}

func TestHookBreaker_HalfOpen_LimitsTestCalls(t *testing.T) {
	b := &hookBreaker{}
	b.state.Store(int32(hookBreakerHalfOpen))
	b.halfOpen.Store(0)

	// First call allowed.
	assert.True(t, b.allow(), "first half-open call should be allowed")

	// Second call rejected (max is 1).
	assert.False(t, b.allow(), "second half-open call should be rejected")
}

// --- hookCircuitBreakers registry tests ---

func TestHookCircuitBreakers_GetCreatesLazily(t *testing.T) {
	cb := newHookCircuitBreakers()

	b1 := cb.get("plugin-a:checkout.process")
	require.NotNil(t, b1)

	// Same key returns same breaker.
	b2 := cb.get("plugin-a:checkout.process")
	assert.Same(t, b1, b2)

	// Different key returns different breaker.
	b3 := cb.get("plugin-b:checkout.process")
	assert.NotSame(t, b1, b3)
}

func TestHookCircuitBreakers_Remove(t *testing.T) {
	cb := newHookCircuitBreakers()

	b1 := cb.get("plugin-a:hook1")
	require.NotNil(t, b1)

	cb.remove("plugin-a:hook1")

	// A new get should create a fresh breaker.
	b2 := cb.get("plugin-a:hook1")
	assert.NotSame(t, b1, b2)
}

func TestHookCircuitBreakers_RemoveByPlugin(t *testing.T) {
	cb := newHookCircuitBreakers()

	_ = cb.get("plugin-a:hook1")
	_ = cb.get("plugin-a:hook2")
	_ = cb.get("plugin-b:hook1")

	cb.removeByPlugin("plugin-a")

	cb.mu.RLock()
	defer cb.mu.RUnlock()
	assert.Len(t, cb.breakers, 1, "only plugin-b breaker should remain")
	_, exists := cb.breakers["plugin-b:hook1"]
	assert.True(t, exists)
}

// --- Integration tests: circuit breaker in DispatchSync ---

func TestDispatchSync_CircuitBreakerOpens_ReturnsError(t *testing.T) {
	var callCount atomic.Int64

	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"failing-plugin": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			callCount.Add(1)
			return nil, fmt.Errorf("plugin crashed")
		},
	})

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-fail", PluginSlug: "failing-plugin", HookName: "checkout.process", HookType: HookSync, Priority: 10, FuncName: "checkout.process"},
	})

	payload := json.RawMessage(`{"order_id":"o-1"}`)

	// Exhaust the breaker threshold with failures.
	for i := int64(0); i < HookBreakerFailureThreshold; i++ {
		_, err := d.DispatchSync(context.Background(), "checkout.process", payload)
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrCircuitBreakerOpen, "should return plugin error, not breaker error, until threshold")
	}

	// The breaker is now open. The next call should fail with ErrCircuitBreakerOpen
	// without actually calling the plugin.
	countBefore := callCount.Load()
	_, err := d.DispatchSync(context.Background(), "checkout.process", payload)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCircuitBreakerOpen)
	assert.Equal(t, countBefore, callCount.Load(), "plugin should not be called when breaker is open")
}

func TestDispatchSyncSafe_CircuitBreakerOpen_SkipsPlugin(t *testing.T) {
	var callCount atomic.Int64

	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"failing-plugin": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			callCount.Add(1)
			return nil, fmt.Errorf("plugin crashed")
		},
		"healthy-plugin": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return hookResultBytes(sdk.ActionModify, json.RawMessage(`{"modified":true}`), ""), nil
		},
	})

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-fail", PluginSlug: "failing-plugin", HookName: "checkout.process", HookType: HookSync, Priority: 10, FuncName: "checkout.process"},
		{PluginID: "id-ok", PluginSlug: "healthy-plugin", HookName: "checkout.process", HookType: HookSync, Priority: 20, FuncName: "checkout.process"},
	})

	payload := json.RawMessage(`{"order_id":"o-1"}`)

	// Open the breaker for failing-plugin via DispatchSync (which surfaces errors).
	for i := int64(0); i < HookBreakerFailureThreshold; i++ {
		_, _ = d.DispatchSync(context.Background(), "checkout.process", payload)
	}

	// Now use DispatchSyncSafe: failing-plugin should be skipped, healthy-plugin runs.
	countBefore := callCount.Load()
	result := d.DispatchSyncSafe(context.Background(), "checkout.process", payload)

	require.NoError(t, result.Err)
	assert.False(t, result.Compensated)
	assert.Equal(t, countBefore, callCount.Load(), "failing-plugin should be skipped")
	assert.Equal(t, []string{"healthy-plugin"}, result.ExecutedPlugins)
	assert.JSONEq(t, `{"modified":true}`, string(result.Payload))
}

func TestDispatchSync_CircuitBreakerRecovers(t *testing.T) {
	var shouldFail atomic.Bool
	shouldFail.Store(true)

	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"flaky-plugin": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			if shouldFail.Load() {
				return nil, fmt.Errorf("transient error")
			}
			return hookResultBytes(sdk.ActionContinue, nil, ""), nil
		},
	})

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-flaky", PluginSlug: "flaky-plugin", HookName: "order.create", HookType: HookSync, Priority: 10, FuncName: "order.create"},
	})

	payload := json.RawMessage(`{"amount":100}`)

	// Open the breaker.
	for i := int64(0); i < HookBreakerFailureThreshold; i++ {
		_, _ = d.DispatchSync(context.Background(), "order.create", payload)
	}

	// Verify breaker is open.
	_, err := d.DispatchSync(context.Background(), "order.create", payload)
	require.ErrorIs(t, err, ErrCircuitBreakerOpen)

	// Simulate reset timeout by backdating the lastFailure timestamp.
	breakerKey := "flaky-plugin" + breakerKeySeparator + "order.create"
	breaker := d.circuitBreakers.get(breakerKey)
	pastTime := time.Now().Add(-HookBreakerResetTimeout - time.Second)
	breaker.lastFailure.Store(pastTime.UnixNano())

	// Now the plugin is healthy.
	shouldFail.Store(false)

	// The half-open test call should succeed and close the breaker.
	result, err := d.DispatchSync(context.Background(), "order.create", payload)
	require.NoError(t, err)
	assert.JSONEq(t, `{"amount":100}`, string(result))

	// Subsequent calls should work normally.
	result, err = d.DispatchSync(context.Background(), "order.create", payload)
	require.NoError(t, err)
	assert.JSONEq(t, `{"amount":100}`, string(result))
}

func TestUnregisterHooks_ClearsCircuitBreakers(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), nil)
	d := NewHookDispatcher(rp, &testPublisher{}, nil, testErrorLogger(), clock.NewReal())

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "hook1", HookType: HookSync, Priority: 10, FuncName: "hook1"},
	})

	// Manually create a breaker entry.
	breaker := d.circuitBreakers.get("plugin-a" + breakerKeySeparator + "hook1")
	for i := int64(0); i < HookBreakerFailureThreshold; i++ {
		breaker.recordFailure()
	}
	require.True(t, breaker.isOpen())

	d.UnregisterHooks("plugin-a")

	// After unregister, a new get should return a fresh (closed) breaker.
	freshBreaker := d.circuitBreakers.get("plugin-a" + breakerKeySeparator + "hook1")
	assert.False(t, freshBreaker.isOpen(), "breaker should be reset after unregister")
}

func TestSwapHooks_ResetsCircuitBreakers(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), nil)
	d := NewHookDispatcher(rp, &testPublisher{}, nil, testErrorLogger(), clock.NewReal())

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "hook1", HookType: HookSync, Priority: 10, FuncName: "hook1"},
	})

	// Open the breaker.
	breaker := d.circuitBreakers.get("plugin-a" + breakerKeySeparator + "hook1")
	for i := int64(0); i < HookBreakerFailureThreshold; i++ {
		breaker.recordFailure()
	}
	require.True(t, breaker.isOpen())

	// Swap hooks (simulating hot reload).
	d.SwapHooks("plugin-a", []HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "hook1", HookType: HookSync, Priority: 10, FuncName: "hook1"},
	})

	// Breaker should be cleared.
	freshBreaker := d.circuitBreakers.get("plugin-a" + breakerKeySeparator + "hook1")
	assert.False(t, freshBreaker.isOpen(), "breaker should be reset after swap")
}
