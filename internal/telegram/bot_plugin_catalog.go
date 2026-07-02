package telegram

import (
	"context"
	"sort"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
)

// Bot-plugin kinds distinguish native Go bot plugins from WASM bot plugins in
// the catalog listing.
const (
	BotPluginKindBuiltin = "builtin"
	BotPluginKindWASM    = "wasm"
)

// BotPluginInfo describes a bot plugin a shop may select as its Telegram bot.
type BotPluginInfo struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Description string `json:"description,omitempty"`
}

// BotPluginCatalog lists every bot plugin selectable as a shop bot: registered
// built-in bots plus enabled WASM plugins whose manifest declares ProvidesBot.
type BotPluginCatalog struct {
	reg     *BuiltinBotRegistry
	plugins PluginLister
}

// NewBotPluginCatalog constructs a BotPluginCatalog backed by the built-in bot
// registry and a PluginLister for enabled WASM plugins.
func NewBotPluginCatalog(reg *BuiltinBotRegistry, plugins PluginLister) *BotPluginCatalog {
	return &BotPluginCatalog{reg: reg, plugins: plugins}
}

// isEnabledWASMBot reports whether a plugin's manifest declares a Telegram bot
// (ProvidesBot=true). It is the shared manifest predicate used by both the
// catalog and botPluginValidator to avoid drift. It does NOT check plugin
// status: catalog callers source plugins from GetEnabled (already enabled),
// while the validator applies its own status check before calling this.
func isEnabledWASMBot(p *plugin.Plugin) bool {
	return p.Manifest != nil && p.Manifest.Telegram != nil && p.Manifest.Telegram.ProvidesBot
}

// ListBotPlugins returns every selectable bot plugin: registered built-in bots
// first (sorted by name), then enabled WASM bot plugins (sorted by name).
func (c *BotPluginCatalog) ListBotPlugins(ctx context.Context) ([]BotPluginInfo, error) {
	var result []BotPluginInfo

	// Built-in bots: a def qualifies only when it declares ProvidesBot AND its
	// handler is actually registered in the live registry.
	for _, def := range builtin.AllPlugins() {
		if !def.ProvidesBot {
			continue
		}
		if _, _, ok := c.reg.Lookup(def.Slug); !ok {
			continue
		}
		result = append(result, BotPluginInfo{
			Slug:        def.Slug,
			Name:        def.Name,
			Kind:        BotPluginKindBuiltin,
			Description: def.Description,
		})
	}
	builtinCount := len(result)

	// WASM bots: enabled plugins whose manifest declares a Telegram bot.
	plugins, err := c.plugins.GetEnabled(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range plugins {
		if !isEnabledWASMBot(p) {
			continue
		}
		result = append(result, BotPluginInfo{
			Slug:        p.Slug,
			Name:        p.Name,
			Kind:        BotPluginKindWASM,
			Description: p.Description,
		})
	}

	// Built-ins stay ahead of WASM plugins; sort by name within each group.
	sort.SliceStable(result[:builtinCount], func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	sort.SliceStable(result[builtinCount:], func(i, j int) bool {
		return result[builtinCount+i].Name < result[builtinCount+j].Name
	})

	return result, nil
}
