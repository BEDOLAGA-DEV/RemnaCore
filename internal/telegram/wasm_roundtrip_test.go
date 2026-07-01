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
// host_call requests reach the bothost op-registry: user.register, cabinet.open,
// plans.list, and telegram.send_text (with the first offer's name), carrying
// the fields mapped from the update. This is the end-to-end proof of BP3 Task 6:
// the guest decodes the {ok} envelope from plans.list, reads the offer name, and
// routes it through a follow-up telegram.send_text — proving the discriminated
// result envelope is usable for data-returning domain ops.
func TestWASMBotRoundTrip(t *testing.T) {
	wasmBytes, err := os.ReadFile(samplebotWASMPath)
	require.NoError(t, err)

	// Recording op-registry: capture the args of all ops the guest calls.
	// (The real std/domain ops are unit-tested in bothost/*_test.go; here we only
	// need to prove the host_call requests arrive with the right shape and that
	// the guest can read the plans.list {ok} result and act on it.)
	reg := bothost.NewRegistry()
	var gotRegister, gotCabinet, gotPlansList, gotSendText json.RawMessage
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
	reg.Register(bothost.OpPlansList, plugin.PermBillingRead,
		func(_ context.Context, _ *bothost.OpContext, args json.RawMessage) (json.RawMessage, error) {
			gotPlansList = args
			return json.RawMessage(`[{"plan_id":"p-1","name":"Starter"}]`), nil
		})
	reg.Register(bothost.OpTelegramSendText, plugin.PermTelegramSend,
		func(_ context.Context, _ *bothost.OpContext, args json.RawMessage) (json.RawMessage, error) {
			gotSendText = args
			return nil, nil
		})
	oc := &bothost.OpContext{TenantID: "tenant-1", CabinetURL: "https://cabinet.example"}
	granted := bothost.NewPermSet(plugin.PermTelegramSend, plugin.PermUsersWrite, plugin.PermBillingRead)
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

	// plans.list reached — the guest called the domain op.
	require.NotNil(t, gotPlansList, "plans.list must be called via host_call")

	// telegram.send_text reached with the first offer's name — proving the guest
	// decoded the {ok} envelope from plans.list, read the offer name, and acted
	// on it. This is the load-bearing assertion of BP3 Task 6.
	require.NotNil(t, gotSendText, "telegram.send_text must be called after plans.list returns offers")
	var sendArgs map[string]any
	require.NoError(t, json.Unmarshal(gotSendText, &sendArgs))
	require.Equal(t, "Starter", sendArgs["text"], "send_text text must equal the first offer's name")
	require.EqualValues(t, 4242, sendArgs["chat_id"], "send_text chat_id must match the update's chat_id")
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
