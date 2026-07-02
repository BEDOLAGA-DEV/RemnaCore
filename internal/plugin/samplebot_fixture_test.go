package plugin

import (
	"context"
	"os"
	"testing"

	extism "github.com/extism/go-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSampleBotFixture loads the committed samplebot.wasm and verifies that:
//  1. The Extism SDK can load it (valid WASM + valid Extism module).
//  2. The expected export "handle_update" is present.
//
// A no-op stub for the host_call import is registered so the module can be
// instantiated without a live bot host. The stub always returns offset 0,
// which the guest treats as a no-error response.
func TestSampleBotFixture(t *testing.T) {
	// Stub host_call so the guest module can be instantiated. The default
	// namespace for NewHostFunctionWithStack is already "extism:host/user",
	// which matches the //go:wasmimport directive in main.go.
	stubHostCall := extism.NewHostFunctionWithStack(
		"host_call",
		func(_ context.Context, _ *extism.CurrentPlugin, stack []uint64) {
			stack[0] = 0 // return offset 0 — guest treats this as success
		},
		[]extism.ValueType{extism.ValueTypePTR},
		[]extism.ValueType{extism.ValueTypePTR},
	)

	manifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmFile{Path: "../../plugins/samplebot/samplebot.wasm"},
		},
	}

	pluginConfig := extism.PluginConfig{
		EnableWasi: true,
	}

	p, err := extism.NewPlugin(context.Background(), manifest, pluginConfig, []extism.HostFunction{stubHostCall})
	require.NoError(t, err, "extism.NewPlugin must load samplebot.wasm without error")
	defer p.Close(context.Background())

	assert.True(t, p.FunctionExists("handle_update"),
		"samplebot.wasm must export handle_update")
}

// TestSampleBotManifest parses the committed plugins/samplebot/plugin.toml and
// verifies it is a valid bot manifest whose declared permissions actually cover
// every op the guest calls (telegram.send_* / user.register / plans.list) — a
// manifest-installed samplebot must not be denied its own operations.
func TestSampleBotManifest(t *testing.T) {
	raw, err := os.ReadFile("../../plugins/samplebot/plugin.toml")
	require.NoError(t, err)

	m, err := ParseManifest(raw)
	require.NoError(t, err)

	require.Equal(t, "samplebot", m.Plugin.ID)
	require.NotNil(t, m.Telegram)
	assert.True(t, m.Telegram.ProvidesBot)
	assert.Equal(t, DefaultBotEntry, m.Telegram.Entry)

	perms := m.ParsePermissions()
	granted := make(map[PermissionScope]bool, len(perms))
	for _, p := range perms {
		granted[p] = true
	}
	assert.True(t, granted[PermTelegramSend], "guest calls telegram.send_text/cabinet.open")
	assert.True(t, granted[PermUsersWrite], "guest calls user.register")
	assert.True(t, granted[PermBillingRead], "guest calls plans.list")
}
