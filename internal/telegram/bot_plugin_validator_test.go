package telegram

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
)

// newValidatorTestFixture builds a BuiltinBotRegistry with a single registered
// built-in slug ("builtin-bot") and a fresh botPluginValidator wired to the
// given PluginReader stub.
func newValidatorTestFixture(repo PluginReader) (*botPluginValidator, string) {
	reg := NewBuiltinBotRegistry()
	const builtinSlug = "builtin-bot"
	reg.Register(builtinSlug, bothost.NewPermSet(), func(_ context.Context, _ bothost.Update, _ bothost.Host) error {
		return nil
	})
	v := &botPluginValidator{reg: reg, plugins: repo}
	return v, builtinSlug
}

// TestBotPluginValidator_BuiltinSlug verifies that a registered built-in slug
// is accepted without consulting the plugin repo.
func TestBotPluginValidator_BuiltinSlug(t *testing.T) {
	// repo returns not-found so that if the validator mistakenly queries it for
	// a built-in slug the test would catch the leak.
	repo := &stubPluginRepo{
		plugin: nil,
		err:    fmt.Errorf("get plugin by slug: %w", plugin.ErrPluginNotFound),
	}
	v, builtinSlug := newValidatorTestFixture(repo)

	ok, err := v.IsValidBotPlugin(context.Background(), builtinSlug)

	require.NoError(t, err)
	assert.True(t, ok, "built-in slug must be accepted")
}

// TestBotPluginValidator_EnabledWASMBotPlugin verifies that an enabled WASM
// plugin with ProvidesBot=true is accepted.
func TestBotPluginValidator_EnabledWASMBotPlugin(t *testing.T) {
	p := &plugin.Plugin{
		Status: plugin.StatusEnabled,
		Manifest: &plugin.Manifest{
			Telegram: &plugin.ManifestTelegram{ProvidesBot: true},
		},
	}
	repo := &stubPluginRepo{plugin: p}
	v, _ := newValidatorTestFixture(repo)

	ok, err := v.IsValidBotPlugin(context.Background(), "wasm-bot")

	require.NoError(t, err)
	assert.True(t, ok, "enabled WASM bot plugin must be accepted")
}

// TestBotPluginValidator_DisabledPlugin verifies that a disabled plugin is
// rejected with (false, nil).
func TestBotPluginValidator_DisabledPlugin(t *testing.T) {
	p := &plugin.Plugin{
		Status: plugin.StatusDisabled,
		Manifest: &plugin.Manifest{
			Telegram: &plugin.ManifestTelegram{ProvidesBot: true},
		},
	}
	repo := &stubPluginRepo{plugin: p}
	v, _ := newValidatorTestFixture(repo)

	ok, err := v.IsValidBotPlugin(context.Background(), "disabled-bot")

	require.NoError(t, err)
	assert.False(t, ok, "disabled plugin must be rejected")
}

// TestBotPluginValidator_EnabledNonBotPlugin verifies that an enabled plugin
// that does not declare ProvidesBot (Telegram section is nil) is rejected with
// (false, nil).
func TestBotPluginValidator_EnabledNonBotPlugin(t *testing.T) {
	p := &plugin.Plugin{
		Status:   plugin.StatusEnabled,
		Manifest: &plugin.Manifest{Telegram: nil},
	}
	repo := &stubPluginRepo{plugin: p}
	v, _ := newValidatorTestFixture(repo)

	ok, err := v.IsValidBotPlugin(context.Background(), "non-bot-plugin")

	require.NoError(t, err)
	assert.False(t, ok, "enabled non-bot plugin must be rejected")
}

// TestBotPluginValidator_UnknownSlug verifies that an unknown slug (repo
// returns ErrPluginNotFound) yields (false, nil) — a clean rejection, not an
// infrastructure error.
func TestBotPluginValidator_UnknownSlug(t *testing.T) {
	notFoundErr := fmt.Errorf("get plugin by slug: %w", plugin.ErrPluginNotFound)
	repo := &stubPluginRepo{err: notFoundErr}
	v, _ := newValidatorTestFixture(repo)

	ok, err := v.IsValidBotPlugin(context.Background(), "unknown-slug")

	require.NoError(t, err, "not-found must be a clean rejection, not an error")
	assert.False(t, ok)
}

// TestBotPluginValidator_RepoInfraError verifies that a genuine infrastructure
// error (not a not-found) is propagated as (false, err).
func TestBotPluginValidator_RepoInfraError(t *testing.T) {
	infraErr := errors.New("connection refused")
	repo := &stubPluginRepo{err: infraErr}
	v, _ := newValidatorTestFixture(repo)

	ok, err := v.IsValidBotPlugin(context.Background(), "any-slug")

	assert.False(t, ok)
	assert.ErrorIs(t, err, infraErr, "infrastructure error must be propagated")
}

// TestBotPluginValidator_NonBotWASMPlugin_TelegramSectionPresent verifies that
// an enabled WASM plugin whose Telegram section exists but has
// ProvidesBot=false is rejected.
func TestBotPluginValidator_NonBotWASMPlugin_TelegramSectionPresent(t *testing.T) {
	p := &plugin.Plugin{
		Status: plugin.StatusEnabled,
		Manifest: &plugin.Manifest{
			Telegram: &plugin.ManifestTelegram{ProvidesBot: false},
		},
	}
	repo := &stubPluginRepo{plugin: p}
	v, _ := newValidatorTestFixture(repo)

	ok, err := v.IsValidBotPlugin(context.Background(), "wasm-bot")

	require.NoError(t, err)
	assert.False(t, ok, "plugin without ProvidesBot must be rejected")
}
