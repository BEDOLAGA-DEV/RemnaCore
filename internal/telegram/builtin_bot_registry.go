package telegram

import (
	"context"
	"sync"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
)

// BotHandler handles one inbound Telegram update for a shop, producing effects
// only through the bothost.Host op interface (never the raw bot token or a DB
// handle). Built-in Go bot plugins register one of these; WASM bot plugins
// (BP2) go through RuntimePool.CallHook instead.
type BotHandler func(ctx context.Context, update bothost.Update, host bothost.Host) error

type registeredBot struct {
	handler BotHandler
	perms   bothost.PermSet
}

// BuiltinBotRegistry maps a built-in bot-plugin slug to its native Go handler
// and the op-permissions it is granted. It is the bot-dispatch analog of
// gateway.BuiltinRouteRegistry. Safe for concurrent use.
type BuiltinBotRegistry struct {
	mu   sync.RWMutex
	bots map[string]registeredBot
}

// NewBuiltinBotRegistry returns an empty BuiltinBotRegistry.
func NewBuiltinBotRegistry() *BuiltinBotRegistry {
	return &BuiltinBotRegistry{bots: make(map[string]registeredBot)}
}

// Register binds a built-in bot slug to its handler and granted permission set.
// Re-registering a slug replaces it.
func (r *BuiltinBotRegistry) Register(slug string, perms bothost.PermSet, h BotHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bots[slug] = registeredBot{handler: h, perms: perms}
}

// Lookup returns the handler and granted permissions for slug, and whether it
// is registered.
func (r *BuiltinBotRegistry) Lookup(slug string) (BotHandler, bothost.PermSet, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.bots[slug]
	return b.handler, b.perms, ok
}
