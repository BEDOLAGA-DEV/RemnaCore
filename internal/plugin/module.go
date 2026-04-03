package plugin

import "go.uber.org/fx"

// Module provides the plugin domain components to the Fx dependency graph.
var Module = fx.Module("plugin",
	fx.Provide(NewRuntimePool),
	fx.Provide(NewHookDispatcher),
	fx.Provide(NewPermissionChecker),
	fx.Provide(NewLifecycleManager),
	fx.Provide(NewHostFunctions),
)

// NewPermissionChecker creates a stateless PermissionChecker.
func NewPermissionChecker() *PermissionChecker {
	return &PermissionChecker{}
}
