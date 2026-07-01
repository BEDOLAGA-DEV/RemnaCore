package telegram

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
)

// stubPluginRepo is a minimal plugin.PluginRepository stub for Resolve unit tests.
// Only GetBySlug is implemented; all other methods panic to catch unexpected calls.
type stubPluginRepo struct {
	plugin *plugin.Plugin
	err    error
}

func (s *stubPluginRepo) GetBySlug(_ context.Context, _ string) (*plugin.Plugin, error) {
	return s.plugin, s.err
}

func (s *stubPluginRepo) Create(_ context.Context, _ *plugin.Plugin) error {
	panic("not called in Resolve tests")
}

func (s *stubPluginRepo) GetByID(_ context.Context, _ string) (*plugin.Plugin, error) {
	panic("not called in Resolve tests")
}

func (s *stubPluginRepo) GetAll(_ context.Context) ([]*plugin.Plugin, error) {
	panic("not called in Resolve tests")
}

func (s *stubPluginRepo) GetEnabled(_ context.Context) ([]*plugin.Plugin, error) {
	panic("not called in Resolve tests")
}

func (s *stubPluginRepo) GetByStatus(_ context.Context, _ plugin.PluginStatus) ([]*plugin.Plugin, error) {
	panic("not called in Resolve tests")
}

func (s *stubPluginRepo) UpdateStatus(_ context.Context, _ string, _ plugin.PluginStatus, _ string, _ *time.Time) error {
	panic("not called in Resolve tests")
}

func (s *stubPluginRepo) UpdateConfig(_ context.Context, _ string, _ map[string]string) error {
	panic("not called in Resolve tests")
}

func (s *stubPluginRepo) UpdatePlugin(_ context.Context, _ *plugin.Plugin) error {
	panic("not called in Resolve tests")
}

func (s *stubPluginRepo) Delete(_ context.Context, _ string) error {
	panic("not called in Resolve tests")
}

func (s *stubPluginRepo) GetWASMByHash(_ context.Context, _ string) ([]byte, error) {
	panic("not called in Resolve tests")
}

func (s *stubPluginRepo) StoreWASM(_ context.Context, _ string, _ []byte) error {
	panic("not called in Resolve tests")
}

// botEnabledPlugin builds a minimal enabled bot plugin for testing Resolve.
func botEnabledPlugin(perms []plugin.PermissionScope, entry string) *plugin.Plugin {
	return &plugin.Plugin{
		Status:      plugin.StatusEnabled,
		Permissions: perms,
		Manifest: &plugin.Manifest{
			Telegram: &plugin.ManifestTelegram{
				ProvidesBot: true,
				Entry:       entry,
			},
		},
	}
}

// TestRuntimePoolDispatcher_Resolve_EnabledBotPlugin verifies that an enabled
// plugin with ProvidesBot=true returns perms, entry, and ok=true.
func TestRuntimePoolDispatcher_Resolve_EnabledBotPlugin(t *testing.T) {
	p := botEnabledPlugin([]plugin.PermissionScope{plugin.PermTelegramSend}, "handle_update")
	repo := &stubPluginRepo{plugin: p}
	d := &runtimePoolDispatcher{pool: nil, plugins: repo}

	perms, entry, ok := d.Resolve(context.Background(), "wasmbot")

	require.True(t, ok)
	assert.Equal(t, "handle_update", entry)
	assert.True(t, perms.Has(plugin.PermTelegramSend))
}

// TestRuntimePoolDispatcher_Resolve_DefaultEntry verifies that an empty entry
// in the manifest is defaulted to plugin.DefaultBotEntry.
func TestRuntimePoolDispatcher_Resolve_DefaultEntry(t *testing.T) {
	p := botEnabledPlugin([]plugin.PermissionScope{}, "") // empty entry → default
	repo := &stubPluginRepo{plugin: p}
	d := &runtimePoolDispatcher{pool: nil, plugins: repo}

	_, entry, ok := d.Resolve(context.Background(), "wasmbot")

	require.True(t, ok)
	assert.Equal(t, plugin.DefaultBotEntry, entry)
}

