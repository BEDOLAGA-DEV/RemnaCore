package rbac_test

import (
	"strings"
	"testing"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalog_KeysAreUniqueAndWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range rbac.Catalog() {
		key := string(d.Key)
		require.False(t, seen[key], "duplicate permission key: %s", key)
		seen[key] = true
		require.Contains(t, key, ".", "permission must be resource.action: %s", key)
		assert.Equal(t, strings.SplitN(key, ".", 2)[0], d.Key.Resource())
		assert.Equal(t, strings.SplitN(key, ".", 2)[1], d.Key.Action())
		assert.NotEmpty(t, d.Description)
	}
}

func TestSystemRoles_OnlyReferenceCatalogPermissions(t *testing.T) {
	valid := map[rbac.Permission]bool{}
	for _, d := range rbac.Catalog() {
		valid[d.Key] = true
	}
	keys := map[string]bool{}
	for _, role := range rbac.SystemRoles() {
		require.NotEmpty(t, role.Key)
		require.False(t, keys[role.Key], "duplicate system role: %s", role.Key)
		keys[role.Key] = true
		require.Contains(t, []string{rbac.ScopeGlobal, rbac.ScopeShop}, role.ScopeKind)
		for _, p := range role.Permissions {
			assert.True(t, valid[p], "role %s references unknown permission %s", role.Key, p)
		}
	}
	// platform_admin is allow-all; it must carry NO explicit permission rows.
	for _, role := range rbac.SystemRoles() {
		if role.Key == rbac.RolePlatformAdmin {
			assert.Empty(t, role.Permissions, "platform_admin must be allow-all, no explicit perms")
		}
	}
}

func TestPermScope_ConstantsAreDistinct(t *testing.T) {
	assert.Equal(t, rbac.PermScope("platform"), rbac.PermScopePlatform)
	assert.Equal(t, rbac.PermScope("shop"), rbac.PermScopeShop)
	assert.NotEqual(t, rbac.PermScopePlatform, rbac.PermScopeShop)
}

func TestDashboardRead_IsCatalogued(t *testing.T) {
	assert.Equal(t, rbac.Permission("dashboard.read"), rbac.DashboardRead)
	found := false
	for _, d := range rbac.Catalog() {
		if d.Key == rbac.DashboardRead {
			found = true
			assert.NotEmpty(t, d.Description)
		}
	}
	assert.True(t, found, "dashboard.read must be in Catalog()")
}

func TestCatalog_EveryEntryIsScoped(t *testing.T) {
	shop := map[rbac.Permission]bool{
		rbac.TariffsRead: true, rbac.TariffsWrite: true,
		rbac.CustomersRead: true, rbac.CustomersManage: true,
		rbac.SubscriptionsRead: true, rbac.SubscriptionsManage: true,
		rbac.BillingRead: true, rbac.DashboardRead: true, rbac.PluginsRead: true,
	}
	for _, d := range rbac.Catalog() {
		require.Contains(t, []rbac.PermScope{rbac.PermScopePlatform, rbac.PermScopeShop}, d.Scope,
			"permission %s has no valid scope", d.Key)
		want := rbac.PermScopePlatform
		if shop[d.Key] {
			want = rbac.PermScopeShop
		}
		assert.Equal(t, want, d.Scope, "permission %s has unexpected scope", d.Key)
	}
}

func TestPermissionScope_LooksUpCatalogAndDefaultsToPlatform(t *testing.T) {
	assert.Equal(t, rbac.PermScopeShop, rbac.PermissionScope(rbac.DashboardRead))
	assert.Equal(t, rbac.PermScopeShop, rbac.PermissionScope(rbac.SubscriptionsRead))
	assert.Equal(t, rbac.PermScopePlatform, rbac.PermissionScope(rbac.AnalyticsRead))
	assert.Equal(t, rbac.PermScopePlatform, rbac.PermissionScope(rbac.InfraRead))
	// Fail-safe: an untagged/unknown permission is treated as the more
	// restrictive platform scope.
	assert.Equal(t, rbac.PermScopePlatform, rbac.PermissionScope(rbac.Permission("nonexistent.permission")))
}

func TestRoleShopOwner_IsCleanOfPlatformPerms(t *testing.T) {
	owner, ok := rbac.SystemRoleByKey(rbac.RoleShopOwner)
	require.True(t, ok)

	has := map[rbac.Permission]bool{}
	for _, p := range owner.Permissions {
		has[p] = true
		assert.Equal(t, rbac.PermScopeShop, rbac.PermissionScope(p),
			"shop_owner must not hold platform-scoped permission %s", p)
	}
	// Removed platform perms (tenant management is platform-scoped per DECISION A).
	assert.False(t, has[rbac.AnalyticsRead])
	assert.False(t, has[rbac.UsersInvite])
	assert.False(t, has[rbac.UsersAssignRole])
	assert.False(t, has[rbac.RolesRead])
	assert.False(t, has[rbac.ShopsRead])
	assert.False(t, has[rbac.ShopsManage])
	assert.False(t, has[rbac.ShopsBranding])
	// Added shop-scoped dashboard.
	assert.True(t, has[rbac.DashboardRead])
	// Retained shop perms.
	assert.True(t, has[rbac.TariffsWrite])
	assert.True(t, has[rbac.SubscriptionsManage])
}

func TestIsKnownPermission(t *testing.T) {
	require.True(t, rbac.IsKnownPermission(rbac.TariffsWrite))
	require.True(t, rbac.IsKnownPermission(rbac.UsersAssignRole))
	require.False(t, rbac.IsKnownPermission(rbac.Permission("made.up")))
	require.False(t, rbac.IsKnownPermission(rbac.Permission("")))
}

func TestSystemRoles_ShopScopedRolesHoldNoPlatformPerms(t *testing.T) {
	checked := 0
	for _, role := range rbac.SystemRoles() {
		if role.ScopeKind != rbac.ScopeShop {
			continue
		}
		checked++
		for _, p := range role.Permissions {
			assert.Equal(t, rbac.PermScopeShop, rbac.PermissionScope(p),
				"shop-scoped role %s grants platform-scoped permission %s", role.Key, p)
		}
	}
	// Guard against a vacuous pass if the shop roles were ever removed.
	assert.Greater(t, checked, 0, "expected at least one ScopeShop system role")
}
