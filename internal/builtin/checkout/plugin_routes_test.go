package checkout

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	rbac "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
)

// TestCheckoutRoutes_AdminOnlyHaveRequiredPermission asserts that every
// AdminOnly route produced by checkoutRoutes() carries a non-empty
// RequiredPermission. If a new AdminOnly route is added without updating the
// permission loop, this test will catch it.
func TestCheckoutRoutes_AdminOnlyHaveRequiredPermission(t *testing.T) {
	routes := checkoutRoutes()
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

// TestCheckoutRoutes_PinnedPermissions pins the exact RequiredPermission for
// all 5 checkout admin routes. All are GET (read/analytics), so all must carry
// billing.read. This test locks the mapping so a future loop change cannot
// silently downgrade or escalate these permissions.
func TestCheckoutRoutes_PinnedPermissions(t *testing.T) {
	routeMap := make(map[string]string) // "METHOD /path" → RequiredPermission
	for _, r := range checkoutRoutes() {
		routeMap[r.Method+" "+r.Path] = r.RequiredPermission
	}

	cases := []struct {
		key  string
		want string
	}{
		// All 5 admin routes are GET (checkout analytics/session views) → billing.read
		{"GET /api/checkout/admin/sessions", string(rbac.BillingRead)},
		{"GET /api/checkout/admin/sessions/{id}", string(rbac.BillingRead)},
		{"GET /api/checkout/admin/abandonment", string(rbac.BillingRead)},
		{"GET /api/checkout/admin/analytics/funnel", string(rbac.BillingRead)},
		{"GET /api/checkout/admin/analytics/methods", string(rbac.BillingRead)},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.key, func(t *testing.T) {
			got, ok := routeMap[tc.key]
			assert.True(t, ok, "route %q not found in checkoutRoutes()", tc.key)
			assert.Equal(t, tc.want, got,
				"permission mismatch for %q", tc.key)
		})
	}
}
