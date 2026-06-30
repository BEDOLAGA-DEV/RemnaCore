package plugin

import "fmt"

// RoutePermissionValidator reports whether a route's required_permission is a
// known RBAC permission key. It is injected (rather than importing the
// identity/rbac domain into this package) so the plugin package stays
// decoupled from the authorization vocabulary.
type RoutePermissionValidator func(permission string) bool

// validateRoutePermissions rejects any manifest route whose non-empty
// required_permission is not a known RBAC permission. An empty value is valid
// (it defaults to plugins.manage at route registration). A nil validator skips
// the check (no validator wired — e.g. in unit tests that don't exercise it).
func validateRoutePermissions(m *Manifest, isKnown RoutePermissionValidator) error {
	if isKnown == nil {
		return nil
	}
	for i, route := range m.Routes {
		if route.RequiredPermission == "" {
			continue
		}
		if !isKnown(route.RequiredPermission) {
			return fmt.Errorf("%w: routes[%d].required_permission %q is not a known RBAC permission",
				ErrInvalidManifest, i, route.RequiredPermission)
		}
	}
	return nil
}
