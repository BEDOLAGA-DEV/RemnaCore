package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
)

// dispatcherWithMock creates a HookDispatcher backed by a mock runtime pool
// where each slug maps to a specific WASMRunner call function.
func dispatcherWithMock(t *testing.T, slugCallFns map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error)) (*HookDispatcher, *testPublisher) {
	t.Helper()

	logger := testErrorLogger()
	pub := &testPublisher{}

	rp := NewRuntimePool(logger, nil)
	for slug, callFn := range slugCallFns {
		p := testPlugin(slug)
		require.NoError(t, rp.LoadPlugin(p))

		// Inject a mock runner via the test helper.
		rp.SetRunnerForTest(slug, &mockRunner{callFn: callFn})
	}

	d := NewHookDispatcher(rp, pub, nil, logger, clock.NewReal())
	return d, pub
}

func hookResultBytes(action sdk.HookAction, modified json.RawMessage, errMsg string) []byte {
	r := sdk.HookResult{
		Action:   action,
		Modified: modified,
		Error:    errMsg,
	}
	b, _ := json.Marshal(r)
	return b
}

func TestDispatchSync_SinglePlugin_Continue(t *testing.T) {
	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"plugin-a": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return hookResultBytes(sdk.ActionContinue, nil, ""), nil
		},
	})

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "invoice.created", HookType: HookSync, Priority: 10, FuncName: "invoice.created"},
	})

	payload := json.RawMessage(`{"amount":100}`)
	result, err := d.DispatchSync(context.Background(), "invoice.created", payload)
	require.NoError(t, err)

	// Payload should be unchanged.
	assert.JSONEq(t, `{"amount":100}`, string(result))
}

func TestDispatchSync_SinglePlugin_Modify(t *testing.T) {
	modifiedPayload := json.RawMessage(`{"amount":200}`)

	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"plugin-a": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return hookResultBytes(sdk.ActionModify, modifiedPayload, ""), nil
		},
	})

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "invoice.created", HookType: HookSync, Priority: 10, FuncName: "invoice.created"},
	})

	payload := json.RawMessage(`{"amount":100}`)
	result, err := d.DispatchSync(context.Background(), "invoice.created", payload)
	require.NoError(t, err)

	assert.JSONEq(t, `{"amount":200}`, string(result))
}

func TestDispatchSync_SinglePlugin_Halt(t *testing.T) {
	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"plugin-a": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return hookResultBytes(sdk.ActionHalt, nil, "payment blocked"), nil
		},
	})

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "invoice.created", HookType: HookSync, Priority: 10, FuncName: "invoice.created"},
	})

	payload := json.RawMessage(`{"amount":100}`)
	_, err := d.DispatchSync(context.Background(), "invoice.created", payload)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrHookHalted)
	assert.Contains(t, err.Error(), "payment blocked")
}

func TestDispatchSync_TwoPlugins_PriorityOrdering(t *testing.T) {
	// Track call order.
	var callOrder []string

	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"plugin-a": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			callOrder = append(callOrder, "plugin-a")
			// Modify: set amount to 200
			return hookResultBytes(sdk.ActionModify, json.RawMessage(`{"amount":200}`), ""), nil
		},
		"plugin-b": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			callOrder = append(callOrder, "plugin-b")
			// Verify plugin-b receives the modified payload from plugin-a.
			var hookCtx sdk.HookContext
			_ = json.Unmarshal(input, &hookCtx)
			assert.JSONEq(t, `{"amount":200}`, string(hookCtx.Payload))

			// Further modify: set amount to 300
			return hookResultBytes(sdk.ActionModify, json.RawMessage(`{"amount":300}`), ""), nil
		},
	})

	// plugin-a has higher priority (lower number = runs first).
	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-b", PluginSlug: "plugin-b", HookName: "invoice.created", HookType: HookSync, Priority: 20, FuncName: "invoice.created"},
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "invoice.created", HookType: HookSync, Priority: 10, FuncName: "invoice.created"},
	})

	payload := json.RawMessage(`{"amount":100}`)
	result, err := d.DispatchSync(context.Background(), "invoice.created", payload)
	require.NoError(t, err)

	// plugin-a should run first (priority 10), then plugin-b (priority 20).
	assert.Equal(t, []string{"plugin-a", "plugin-b"}, callOrder)
	assert.JSONEq(t, `{"amount":300}`, string(result))
}

