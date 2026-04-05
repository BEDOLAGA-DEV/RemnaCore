// Package hookdispatch defines the port interface for dispatching plugin hooks.
// Domain packages depend on this interface instead of the concrete
// plugin.HookDispatcher, keeping the dependency arrow from domain -> pkg
// rather than domain -> internal/plugin.
package hookdispatch

import (
	"context"
	"encoding/json"
)

// Dispatcher is the port for synchronous hook dispatch. It is implemented by
// plugin.HookDispatcher in the plugin runtime layer.
type Dispatcher interface {
	DispatchSync(ctx context.Context, hookName string, payload json.RawMessage) (json.RawMessage, error)
	DispatchSyncVersioned(ctx context.Context, hookName string, currentVersion int, payload json.RawMessage) (json.RawMessage, error)

	// BeginFlow snapshots the current plugin pool versions and returns a
	// context that pins those versions for all subsequent DispatchSync calls.
	// Use this at the start of a multi-hook business flow (e.g., checkout) to
	// guarantee version consistency even if a plugin is hot-reloaded mid-flow.
	// If called on a context that already has flow bindings, it returns the
	// context unchanged.
	BeginFlow(ctx context.Context) context.Context
}
