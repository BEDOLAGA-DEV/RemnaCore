package plugin

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
)

// mockPublisher records all published events for assertions.
type mockPublisher struct {
	events []domainevent.Event
}

func (mp *mockPublisher) Publish(_ context.Context, event domainevent.Event) error {
	mp.events = append(mp.events, event)
	return nil
}

func dispatcherLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// dispatcherWithMock creates a HookDispatcher backed by a mock runtime pool
// where each slug maps to a specific WASMRunner call function.
func dispatcherWithMock(t *testing.T, slugCallFns map[string]func(ctx context.Context, funcName string, input []byte) ([]byte, error)) (*HookDispatcher, *mockPublisher) {
	t.Helper()

	logger := dispatcherLogger()
	pub := &mockPublisher{}

	rp := NewRuntimePool(logger, nil)
	for slug, callFn := range slugCallFns {
		p := testPlugin(slug)
		require.NoError(t, rp.LoadPlugin(p))

		// Manually set the runner with the provided callFn.
		rp.mu.Lock()
		rp.plugins[slug].Runner = &mockRunner{callFn: callFn}
		rp.mu.Unlock()
	}

	d := NewHookDispatcher(rp, pub, logger)
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
	rp := NewRuntimePool(dispatcherLogger(), nil)
	d := NewHookDispatcher(rp, &mockPublisher{}, dispatcherLogger())

	payload := json.RawMessage(`{"amount":100}`)
	result, err := d.DispatchSync(context.Background(), "nonexistent.hook", payload)
	require.NoError(t, err)

	// Should pass through unchanged.
	assert.JSONEq(t, `{"amount":100}`, string(result))
}

func TestRegisterHooks_And_UnregisterHooks(t *testing.T) {
	rp := NewRuntimePool(dispatcherLogger(), nil)
	d := NewHookDispatcher(rp, &mockPublisher{}, dispatcherLogger())

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
	rp := NewRuntimePool(dispatcherLogger(), nil)
	pub := &mockPublisher{}
	d := NewHookDispatcher(rp, pub, dispatcherLogger())

	payload := json.RawMessage(`{"user_id":"u-1"}`)
	err := d.DispatchAsync(context.Background(), "subscription.renewed", payload)
	require.NoError(t, err)

	require.Len(t, pub.events, 1)
	assert.Equal(t, domainevent.EventType("plugin.hook.subscription.renewed"), pub.events[0].Type)
	assert.Equal(t, "subscription.renewed", pub.events[0].Data["hook_name"])
}

func TestDispatchAsync_NilPublisher(t *testing.T) {
	rp := NewRuntimePool(dispatcherLogger(), nil)
	d := NewHookDispatcher(rp, nil, dispatcherLogger())

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

func TestUnregisterHooks_DoesNotAffectOtherPlugins(t *testing.T) {
	rp := NewRuntimePool(dispatcherLogger(), nil)
	d := NewHookDispatcher(rp, &mockPublisher{}, dispatcherLogger())

	d.RegisterHooks([]HookRegistration{
		{PluginID: "id-a", PluginSlug: "plugin-a", HookName: "invoice.created", HookType: HookSync, Priority: 10, FuncName: "invoice.created"},
		{PluginID: "id-b", PluginSlug: "plugin-b", HookName: "invoice.created", HookType: HookSync, Priority: 20, FuncName: "invoice.created"},
	})

	d.UnregisterHooks("plugin-a")

	regs := d.Registrations("invoice.created")
	require.Len(t, regs, 1)
	assert.Equal(t, "plugin-b", regs[0].PluginSlug)
}