func TestDispatchSync_NoHandlers(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), nil)
	d := NewHookDispatcher(rp, &testPublisher{}, nil, testErrorLogger(), clock.NewReal())

	payload := json.RawMessage(`{"amount":100}`)
	result, err := d.DispatchSync(context.Background(), "nonexistent.hook", payload)
	require.NoError(t, err)

	// Should pass through unchanged.
	assert.JSONEq(t, `{"amount":100}`, string(result))
}

func TestRegisterHooks_And_UnregisterHooks(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), nil)
	d := NewHookDispatcher(rp, &testPublisher{}, nil, testErrorLogger(), clock.NewReal())

	regs := []HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "invoice.created", HookType: HookSync, Priority: 10, FuncName: "invoice.created"},
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "payment.completed", HookType: HookSync, Priority: 20, FuncName: "payment.completed"},
	}

	d.RegisterHooks(regs)
	assert.Len(t, d.Registrations("invoice.created"), 1)
	assert.Len(t, d.Registrations("payment.completed"), 1)

	d.UnregisterHooks("plugin-a")
	assert.Empty(t, d.Registrations("invoice.created"))
	assert.Empty(t, d.Registrations("payment.completed"))
}

func TestDispatchAsync_PublishesEvent(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), nil)
	pub := &testPublisher{}
	d := NewHookDispatcher(rp, pub, nil, testErrorLogger(), clock.NewReal())

	payload := json.RawMessage(`{"user_id":"u-1"}`)
	err := d.DispatchAsync(context.Background(), "subscription.renewed", payload)
	require.NoError(t, err)

	require.Len(t, pub.events, 1)
	assert.Equal(t, domainevent.EventType("plugin.hook.subscription.renewed"), pub.events[0].Type)
	assert.Equal(t, "subscription.renewed", pub.events[0].DataAsMap()["hook_name"])
}

func TestDispatchAsync_NilPublisher(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), nil)
	d := NewHookDispatcher(rp, nil, nil, testErrorLogger(), clock.NewReal())

	err := d.DispatchAsync(context.Background(), "hook.name", json.RawMessage(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "event publisher not configured")
}

func TestDispatchSync_SkipsAsyncRegistrations(t *testing.T) {
	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"plugin-a": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			t.Fatal("async registration should not be called in sync dispatch")
			return nil, nil
		},
	})

	// Register as async — should be skipped by DispatchSync.
	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "invoice.created", HookType: HookAsync, Priority: 10, FuncName: "invoice.created"},
	})

	payload := json.RawMessage(`{"amount":100}`)
	result, err := d.DispatchSync(context.Background(), "invoice.created", payload)
	require.NoError(t, err)
	assert.JSONEq(t, `{"amount":100}`, string(result))
}

func TestDispatchSync_Timeout(t *testing.T) {
	// Create a mock runner that blocks until context is cancelled, simulating
	// a stuck plugin that exceeds its timeout.
	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"slow-plugin": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			// Block until the context deadline fires.
			<-ctx.Done()
			return nil, ctx.Err()
		},
	})

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-slow", PluginSlug: "slow-plugin", HookName: "invoice.created", HookType: HookSync, Priority: 10, FuncName: "invoice.created"},
	})

	payload := json.RawMessage(`{"amount":100}`)
	_, err := d.DispatchSync(context.Background(), "invoice.created", payload)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrHookTimeout)
	assert.Contains(t, err.Error(), "slow-plugin")
	assert.Contains(t, err.Error(), "timed out")
}

