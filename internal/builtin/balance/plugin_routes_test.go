package balance

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	rbac "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
)

// TestBalanceRoutes_AdminOnlyHaveRequiredPermission asserts that every
// AdminOnly route produced by balanceRoutes() carries a non-empty
// RequiredPermission. If a new AdminOnly route is added without updating the
// permission loop, this test will catch it.
func TestBalanceRoutes_AdminOnlyHaveRequiredPermission(t *testing.T) {
	routes := balanceRoutes()
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

// TestBalanceRoutes_PinnedPermissions pins the exact RequiredPermission for a
// representative set of balance admin routes, locking the GET→billing.read and
// POST→billing.manage mapping so a future loop change cannot silently downgrade
// permissions.
func TestBalanceRoutes_PinnedPermissions(t *testing.T) {
	routeMap := make(map[string]string) // "METHOD /path" → RequiredPermission
	for _, r := range balanceRoutes() {
		routeMap[r.Method+" "+r.Path] = r.RequiredPermission
	}

	cases := []struct {
		key  string
		want string
	}{
		// GET routes → billing.read
		{"GET /api/balance/admin/wallets", string(rbac.BillingRead)},
		{"GET /api/balance/admin/users/{userID}", string(rbac.BillingRead)},
		{"GET /api/balance/admin/users/{userID}/ledger", string(rbac.BillingRead)},
		{"GET /api/balance/admin/ledger", string(rbac.BillingRead)},
		{"GET /api/balance/admin/topups", string(rbac.BillingRead)},
		{"GET /api/balance/admin/analytics/overview", string(rbac.BillingRead)},

		// POST mutation routes → billing.manage
		{"POST /api/balance/admin/users/{userID}/adjust", string(rbac.BillingManage)},
		{"POST /api/balance/admin/users/{userID}/transfer", string(rbac.BillingManage)},
		{"POST /api/balance/admin/topups/{id}/approve", string(rbac.BillingManage)},
		{"POST /api/balance/admin/topups/{id}/reject", string(rbac.BillingManage)},
		{"POST /api/balance/admin/export", string(rbac.BillingManage)},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.key, func(t *testing.T) {
			got, ok := routeMap[tc.key]
			assert.True(t, ok, "route %q not found in balanceRoutes()", tc.key)
			assert.Equal(t, tc.want, got,
				"permission mismatch for %q", tc.key)
		})
	}
}
