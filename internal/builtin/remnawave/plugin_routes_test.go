package remnawave

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	rbac "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
)

// TestRemnawaveRoutes_AdminOnlyHaveRequiredPermission asserts that every
// AdminOnly route produced by remnawaveRoutes() carries a non-empty
// RequiredPermission. This guards against new AdminOnly routes being added
// without a permission assignment (spec §11 gap).
func TestRemnawaveRoutes_AdminOnlyHaveRequiredPermission(t *testing.T) {
	routes := remnawaveRoutes()
	for _, r := range routes {
		if !r.AdminOnly {
			continue
		}
		t.Run(fmt.Sprintf("%s %s", r.Method, r.Path), func(t *testing.T) {
			assert.NotEmpty(t, r.RequiredPermission,
				"AdminOnly route must have a RequiredPermission")
		})
	}
}

// TestRemnawaveRoutes_InfraDomainPermissions pins representative routes to their
// exact RBAC gate. GET topology routes must map to infra.read; mutating routes
// must map to infra.manage.
func TestRemnawaveRoutes_InfraDomainPermissions(t *testing.T) {
	// Build a (method, path) → RequiredPermission lookup for exact matching.
	type key struct{ method, path string }
	routeMap := make(map[key]string)
	for _, r := range remnawaveRoutes() {
		routeMap[key{r.Method, r.Path}] = r.RequiredPermission
	}

	cases := []struct {
		method string
		path   string
		want   string
	}{
		// --- infra.read gate (GET topology / analytics) ---
		{"GET", "/api/remnawave/nodes", string(rbac.InfraRead)},
		{"GET", "/api/remnawave/panels", string(rbac.InfraRead)},
		{"GET", "/api/remnawave/squads/internal", string(rbac.InfraRead)},
		{"GET", "/api/remnawave/squads/external", string(rbac.InfraRead)},
		{"GET", "/api/remnawave/dashboard/overview", string(rbac.InfraRead)},
		{"GET", "/api/remnawave/analytics/traffic-by-node", string(rbac.InfraRead)},

		// --- infra.manage gate (mutating routes) ---
		{"POST", "/api/remnawave/panels", string(rbac.InfraManage)},
		{"PUT", "/api/remnawave/panels/{panelID}", string(rbac.InfraManage)},
		{"DELETE", "/api/remnawave/panels/{panelID}", string(rbac.InfraManage)},
		{"POST", "/api/remnawave/nodes/{panelID}", string(rbac.InfraManage)},
		{"DELETE", "/api/remnawave/nodes/{panelID}/{nodeUUID}", string(rbac.InfraManage)},
		{"POST", "/api/remnawave/drift/reconcile", string(rbac.InfraManage)},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("%s %s", tc.method, tc.path), func(t *testing.T) {
			got, ok := routeMap[key{tc.method, tc.path}]
			assert.True(t, ok, "route %s %q not found in remnawaveRoutes()", tc.method, tc.path)
			assert.Equal(t, tc.want, got,
				"infra-domain permission mismatch for %s %q", tc.method, tc.path)
		})
	}
}

// TestRemnawaveRoutes_PublicRoutesHaveNoPermission asserts that public routes
// do not carry a RequiredPermission, keeping the contract clean for the route
// registrar (currently all remnawave routes are AdminOnly, but this guards
// future public additions).
func TestRemnawaveRoutes_PublicRoutesHaveNoPermission(t *testing.T) {
	routes := remnawaveRoutes()
	for _, r := range routes {
		if !r.Public {
			continue
		}
		t.Run(fmt.Sprintf("%s %s", r.Method, r.Path), func(t *testing.T) {
			assert.Empty(t, r.RequiredPermission,
				"Public route must not have a RequiredPermission")
		})
	}
}