func TestDispatchSync_Timeout_CustomManifest(t *testing.T) {
	// Create a plugin with a very short custom timeout.
	logger := testErrorLogger()
	pub := &testPublisher{}

	rp := NewRuntimePool(logger, nil)

	// Build a plugin whose manifest declares a 50ms sync timeout.
	m := &Manifest{
		Plugin: ManifestPlugin{ID: "fast-timeout", Name: "FastTimeout", Version: "1.0.0", SDKVersion: CurrentSDKVersion},
		Hooks:  ManifestHooks{Sync: []string{"hook.test"}},
		Limits: ManifestLimits{TimeoutSyncMs: 50},
	}
	p, err := NewPlugin(m, []byte("fake-wasm"), time.Now())
	require.NoError(t, err)
	require.NoError(t, rp.LoadPlugin(p))

	// Set a runner that blocks longer than the 50ms timeout.
	rp.SetRunnerForTest("fast-timeout", &mockRunner{
		callFn: func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	})

	d := NewHookDispatcher(rp, pub, nil, logger, clock.NewReal())
	d.RegisterHooks([]HookRegistration{
		{PluginID: p.ID, PluginSlug: "fast-timeout", HookName: "hook.test", HookType: HookSync, Priority: 10, FuncName: "hook.test"},
	})

	payload := json.RawMessage(`{"key":"value"}`)
	_, err = d.DispatchSync(context.Background(), "hook.test", payload)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrHookTimeout)
}

func TestUnregisterHooks_DoesNotAffectOtherPlugins(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), nil)
	d := NewHookDispatcher(rp, &testPublisher{}, nil, testErrorLogger(), clock.NewReal())

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "invoice.created", HookType: HookSync, Priority: 10, FuncName: "invoice.created"},
		{PluginID: "id-b", PluginSlug: "plugin-b", HookName: "invoice.created", HookType: HookSync, Priority: 20, FuncName: "invoice.created"},
	})

	d.UnregisterHooks("plugin-a")

	regs := d.Registrations("invoice.created")
	require.Len(t, regs, 1)
	assert.Equal(t, "plugin-b", regs[0].PluginSlug)
}

// --- DispatchSyncVersioned Tests ---

func TestDispatchSyncVersioned_FallbackToV1(t *testing.T) {
	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"plugin-a": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return hookResultBytes(sdk.ActionModify, json.RawMessage(`{"version":"v1"}`), ""), nil
		},
	})

	// Register handler for the unversioned (v1) hook only.
	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "payment.create_charge", HookType: HookSync, Priority: 10, FuncName: "payment.create_charge"},
	})

	payload := json.RawMessage(`{"amount":100}`)
	result, err := d.DispatchSyncVersioned(context.Background(), "payment.create_charge", 2, payload)
	require.NoError(t, err)

	// Should fall back to v1 handler since no v2 handler is registered.
	assert.JSONEq(t, `{"version":"v1"}`, string(result))
}

func TestDispatchSyncVersioned_UsesV2(t *testing.T) {
	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"plugin-a": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return hookResultBytes(sdk.ActionModify, json.RawMessage(`{"version":"v2"}`), ""), nil
		},
	})

	// Register handler for the v2 hook.
	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "payment.create_charge.v2", HookType: HookSync, Priority: 10, FuncName: "payment.create_charge.v2"},
	})

	payload := json.RawMessage(`{"amount":100}`)
	result, err := d.DispatchSyncVersioned(context.Background(), "payment.create_charge", 2, payload)
	require.NoError(t, err)

	// Should use v2 handler.
	assert.JSONEq(t, `{"version":"v2"}`, string(result))
}

func TestDispatchSyncVersioned_UsesHighestAvailable(t *testing.T) {
	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"plugin-v2": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return hookResultBytes(sdk.ActionModify, json.RawMessage(`{"version":"v2"}`), ""), nil
		},
		"plugin-v1": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return hookResultBytes(sdk.ActionModify, json.RawMessage(`{"version":"v1"}`), ""), nil
		},
	})

	// Register v1 and v2 handlers.
	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-v1", PluginSlug: "plugin-v1", HookName: "payment.create_charge", HookType: HookSync, Priority: 10, FuncName: "payment.create_charge"},
		{PluginID: "id-v2", PluginSlug: "plugin-v2", HookName: "payment.create_charge.v2", HookType: HookSync, Priority: 10, FuncName: "payment.create_charge.v2"},
	})

	payload := json.RawMessage(`{"amount":100}`)

	// Dispatch with currentVersion=3 — should try v3 (not found), then v2 (found).
	result, err := d.DispatchSyncVersioned(context.Background(), "payment.create_charge", 3, payload)
	require.NoError(t, err)
	assert.JSONEq(t, `{"version":"v2"}`, string(result))
}

