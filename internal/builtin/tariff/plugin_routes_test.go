package tariff

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	rbac "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
)

// TestTariffRoutes_AdminOnlyHaveRequiredPermission asserts that every
// AdminOnly route produced by tariffRoutes() carries a non-empty
// RequiredPermission. This is a compile-time-equivalent guard: if a new
// AdminOnly route is added without updating the permission switch, this test
// will catch it.
func TestTariffRoutes_AdminOnlyHaveRequiredPermission(t *testing.T) {
	routes := tariffRoutes()
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

// TestTariffRoutes_SecurityCarveOutPermissions pins the EXACT RequiredPermission
// for the security-critical carve-out routes. These routes are split across two
// RBAC gates:
//
//   - infra.read  — platform-topology and analytics routes that expose
//     Remnawave panel/squad/node data; shop_owner must NOT see these.
//   - tariffs.write — reseller routes that expose or modify reseller-specific
//     pricing config; only reseller-enabled admins may access them.
//
// This test locks the mapping so that a future switch-case reorder cannot
// silently change which permission guards these routes.
func TestTariffRoutes_SecurityCarveOutPermissions(t *testing.T) {
	// Build a path → route map for fast lookup.
	routeByPath := make(map[string]string) // path → RequiredPermission
	for _, r := range tariffRoutes() {
		routeByPath[r.Path] = r.RequiredPermission
	}

	cases := []struct {
		path string
		want string
	}{
		// --- infra.read gate (platform topology + analytics) ---
		{"/api/tariffs/panels", string(rbac.InfraRead)},
		{"/api/tariffs/internal-squads", string(rbac.InfraRead)},
		{"/api/tariffs/external-squads", string(rbac.InfraRead)},
		{"/api/tariffs/nodes", string(rbac.InfraRead)},
		{"/api/tariffs/analytics/mrr-by-tariff", string(rbac.InfraRead)},
		{"/api/tariffs/analytics/conversion-funnel", string(rbac.InfraRead)},

		// --- tariffs.write gate (reseller config) ---
		{"/api/tariffs/reseller/catalog", string(rbac.TariffsWrite)},
		{"/api/tariffs/reseller/customize", string(rbac.TariffsWrite)},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			got, ok := routeByPath[tc.path]
			assert.True(t, ok, "route %q not found in tariffRoutes()", tc.path)
			assert.Equal(t, tc.want, got,
				"security carve-out permission mismatch for %q", tc.path)
		})
	}
}

// TestTariffRoutes_PublicRoutesHaveNoPermission asserts that public routes
// (no auth required) do not carry a RequiredPermission, keeping the contract
// clean for the route registrar.
func TestTariffRoutes_PublicRoutesHaveNoPermission(t *testing.T) {
	routes := tariffRoutes()
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
