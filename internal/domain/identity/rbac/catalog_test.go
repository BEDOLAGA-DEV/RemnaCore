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