func TestDispatchSyncVersioned_NoHandlersPassesThrough(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), nil)
	d := NewHookDispatcher(rp, &testPublisher{}, nil, testErrorLogger(), clock.NewReal())

	// No handlers registered at all.
	payload := json.RawMessage(`{"amount":100}`)
	result, err := d.DispatchSyncVersioned(context.Background(), "nonexistent.hook", 3, payload)
	require.NoError(t, err)

	// Should pass through unchanged (no handlers for any version).
	assert.JSONEq(t, `{"amount":100}`, string(result))
}

func TestDispatchSyncVersioned_Version1DispatchesToBase(t *testing.T) {
	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"plugin-a": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return hookResultBytes(sdk.ActionModify, json.RawMessage(`{"version":"base"}`), ""), nil
		},
	})

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "payment.create_charge", HookType: HookSync, Priority: 10, FuncName: "payment.create_charge"},
	})

	payload := json.RawMessage(`{"amount":100}`)

	// currentVersion=1 means only the base hook.
	result, err := d.DispatchSyncVersioned(context.Background(), "payment.create_charge", 1, payload)
	require.NoError(t, err)
	assert.JSONEq(t, `{"version":"base"}`, string(result))
}

func TestHasHandlers(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), nil)
	d := NewHookDispatcher(rp, &testPublisher{}, nil, testErrorLogger(), clock.NewReal())

	assert.False(t, d.hasHandlers("nonexistent.hook"))

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "invoice.created", HookType: HookSync, Priority: 10, FuncName: "invoice.created"},
	})

	assert.True(t, d.hasHandlers("invoice.created"))
	assert.False(t, d.hasHandlers("invoice.created.v2"))
}

// --- DispatchSyncSafe Tests ---

func TestDispatchSyncSafe_AllContinue(t *testing.T) {
	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"plugin-a": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return hookResultBytes(sdk.ActionContinue, nil, ""), nil
		},
		"plugin-b": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return hookResultBytes(sdk.ActionContinue, nil, ""), nil
		},
	})

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "order.finalized", HookType: HookSync, Priority: 10, FuncName: "order.finalized"},
		{PluginID: "id-b", PluginSlug: "plugin-b", HookName: "order.finalized", HookType: HookSync, Priority: 20, FuncName: "order.finalized"},
	})

	payload := json.RawMessage(`{"order_id":"o-1"}`)
	result := d.DispatchSyncSafe(context.Background(), "order.finalized", payload)

	require.NoError(t, result.Err)
	assert.False(t, result.Compensated)
	assert.JSONEq(t, `{"order_id":"o-1"}`, string(result.Payload))
	assert.JSONEq(t, `{"order_id":"o-1"}`, string(result.OriginalPayload))
	assert.Equal(t, []string{"plugin-a", "plugin-b"}, result.ExecutedPlugins)
}

func TestDispatchSyncSafe_ModifyChain(t *testing.T) {
	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"plugin-a": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return hookResultBytes(sdk.ActionModify, json.RawMessage(`{"amount":200}`), ""), nil
		},
		"plugin-b": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return hookResultBytes(sdk.ActionModify, json.RawMessage(`{"amount":300}`), ""), nil
		},
	})

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "order.finalized", HookType: HookSync, Priority: 10, FuncName: "order.finalized"},
		{PluginID: "id-b", PluginSlug: "plugin-b", HookName: "order.finalized", HookType: HookSync, Priority: 20, FuncName: "order.finalized"},
	})

	payload := json.RawMessage(`{"amount":100}`)
	result := d.DispatchSyncSafe(context.Background(), "order.finalized", payload)

	require.NoError(t, result.Err)
	assert.False(t, result.Compensated)
	assert.JSONEq(t, `{"amount":300}`, string(result.Payload))
	assert.JSONEq(t, `{"amount":100}`, string(result.OriginalPayload))
	assert.Equal(t, []string{"plugin-a", "plugin-b"}, result.ExecutedPlugins)
}

