package telegram

import (
	"context"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
)

// WASMBotDispatcher abstracts WASM bot plugin resolution and invocation so that
// dispatchUpdate can route updates to WASM plugins without knowing the runtime
// pool internals, and tests can inject a fake.
type WASMBotDispatcher interface {
	// Resolve checks whether slug names an enabled, ProvidesBot WASM plugin.
	// On success it returns the permission set from the plugin manifest and the
	// WASM export entry point name. ok=false means the caller should fall through
	// to the next resolution strategy (typically the cabinet-bot fallback).
	Resolve(ctx context.Context, slug string) (perms bothost.PermSet, entry string, ok bool)

	// Invoke serialises the update envelope and executes the plugin's entry
	// function via the runtime pool. ctx must carry a plugin.BotHostBridge
	// (injected by the caller via plugin.WithBotHost) so the plugin can issue
	// host_call operations during the invocation.
	Invoke(ctx context.Context, slug, entry string, envelope []byte) error
}

// PluginReader is the narrow slice of the plugin repository the WASM bot
// dispatcher needs: resolving a plugin by slug. Depending on this instead of
// the full plugin.PluginRepository keeps the dispatcher decoupled and its test
// double to a single method.
type PluginReader interface {
	GetBySlug(ctx context.Context, slug string) (*plugin.Plugin, error)
}

// runtimePoolDispatcher is the production WASMBotDispatcher backed by the
// plugin runtime pool and the plugin repository.
type runtimePoolDispatcher struct {
	pool    *plugin.RuntimePool
	plugins PluginReader
}

// Resolve looks up slug in the plugin repository and returns its permission set
// and entry point when the plugin is enabled and declares ProvidesBot=true.
// Any error or missing/disabled/non-bot plugin yields ok=false.
func (d *runtimePoolDispatcher) Resolve(ctx context.Context, slug string) (bothost.PermSet, string, bool) {
	p, err := d.plugins.GetBySlug(ctx, slug)
	if err != nil || p == nil {
		return nil, "", false
	}
	if p.Status != plugin.StatusEnabled {
		return nil, "", false
	}
	if p.Manifest == nil || p.Manifest.Telegram == nil || !p.Manifest.Telegram.ProvidesBot {
		return nil, "", false
	}
	perms := bothost.NewPermSet(p.Permissions...)
	entry := p.Manifest.Telegram.Entry
	if entry == "" {
		entry = plugin.DefaultBotEntry
	}
	return perms, entry, true
}

// Invoke calls funcName on the slug plugin via the runtime pool and discards the
// output (bot plugins produce effects via host_call, not return values).
func (d *runtimePoolDispatcher) Invoke(ctx context.Context, slug, entry string, envelope []byte) error {
	_, err := d.pool.CallHook(ctx, slug, entry, envelope)
	return err
}

// NewWASMBotDispatcher wires the production runtimePoolDispatcher into the fx
// dependency graph. pool and plugins are provided by the plugin module
// (plugin.PluginRepository satisfies PluginReader).
func NewWASMBotDispatcher(pool *plugin.RuntimePool, plugins PluginReader) WASMBotDispatcher {
	return &runtimePoolDispatcher{pool: pool, plugins: plugins}
}
