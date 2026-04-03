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
}
