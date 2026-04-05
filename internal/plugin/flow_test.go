package plugin

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptureFlowBindings_ReturnsCurrentVersions(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), mockFactory(nil))

	p1 := testPlugin("plugin-a")
	p2 := testPlugin("plugin-b")
	require.NoError(t, rp.LoadPlugin(p1))
	require.NoError(t, rp.LoadPlugin(p2))

	bindings := rp.CaptureFlowBindings()

	require.Len(t, bindings, 2)
	assert.NotZero(t, bindings["plugin-a"], "plugin-a should have a non-zero version")
	assert.NotZero(t, bindings["plugin-b"], "plugin-b should have a non-zero version")
}

func TestCaptureFlowBindings_Empty(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), mockFactory(nil))

	bindings := rp.CaptureFlowBindings()

	assert.Empty(t, bindings)
}

func TestCaptureFlowBindings_VersionIncrementsOnReload(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), mockFactory(nil))

	p := testPlugin("plugin-a")
	require.NoError(t, rp.LoadPlugin(p))

	bindingsV1 := rp.CaptureFlowBindings()
	versionV1 := bindingsV1["plugin-a"]

	// Reload the same plugin — version should increment.
	require.NoError(t, rp.LoadPlugin(p))

	bindingsV2 := rp.CaptureFlowBindings()
	versionV2 := bindingsV2["plugin-a"]

	assert.Greater(t, versionV2, versionV1,
		"reloaded plugin should have a higher version")
}

func TestWithFlowBindings_RoundTrip(t *testing.T) {
	bindings := FlowBindings{"plugin-a": 42, "plugin-b": 7}
	ctx := withFlowBindings(context.Background(), bindings)

	got := flowBindingsFromContext(ctx)
	require.NotNil(t, got)
	assert.Equal(t, uint64(42), got["plugin-a"])
	assert.Equal(t, uint64(7), got["plugin-b"])
}

func TestFlowBindingsFromContext_NilWhenNotSet(t *testing.T) {
	got := flowBindingsFromContext(context.Background())
	assert.Nil(t, got)
}

func TestCallHook_WithFlowBindings_UsesPinnedPool(t *testing.T) {
	// Set up a runtime pool with two versions of the same plugin. The flow
	// bindings pin to v1. After a hot reload to v2, CallHook should still
	// route to the retired v1 pool.

	var callVersion atomic.Int64

	v1Factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
		return &mockRunner{
			callFn: func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
				callVersion.Store(1)
				return []byte("v1-output"), nil
			},
		}, nil
	}

	rp := NewRuntimePool(testErrorLogger(), v1Factory)

	p := testPlugin("versioned-plugin")
	require.NoError(t, rp.LoadPlugin(p))

	// Capture flow bindings while v1 is active.
	bindings := rp.CaptureFlowBindings()
	ctx := withFlowBindings(context.Background(), bindings)

	// Hot reload the plugin with a new factory that marks calls as v2.
	v2Factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
		return &mockRunner{
			callFn: func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
				callVersion.Store(2)
				return []byte("v2-output"), nil
			},
		}, nil
	}
	rp.mu.Lock()
	rp.runnerFactory = v2Factory
	rp.mu.Unlock()

	require.NoError(t, rp.LoadPlugin(p))

	// Call with the flow-pinned context — should use v1 (retired pool).
	output, err := rp.CallHook(ctx, "versioned-plugin", "hook.test", nil)
	require.NoError(t, err)
	assert.Equal(t, []byte("v1-output"), output)
	assert.Equal(t, int64(1), callVersion.Load(),
		"flow-pinned call should have used v1 pool")

	// Call WITHOUT flow bindings — should use v2 (current pool).
	output, err = rp.CallHook(context.Background(), "versioned-plugin", "hook.test", nil)
	require.NoError(t, err)
	assert.Equal(t, []byte("v2-output"), output)
	assert.Equal(t, int64(2), callVersion.Load(),
		"non-pinned call should have used v2 pool")
}

