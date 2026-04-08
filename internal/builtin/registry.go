// Package builtin provides a self-registration mechanism for built-in plugins.
// Each plugin package calls Register() in its init() function — the platform
// just imports the package and everything wires automatically.
package builtin

import (
	"sync"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
)

var (
	mu      sync.Mutex
	plugins []plugin.BuiltInPluginDef
	routes  []RouteRegistration
)

// RouteRegistration holds a deferred route registration function.
// It is called during Fx startup with the actual handler instance.
type RouteRegistration struct {
	Slug     string
	Function string
	// HandlerFactory is set by each plugin's init() and resolved during Fx wiring.
	// We store the function name → the wiring layer maps it to the Go handler.
	HandlerKey string
}

// RegisterPlugin adds a built-in plugin definition to the global registry.
// Called from plugin package init() functions.
func RegisterPlugin(def plugin.BuiltInPluginDef) {
	mu.Lock()
	defer mu.Unlock()
	plugins = append(plugins, def)
}

// AllPlugins returns all registered built-in plugin definitions.
func AllPlugins() []plugin.BuiltInPluginDef {
	mu.Lock()
	defer mu.Unlock()
	return append([]plugin.BuiltInPluginDef{}, plugins...)
}
