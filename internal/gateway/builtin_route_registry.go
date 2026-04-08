package gateway

import (
	"net/http"
	"sync"
)

// builtinRouteKey uniquely identifies a built-in plugin route.
type builtinRouteKey struct {
	Slug     string
	Function string
}

// BuiltinRouteRegistry maps built-in plugin RPC function names to native Go
// HTTP handlers. When RegisterPluginRoutes encounters a built-in plugin, it
// looks up the handler here instead of proxying to WASM.
type BuiltinRouteRegistry struct {
	mu       sync.RWMutex
	handlers map[builtinRouteKey]http.HandlerFunc
}

// NewBuiltinRouteRegistry creates an empty registry.
func NewBuiltinRouteRegistry() *BuiltinRouteRegistry {
	return &BuiltinRouteRegistry{
		handlers: make(map[builtinRouteKey]http.HandlerFunc),
	}
}

// Register maps a plugin slug + function name to a native handler.
func (r *BuiltinRouteRegistry) Register(slug, function string, handler http.HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[builtinRouteKey{Slug: slug, Function: function}] = handler
}

// Lookup returns the native handler for a built-in plugin function, or nil.
func (r *BuiltinRouteRegistry) Lookup(slug, function string) http.HandlerFunc {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.handlers[builtinRouteKey{Slug: slug, Function: function}]
}