func TestDispatchSyncSafe_PluginFails_CompensationCalled(t *testing.T) {
	var compensatedSlugs []string

	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"plugin-a": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			// Dispatch to the correct function based on funcName.
			var hookCtx sdk.HookContext
			_ = json.Unmarshal(input, &hookCtx)
			if hookCtx.HookName == "order.finalized.compensate" {
				compensatedSlugs = append(compensatedSlugs, "plugin-a")
				return hookResultBytes(sdk.ActionContinue, nil, ""), nil
			}
			return hookResultBytes(sdk.ActionModify, json.RawMessage(`{"amount":200}`), ""), nil
		},
		"plugin-b": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return nil, fmt.Errorf("plugin-b crashed")
		},
	})

	// Register main hooks and compensation hook for plugin-a.
	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "order.finalized", HookType: HookSync, Priority: 10, FuncName: "order.finalized"},
		{PluginID: "id-b", PluginSlug: "plugin-b", HookName: "order.finalized", HookType: HookSync, Priority: 20, FuncName: "order.finalized"},
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "order.finalized.compensate", HookType: HookSync, Priority: 10, FuncName: "order.finalized.compensate"},
	})

	payload := json.RawMessage(`{"amount":100}`)
	result := d.DispatchSyncSafe(context.Background(), "order.finalized", payload)

	require.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "plugin-b crashed")
	assert.True(t, result.Compensated)
	assert.Equal(t, []string{"plugin-a"}, result.ExecutedPlugins)
	// plugin-a's compensation hook should have been called.
	assert.Equal(t, []string{"plugin-a"}, compensatedSlugs)
	// All compensations succeeded — no failures recorded.
	assert.Empty(t, result.FailedCompensations)
	// Payload should be the last successfully modified payload (plugin-a's modification).
	assert.JSONEq(t, `{"amount":200}`, string(result.Payload))
	// Original payload preserved.
	assert.JSONEq(t, `{"amount":100}`, string(result.OriginalPayload))
}

func TestDispatchSyncSafe_Rollback(t *testing.T) {
	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"plugin-a": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return hookResultBytes(sdk.ActionModify, json.RawMessage(`{"amount":200}`), ""), nil
		},
		"plugin-b": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return hookResultBytes(sdk.ActionRollback, nil, "fraud detected"), nil
		},
	})

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "order.finalized", HookType: HookSync, Priority: 10, FuncName: "order.finalized"},
		{PluginID: "id-b", PluginSlug: "plugin-b", HookName: "order.finalized", HookType: HookSync, Priority: 20, FuncName: "order.finalized"},
	})

	payload := json.RawMessage(`{"amount":100}`)
	result := d.DispatchSyncSafe(context.Background(), "order.finalized", payload)

	require.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "rollback requested")
	assert.Contains(t, result.Err.Error(), "plugin-b")
	assert.True(t, result.Compensated)
	// Rollback returns the ORIGINAL payload, not the modified one.
	assert.JSONEq(t, `{"amount":100}`, string(result.Payload))
	assert.JSONEq(t, `{"amount":100}`, string(result.OriginalPayload))
	assert.Equal(t, []string{"plugin-a"}, result.ExecutedPlugins)
}

func TestDispatchSyncSafe_NoCompensateHook(t *testing.T) {
	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"plugin-a": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return hookResultBytes(sdk.ActionModify, json.RawMessage(`{"amount":200}`), ""), nil
		},
		"plugin-b": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return nil, fmt.Errorf("plugin-b exploded")
		},
	})

	// Register main hooks but NO compensate hook for plugin-a.
	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "order.finalized", HookType: HookSync, Priority: 10, FuncName: "order.finalized"},
		{PluginID: "id-b", PluginSlug: "plugin-b", HookName: "order.finalized", HookType: HookSync, Priority: 20, FuncName: "order.finalized"},
	})

	payload := json.RawMessage(`{"amount":100}`)
	result := d.DispatchSyncSafe(context.Background(), "order.finalized", payload)

	require.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "plugin-b exploded")
	// Compensation was attempted (result.Compensated is true) even though no
	// compensate hook exists — the dispatcher silently skips missing hooks.
	assert.True(t, result.Compensated)
	assert.Equal(t, []string{"plugin-a"}, result.ExecutedPlugins)
	// No compensate hook registered — nothing to fail.
	assert.Empty(t, result.FailedCompensations)
}

