// Package hookdispatch defines the port interface for dispatching plugin hooks.
// Domain packages depend on this interface instead of the concrete
// plugin.HookDispatcher, keeping the dependency arrow from domain -> pkg
// rather than domain -> internal/plugin.
package hookdispatch

import (
	"context"
	"encoding/json"
)

// ChainResult holds the outcome of a compensating dispatch chain.
type ChainResult struct {
	// Payload is the final payload after all successful plugins executed.
	// On error, this is the last successfully modified payload before failure.
	Payload json.RawMessage
	// OriginalPayload is the payload as it was before the chain started.
	OriginalPayload json.RawMessage
	// ExecutedPlugins lists the slugs of plugins that executed successfully
	// before the chain stopped (for audit/debugging).
	ExecutedPlugins []string
	// Err is non-nil if the chain was interrupted by a failure or rollback.
	Err error
	// Compensated is true if compensation hooks were called successfully.
	Compensated bool
}

// Dispatcher is the port for synchronous hook dispatch. It is implemented by
// plugin.HookDispatcher in the plugin runtime layer.
type Dispatcher interface {
	DispatchSync(ctx context.Context, hookName string, payload json.RawMessage) (json.RawMessage, error)
	DispatchSyncVersioned(ctx context.Context, hookName string, currentVersion int, payload json.RawMessage) (json.RawMessage, error)

	// DispatchSyncSafe executes synchronous hooks with compensation support.
	// If a plugin in the chain fails, the dispatcher calls
	// "{hookName}.compensate" on each previously executed plugin in reverse
	// priority order, passing the original payload. Returns a ChainResult
	// regardless of success/failure, giving the caller access to both the
	// original and modified payloads.
	DispatchSyncSafe(ctx context.Context, hookName string, payload json.RawMessage) *ChainResult

	// BeginFlow snapshots the current plugin pool versions and returns a
	// context that pins those versions for all subsequent DispatchSync calls.
	// Use this at the start of a multi-hook business flow (e.g., checkout) to
	// guarantee version consistency even if a plugin is hot-reloaded mid-flow.
	// If called on a context that already has flow bindings, it returns the
	// context unchanged.
	BeginFlow(ctx context.Context) context.Context
}