// TestRuntimePoolDispatcher_Resolve_DisabledPlugin verifies that a disabled
// plugin returns ok=false.
func TestRuntimePoolDispatcher_Resolve_DisabledPlugin(t *testing.T) {
	p := &plugin.Plugin{
		Status: plugin.StatusDisabled,
		Manifest: &plugin.Manifest{
			Telegram: &plugin.ManifestTelegram{ProvidesBot: true},
		},
	}
	repo := &stubPluginRepo{plugin: p}
	d := &runtimePoolDispatcher{pool: nil, plugins: repo}

	_, _, ok := d.Resolve(context.Background(), "wasmbot")
	assert.False(t, ok)
}

// TestRuntimePoolDispatcher_Resolve_NonBotPlugin verifies that an enabled plugin
// with no Telegram section (not a bot) returns ok=false.
func TestRuntimePoolDispatcher_Resolve_NonBotPlugin(t *testing.T) {
	p := &plugin.Plugin{
		Status:   plugin.StatusEnabled,
		Manifest: &plugin.Manifest{Telegram: nil},
	}
	repo := &stubPluginRepo{plugin: p}
	d := &runtimePoolDispatcher{pool: nil, plugins: repo}

	_, _, ok := d.Resolve(context.Background(), "wasmbot")
	assert.False(t, ok)
}

// TestRuntimePoolDispatcher_Resolve_GetBySlugError verifies that a repo error
// returns ok=false.
func TestRuntimePoolDispatcher_Resolve_GetBySlugError(t *testing.T) {
	repo := &stubPluginRepo{err: errors.New("db error")}
	d := &runtimePoolDispatcher{pool: nil, plugins: repo}

	_, _, ok := d.Resolve(context.Background(), "wasmbot")
	assert.False(t, ok)
}

// TestRuntimePoolDispatcher_Resolve_NilPlugin verifies that a nil plugin (not
// found, no error) returns ok=false.
func TestRuntimePoolDispatcher_Resolve_NilPlugin(t *testing.T) {
	repo := &stubPluginRepo{plugin: nil, err: nil}
	d := &runtimePoolDispatcher{pool: nil, plugins: repo}

	_, _, ok := d.Resolve(context.Background(), "wasmbot")
	assert.False(t, ok)
}

// TestRuntimePoolDispatcher_Resolve_PermsMapped verifies that Permissions on
// the plugin are reflected in the returned PermSet.
func TestRuntimePoolDispatcher_Resolve_PermsMapped(t *testing.T) {
	p := botEnabledPlugin(
		[]plugin.PermissionScope{plugin.PermTelegramSend, plugin.PermUsersWrite},
		"handle_update",
	)
	repo := &stubPluginRepo{plugin: p}
	d := &runtimePoolDispatcher{pool: nil, plugins: repo}

	perms, _, ok := d.Resolve(context.Background(), "wasmbot")

	require.True(t, ok)
	assert.True(t, perms.Has(plugin.PermTelegramSend))
	assert.True(t, perms.Has(plugin.PermUsersWrite))
}

// fakeWASMDispatcher is a test double for WASMBotDispatcher.
type fakeWASMDispatcher struct {
	resolvePerms bothost.PermSet
	resolveEntry string
	resolveOK    bool

	invokeCalled bool
	invokeSlug   string
	invokeEntry  string
	invokeEnv    []byte
}

func (f *fakeWASMDispatcher) Resolve(_ context.Context, _ string) (bothost.PermSet, string, bool) {
	return f.resolvePerms, f.resolveEntry, f.resolveOK
}

func (f *fakeWASMDispatcher) Invoke(_ context.Context, slug, entry string, envelope []byte) error {
	f.invokeCalled = true
	f.invokeSlug = slug
	f.invokeEntry = entry
	f.invokeEnv = make([]byte, len(envelope))
	copy(f.invokeEnv, envelope)
	return nil
}