func TestDispatchSyncSafe_NoHandlers(t *testing.T) {
	rp := NewRuntimePool(testErrorLogger(), nil)
	d := NewHookDispatcher(rp, &testPublisher{}, nil, testErrorLogger(), clock.NewReal())

	payload := json.RawMessage(`{"amount":100}`)
	result := d.DispatchSyncSafe(context.Background(), "nonexistent.hook", payload)

	require.NoError(t, result.Err)
	assert.False(t, result.Compensated)
	assert.JSONEq(t, `{"amount":100}`, string(result.Payload))
	assert.JSONEq(t, `{"amount":100}`, string(result.OriginalPayload))
	assert.Empty(t, result.ExecutedPlugins)
}

func TestDispatchSyncSafe_HaltTriggersCompensation(t *testing.T) {
	var compensatedSlugs []string

	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"plugin-a": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			var hookCtx sdk.HookContext
			_ = json.Unmarshal(input, &hookCtx)
			if hookCtx.HookName == "order.finalized.compensate" {
				compensatedSlugs = append(compensatedSlugs, "plugin-a")
				return hookResultBytes(sdk.ActionContinue, nil, ""), nil
			}
			return hookResultBytes(sdk.ActionContinue, nil, ""), nil
		},
		"plugin-b": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return hookResultBytes(sdk.ActionHalt, nil, "compliance violation"), nil
		},
	})

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "order.finalized", HookType: HookSync, Priority: 10, FuncName: "order.finalized"},
		{PluginID: "id-b", PluginSlug: "plugin-b", HookName: "order.finalized", HookType: HookSync, Priority: 20, FuncName: "order.finalized"},
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "order.finalized.compensate", HookType: HookSync, Priority: 10, FuncName: "order.finalized.compensate"},
	})

	payload := json.RawMessage(`{"amount":100}`)
	result := d.DispatchSyncSafe(context.Background(), "order.finalized", payload)

	require.Error(t, result.Err)
	assert.ErrorIs(t, result.Err, ErrHookHalted)
	assert.Contains(t, result.Err.Error(), "compliance violation")
	assert.True(t, result.Compensated)
	assert.Equal(t, []string{"plugin-a"}, result.ExecutedPlugins)
	assert.Equal(t, []string{"plugin-a"}, compensatedSlugs)
	// All compensations succeeded — no failures recorded.
	assert.Empty(t, result.FailedCompensations)
}

func TestDispatchSyncSafe_CompensationFails_RecordedInResult(t *testing.T) {
	d, _ := dispatcherWithMock(t, map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error){
		"plugin-a": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			var hookCtx sdk.HookContext
			_ = json.Unmarshal(input, &hookCtx)
			if hookCtx.HookName == "order.finalized.compensate" {
				return nil, fmt.Errorf("compensation DB unavailable")
			}
			return hookResultBytes(sdk.ActionModify, json.RawMessage(`{"amount":200}`), ""), nil
		},
		"plugin-b": func(ctx context.Context, funcName string, input []byte) ([]byte, error) {
			return nil, fmt.Errorf("plugin-b crashed")
		},
	})

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "order.finalized", HookType: HookSync, Priority: 10, FuncName: "order.finalized"},
		{PluginID: "id-b", PluginSlug: "plugin-b", HookName: "order.finalized", HookType: HookSync, Priority: 20, FuncName: "order.finalized"},
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "order.finalized.compensate", HookType: HookSync, Priority: 10, FuncName: "order.finalized.compensate"},
	})

	payload := json.RawMessage(`{"amount":100}`)
	result := d.DispatchSyncSafe(context.Background(), "order.finalized", payload)

	require.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "plugin-b crashed")
	assert.True(t, result.Compensated)
	assert.Equal(t, []string{"plugin-a"}, result.ExecutedPlugins)

	// Compensation for plugin-a failed — should be recorded.
	require.Len(t, result.FailedCompensations, 1)
	assert.Equal(t, hookdispatch.FailedCompensation{
		PluginSlug: "plugin-a",
		HookName:   "order.finalized.compensate",
		Error:      "compensation DB unavailable",
	}, result.FailedCompensations[0])
}