func TestCallHook_WithFlowBindings_FallsThroughWhenExpired(t *testing.T) {
	// When a flow-pinned version has been fully drained and removed from
	// retiredPools, CallHook should fall through to the current pool.

	rp := NewRuntimePool(testErrorLogger(), mockFactory(func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
		return []byte("current-output"), nil
	}))

	p := testPlugin("expire-plugin")
	require.NoError(t, rp.LoadPlugin(p))

	// Capture bindings referencing the v1 pool.
	bindings := rp.CaptureFlowBindings()

	// Reload to create v2.
	require.NoError(t, rp.LoadPlugin(p))

	// The retired pool should still be present during the grace period.
	assert.Equal(t, 1, rp.RetiredPoolCount(),
		"retired pool should exist during grace period")

	// Manually remove the retired pool to simulate expiry without waiting
	// for the full RetiredPoolGracePeriod.
	rp.mu.Lock()
	for key, pool := range rp.retiredPools {
		_ = pool.Drain(context.Background())
		delete(rp.retiredPools, key)
	}
	rp.mu.Unlock()

	// Call with stale flow bindings — should fall through to current pool.
	ctx := withFlowBindings(context.Background(), bindings)
	output, err := rp.CallHook(ctx, "expire-plugin", "hook.test", nil)
	require.NoError(t, err)
	assert.Equal(t, []byte("current-output"), output)
}

func TestCallHook_WithFlowBindings_MatchesCurrentPool(t *testing.T) {
	// When the current pool version matches the pinned version (no reload
	// happened), it should use the current pool directly — the common case.

	rp := NewRuntimePool(testErrorLogger(), mockFactory(func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
		return []byte("same-version"), nil
	}))

	p := testPlugin("same-version-plugin")
	require.NoError(t, rp.LoadPlugin(p))

	bindings := rp.CaptureFlowBindings()
	ctx := withFlowBindings(context.Background(), bindings)

	output, err := rp.CallHook(ctx, "same-version-plugin", "hook.test", nil)
	require.NoError(t, err)
	assert.Equal(t, []byte("same-version"), output)
}

func TestCallHook_WithoutFlowBindings_BehavesAsDefault(t *testing.T) {
	// Without flow bindings, CallHook should behave exactly as before.
	expected := []byte(`{"action":"continue"}`)
	factory := mockFactory(func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
		return expected, nil
	})

	rp := NewRuntimePool(testErrorLogger(), factory)

	p := testPlugin("default-plugin")
	require.NoError(t, rp.LoadPlugin(p))

	output, err := rp.CallHook(context.Background(), "default-plugin", "hook.test", []byte("input"))
	require.NoError(t, err)
	assert.Equal(t, expected, output)
}

func TestRetiredPools_KeptDuringGracePeriod(t *testing.T) {
	var closedCount atomic.Int32

	factory := func(_ string, wasmBytes []byte, config map[string]string, _ ManifestLimits) (WASMRunner, error) {
		return &trackingMockRunner{onClose: func() { closedCount.Add(1) }}, nil
	}

	rp := NewRuntimePool(testErrorLogger(), factory)

	p := testPlugin("retire-test")
	require.NoError(t, rp.LoadPlugin(p))

	// Reload — the old pool becomes a retired pool.
	require.NoError(t, rp.LoadPlugin(p))

	// The retired pool should still be present during the grace period
	// (RetiredPoolGracePeriod = 30s), allowing flow-pinned callers to finish.
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, rp.RetiredPoolCount(),
		"retired pool should still exist during grace period")

	// No runners from the retired pool should have been closed yet because
	// the grace period timer has not fired.
	assert.Equal(t, int32(0), closedCount.Load(),
		"no runners should be closed during grace period")
}

func TestPoolVersion_MonotonicallyIncreasing(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), mockFactory(nil))

	var versions []uint64

	for i := range 5 {
		p := testPlugin(fmt.Sprintf("plugin-%d", i))
		require.NoError(t, rp.LoadPlugin(p))

		rp.mu.RLock()
		pool := rp.plugins[p.Slug]
		rp.mu.RUnlock()

		if pool != nil {
			versions = append(versions, pool.version)
		}
	}

	// Each version should be strictly greater than the previous.
	for i := 1; i < len(versions); i++ {
		assert.Greater(t, versions[i], versions[i-1],
			"version %d should be greater than version %d", i, i-1)
	}
}

func TestCallHook_FlowBindings_UnknownSlugIgnored(t *testing.T) {
	// Flow bindings for a slug not present in the pool should not cause errors.
	rp := NewRuntimePool(testErrorLogger(), mockFactory(func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
		return []byte("ok"), nil
	}))

	p := testPlugin("known-plugin")
	require.NoError(t, rp.LoadPlugin(p))

	// Bindings reference a different slug that doesn't exist.
	bindings := FlowBindings{"unknown-plugin": 999}
	ctx := withFlowBindings(context.Background(), bindings)

	// Calling a known plugin with bindings for an unknown one should work fine.
	output, err := rp.CallHook(ctx, "known-plugin", "hook.test", nil)
	require.NoError(t, err)
	assert.Equal(t, []byte("ok"), output)
}
