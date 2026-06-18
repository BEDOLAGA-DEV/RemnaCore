package gateway_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
)

// scopedRoute pins a core admin route to the permission it is gated on and the
// PermScope that permission is expected to carry. It guards against gating a
// platform aggregate on a shop-scoped permission (or vice-versa).
type scopedRoute struct {
	method    string
	path      string
	perm      rbac.Permission
	wantScope rbac.PermScope
}

// TestCoreAdminRoutes_ScopeConsistency asserts that every core /admin route's
// gating permission carries the PermScope the route intends. Keep in sync with
// router.go's /admin sub-router.
func TestCoreAdminRoutes_ScopeConsistency(t *testing.T) {
	routes := []scopedRoute{
		// Plugin management.
		{http.MethodGet, "/api/admin/plugins", rbac.PluginsManage, rbac.PermScopePlatform},
		{http.MethodPost, "/api/admin/plugins", rbac.PluginsManage, rbac.PermScopePlatform},
		{http.MethodGet, "/api/admin/plugin-pages", rbac.PluginsRead, rbac.PermScopeShop},
		// User management — platform.
		{http.MethodGet, "/api/admin/users", rbac.UsersRead, rbac.PermScopePlatform},
		{http.MethodGet, "/api/admin/users/{userID}", rbac.UsersRead, rbac.PermScopePlatform},
		// Platform aggregate lists — re-gated to AnalyticsRead (platform) in C1.
		{http.MethodGet, "/api/admin/subscriptions", rbac.AnalyticsRead, rbac.PermScopePlatform},
		{http.MethodGet, "/api/admin/invoices", rbac.AnalyticsRead, rbac.PermScopePlatform},
		// Settings + observability — platform.
		{http.MethodGet, "/api/admin/settings", rbac.SettingsManage, rbac.PermScopePlatform},
		{http.MethodPut, "/api/admin/settings", rbac.SettingsManage, rbac.PermScopePlatform},
		{http.MethodGet, "/api/admin/stats", rbac.AnalyticsRead, rbac.PermScopePlatform},
		{http.MethodGet, "/api/admin/metrics", rbac.AnalyticsRead, rbac.PermScopePlatform},
		{http.MethodGet, "/api/admin/metrics/history", rbac.AnalyticsRead, rbac.PermScopePlatform},
		{http.MethodGet, "/api/admin/sessions", rbac.SessionsRead, rbac.PermScopePlatform},
		{http.MethodGet, "/api/admin/activity", rbac.SessionsRead, rbac.PermScopePlatform},
		// Shop / tenant management — platform-scoped (tenant management is a
		// platform capability per spec §6 / DECISION A).
		{http.MethodPost, "/api/admin/shops", rbac.ShopsManage, rbac.PermScopePlatform},
		{http.MethodPost, "/api/admin/tenants", rbac.ShopsManage, rbac.PermScopePlatform},
		{http.MethodGet, "/api/admin/tenants", rbac.ShopsRead, rbac.PermScopePlatform},
		{http.MethodGet, "/api/admin/tenants/{tenantID}", rbac.ShopsRead, rbac.PermScopePlatform},
		{http.MethodPut, "/api/admin/tenants/{tenantID}/branding", rbac.ShopsBranding, rbac.PermScopePlatform},
	}

	// Guard against a vacuous pass if the table is ever emptied.
	assert.Greater(t, len(routes), 0, "scope-consistency route table must not be empty")

	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			assert.Equal(t, rt.wantScope, rbac.PermissionScope(rt.perm),
				"%s %s is gated on %s but its catalog scope is not %s",
				rt.method, rt.path, rt.perm, rt.wantScope)
		})
	}
}
