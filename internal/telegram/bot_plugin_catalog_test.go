package telegram

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/cabinetbot"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
)

// stubPluginLister is a PluginLister stub returning a fixed set of enabled
// plugins (or an error) for catalog unit tests.
type stubPluginLister struct {
	plugins []*plugin.Plugin
	err     error
}

func (s *stubPluginLister) GetEnabled(_ context.Context) ([]*plugin.Plugin, error) {
	return s.plugins, s.err
}

// registerCabinetBotFixture registers the cabinet-bot handler in reg so the
// catalog treats it as a live built-in bot.
func registerCabinetBotFixture(reg *BuiltinBotRegistry) {
	reg.Register(cabinetbot.Slug, bothost.NewPermSet(), func(_ context.Context, _ bothost.Update, _ bothost.Host) error {
		return nil
	})
}

// wasmBotManifestPlugin builds an enabled WASM plugin whose manifest declares a
// Telegram bot.
func wasmBotManifestPlugin(slug, name string) *plugin.Plugin {
	return &plugin.Plugin{
		Slug:     slug,
		Name:     name,
		Status:   plugin.StatusEnabled,
		Manifest: &plugin.Manifest{Telegram: &plugin.ManifestTelegram{ProvidesBot: true}},
	}
}

func findBySlug(list []BotPluginInfo, slug string) (BotPluginInfo, bool) {
	for _, info := range list {
		if info.Slug == slug {
			return info, true
		}
	}
	return BotPluginInfo{}, false
}

// TestBotPluginCatalog_BuiltinRegistered verifies that a cabinet-bot registered
// in the registry appears in the list with kind=builtin.
func TestBotPluginCatalog_BuiltinRegistered(t *testing.T) {
	reg := NewBuiltinBotRegistry()
	registerCabinetBotFixture(reg)
	catalog := NewBotPluginCatalog(reg, &stubPluginLister{})

	list, err := catalog.ListBotPlugins(context.Background())

	require.NoError(t, err)
	info, ok := findBySlug(list, cabinetbot.Slug)
	require.True(t, ok, "registered cabinet-bot must be listed")
	assert.Equal(t, BotPluginKindBuiltin, info.Kind)
}

// TestBotPluginCatalog_BuiltinNotRegistered verifies that a built-in not present
// in the registry is excluded.
func TestBotPluginCatalog_BuiltinNotRegistered(t *testing.T) {
	reg := NewBuiltinBotRegistry() // cabinet-bot NOT registered
	catalog := NewBotPluginCatalog(reg, &stubPluginLister{})

	list, err := catalog.ListBotPlugins(context.Background())

	require.NoError(t, err)
	_, ok := findBySlug(list, cabinetbot.Slug)
	assert.False(t, ok, "unregistered built-in must be excluded")
}

// TestBotPluginCatalog_WASMBotIncluded verifies that an enabled WASM plugin
// whose manifest declares ProvidesBot is listed with kind=wasm.
func TestBotPluginCatalog_WASMBotIncluded(t *testing.T) {
	reg := NewBuiltinBotRegistry()
	lister := &stubPluginLister{plugins: []*plugin.Plugin{
		wasmBotManifestPlugin("wasm-bot", "WASM Bot"),
	}}
	catalog := NewBotPluginCatalog(reg, lister)

	list, err := catalog.ListBotPlugins(context.Background())

	require.NoError(t, err)
	info, ok := findBySlug(list, "wasm-bot")
	require.True(t, ok, "enabled WASM bot plugin must be listed")
	assert.Equal(t, BotPluginKindWASM, info.Kind)
}

// TestBotPluginCatalog_WASMNonBotExcluded verifies that an enabled WASM plugin
// with no Telegram section is excluded.
func TestBotPluginCatalog_WASMNonBotExcluded(t *testing.T) {
	reg := NewBuiltinBotRegistry()
	lister := &stubPluginLister{plugins: []*plugin.Plugin{
		{Slug: "non-bot", Name: "Non Bot", Status: plugin.StatusEnabled, Manifest: &plugin.Manifest{Telegram: nil}},
	}}
	catalog := NewBotPluginCatalog(reg, lister)

	list, err := catalog.ListBotPlugins(context.Background())

	require.NoError(t, err)
	_, ok := findBySlug(list, "non-bot")
	assert.False(t, ok, "non-bot WASM plugin must be excluded")
}

// TestBotPluginCatalog_WASMNilManifestExcluded verifies that an enabled WASM
// plugin with a nil manifest is excluded.
func TestBotPluginCatalog_WASMNilManifestExcluded(t *testing.T) {
	reg := NewBuiltinBotRegistry()
	lister := &stubPluginLister{plugins: []*plugin.Plugin{
		{Slug: "no-manifest", Name: "No Manifest", Status: plugin.StatusEnabled, Manifest: nil},
	}}
	catalog := NewBotPluginCatalog(reg, lister)

	list, err := catalog.ListBotPlugins(context.Background())

	require.NoError(t, err)
	_, ok := findBySlug(list, "no-manifest")
	assert.False(t, ok, "nil-manifest WASM plugin must be excluded")
}

// TestBotPluginCatalog_Ordering verifies that built-in bots appear before WASM
// bots.
func TestBotPluginCatalog_Ordering(t *testing.T) {
	reg := NewBuiltinBotRegistry()
	registerCabinetBotFixture(reg)
	lister := &stubPluginLister{plugins: []*plugin.Plugin{
		wasmBotManifestPlugin("zeta-bot", "Zeta Bot"),
		wasmBotManifestPlugin("alpha-bot", "Alpha Bot"),
	}}
	catalog := NewBotPluginCatalog(reg, lister)

	list, err := catalog.ListBotPlugins(context.Background())

	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, BotPluginKindBuiltin, list[0].Kind, "built-ins must come first")
	assert.Equal(t, BotPluginKindWASM, list[1].Kind)
	assert.Equal(t, BotPluginKindWASM, list[2].Kind)
	// WASM group sorted by Name.
	assert.Equal(t, "alpha-bot", list[1].Slug)
	assert.Equal(t, "zeta-bot", list[2].Slug)
}

// TestBotPluginCatalog_ListerError verifies that a lister error is propagated.
func TestBotPluginCatalog_ListerError(t *testing.T) {
	reg := NewBuiltinBotRegistry()
	lister := &stubPluginLister{err: assert.AnError}
	catalog := NewBotPluginCatalog(reg, lister)

	_, err := catalog.ListBotPlugins(context.Background())

	assert.ErrorIs(t, err, assert.AnError)
}
