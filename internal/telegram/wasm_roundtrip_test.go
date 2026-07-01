package telegram

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

const samplebotWASMPath = "../../plugins/samplebot/samplebot.wasm"

// TestWASMBotRoundTrip loads the committed sample WASM bot through a real
// RuntimePool and invokes its handle_update export, asserting that the guest's
// host_call requests reach the bothost op-registry: user.register then
// cabinet.open, carrying the fields mapped from the update. This is the
// end-to-end proof of the BP2 ABI — the guest's `//go:wasmimport
// extism:host/user host_call` links to the host registration, the per-dispatch
// op-host bridged via ctx is reachable, and effects flow through the registry.
func TestWASMBotRoundTrip(t *testing.T) {
	wasmBytes, err := os.ReadFile(samplebotWASMPath)
	require.NoError(t, err)

	// Recording op-registry: capture the args of the two ops the guest calls.
	// (The real std ops are unit-tested in bothost/ops_test.go; here we only
	// need to prove the host_call requests arrive with the right shape.)
	reg := bothost.NewRegistry()
	var gotRegister, gotCabinet json.RawMessage
	reg.Register(bothost.OpUserRegister, plugin.PermUsersWrite,
		func(_ context.Context, _ *bothost.OpContext, args json.RawMessage) (json.RawMessage, error) {
			gotRegister = args
			return json.RawMessage(`{"user_id":"u-1"}`), nil
		})
	reg.Register(bothost.OpCabinetOpen, plugin.PermTelegramSend,
		func(_ context.Context, _ *bothost.OpContext, args json.RawMessage) (json.RawMessage, error) {
			gotCabinet = args
			return nil, nil
		})
	oc := &bothost.OpContext{TenantID: "tenant-1", CabinetURL: "https://cabinet.example"}
	granted := bothost.NewPermSet(plugin.PermTelegramSend, plugin.PermUsersWrite)
	host := reg.Bind(oc, granted)

	// Real extism runtime pool loading the committed fixture. HostFunctions must
	// be non-nil so buildExtismHostFunctions registers host_call for the guest.
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hf := plugin.NewHostFunctions(logger, clock.NewReal())
	pool := plugin.NewRuntimePool(logger, plugin.ExtismRunnerFactory(hf))

	m := &plugin.Manifest{
		Plugin:   plugin.ManifestPlugin{ID: "samplebot", Name: "Sample Bot", Version: "1.0.0", SDKVersion: plugin.CurrentSDKVersion},
		Telegram: &plugin.ManifestTelegram{ProvidesBot: true, Entry: plugin.DefaultBotEntry},
	}
	p, err := plugin.NewPlugin(m, wasmBytes, time.Now())
	require.NoError(t, err)
	require.NoError(t, pool.LoadPlugin(p))

	// Invoke handle_update with a /start-like update, bridging the op-host via ctx.
	upd := bothost.Update{ChatID: 4242, From: bothost.User{ID: 77, FirstName: "Neo"}, Text: "/start"}
	env, err := json.Marshal(upd)
	require.NoError(t, err)
	ctx := plugin.WithBotHost(context.Background(), host)
	_, err = pool.CallHook(ctx, "samplebot", plugin.DefaultBotEntry, env)
	require.NoError(t, err)

	// user.register reached, carrying the mapped telegram id + display name.
	require.NotNil(t, gotRegister, "user.register must be called via host_call")
	var regArgs map[string]any
	require.NoError(t, json.Unmarshal(gotRegister, &regArgs))
	require.EqualValues(t, 77, regArgs["telegram_id"])
	require.Equal(t, "Neo", regArgs["display_name"])

	// cabinet.open reached, carrying the chat id.
	require.NotNil(t, gotCabinet, "cabinet.open must be called via host_call")
	var cabArgs map[string]any
	require.NoError(t, json.Unmarshal(gotCabinet, &cabArgs))
	require.EqualValues(t, 4242, cabArgs["chat_id"])
}

// TestWASMBotRoundTrip_PermissionDenied proves the permission gate holds across
// the real WASM boundary: a plugin granted only telegram:send calls
// user.register (which requires users:write); the registry denies it, the host
// returns an error envelope, and the guest's handle_update fails — so CallHook
// errors and cabinet.open is never reached. This is the end-to-end proof that
// enforcement is the granted PermSet inside Registry.Call, not host_call import
// visibility (host_call is registered for every plugin).
func TestWASMBotRoundTrip_PermissionDenied(t *testing.T) {
	wasmBytes, err := os.ReadFile(samplebotWASMPath)
	require.NoError(t, err)

	reg := bothost.NewRegistry()
	var cabinetCalled bool
	reg.Register(bothost.OpUserRegister, plugin.PermUsersWrite,
		func(_ context.Context, _ *bothost.OpContext, _ json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"user_id":"u-1"}`), nil
		})
	reg.Register(bothost.OpCabinetOpen, plugin.PermTelegramSend,
		func(_ context.Context, _ *bothost.OpContext, _ json.RawMessage) (json.RawMessage, error) {
			cabinetCalled = true
			return nil, nil
		})
	oc := &bothost.OpContext{TenantID: "tenant-1", CabinetURL: "https://cabinet.example"}
	granted := bothost.NewPermSet(plugin.PermTelegramSend) // deliberately NOT users:write
	host := reg.Bind(oc, granted)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hf := plugin.NewHostFunctions(logger, clock.NewReal())
	pool := plugin.NewRuntimePool(logger, plugin.ExtismRunnerFactory(hf))

	m := &plugin.Manifest{
		Plugin:   plugin.ManifestPlugin{ID: "samplebot", Name: "Sample Bot", Version: "1.0.0", SDKVersion: plugin.CurrentSDKVersion},
		Telegram: &plugin.ManifestTelegram{ProvidesBot: true, Entry: plugin.DefaultBotEntry},
	}
	p, err := plugin.NewPlugin(m, wasmBytes, time.Now())
	require.NoError(t, err)
	require.NoError(t, pool.LoadPlugin(p))

	upd := bothost.Update{ChatID: 4242, From: bothost.User{ID: 77, FirstName: "Neo"}, Text: "/start"}
	env, err := json.Marshal(upd)
	require.NoError(t, err)
	ctx := plugin.WithBotHost(context.Background(), host)
	_, err = pool.CallHook(ctx, "samplebot", plugin.DefaultBotEntry, env)

	require.Error(t, err, "user.register must be denied without the users:write grant")
	require.False(t, cabinetCalled, "cabinet.open must not run after the denied user.register")
}
