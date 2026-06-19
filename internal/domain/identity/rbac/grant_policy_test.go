package rbac_test

import (
	"testing"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ownerActor(shop string) rbac.Actor {
	// shop_owner in `shop`: holds shop_owner's permission set, member of `shop`.
	perms := map[rbac.Permission]struct{}{}
	for _, sr := range rbac.SystemRoles() {
		if sr.Key == rbac.RoleShopOwner {
			for _, p := range sr.Permissions {
				perms[p] = struct{}{}
			}
		}
	}
	return rbac.Actor{Permissions: perms, AllowedTenants: map[string]struct{}{shop: {}}}
}

func staffTarget() rbac.GrantTarget {
	var perms []rbac.Permission
	for _, sr := range rbac.SystemRoles() {
		if sr.Key == rbac.RoleShopStaff {
			perms = sr.Permissions
		}
	}
	return rbac.GrantTarget{RoleKey: rbac.RoleShopStaff, ScopeKind: rbac.ScopeShop, Permissions: perms}
}

func TestCanGrant(t *testing.T) {
	shopA := "11111111-1111-1111-1111-111111111111"
	shopB := "22222222-2222-2222-2222-222222222222"

	t.Run("platform admin may grant anything anywhere", func(t *testing.T) {
		admin := rbac.Actor{IsPlatformAdmin: true}
		require.NoError(t, rbac.CanGrant(admin, rbac.GrantTarget{RoleKey: rbac.RolePlatformAdmin, ScopeKind: rbac.ScopeGlobal}, nil))
		require.NoError(t, rbac.CanGrant(admin, staffTarget(), &shopA))
	})
	t.Run("shop owner cannot grant staff (no users.assign_role)", func(t *testing.T) {
		// As of C1.5 shop_owner is pruned of platform perms, including
		// UsersAssignRole, so a shop principal never carries grant capability
		// through its shop binding. CanGrant rejects at the Rule 2 check.
		assert.ErrorIs(t, rbac.CanGrant(ownerActor(shopA), staffTarget(), &shopA), rbac.ErrGrantNotAllowed)
	})
	t.Run("shop owner cannot grant in a shop it is not a member of", func(t *testing.T) {
		assert.ErrorIs(t, rbac.CanGrant(ownerActor(shopA), staffTarget(), &shopB), rbac.ErrGrantNotAllowed)
	})
	t.Run("shop owner cannot grant a global role", func(t *testing.T) {
		assert.ErrorIs(t, rbac.CanGrant(ownerActor(shopA), rbac.GrantTarget{RoleKey: "x", ScopeKind: rbac.ScopeGlobal}, nil), rbac.ErrGrantNotAllowed)
	})
	t.Run("shop owner cannot create another owner", func(t *testing.T) {
		owner := rbac.GrantTarget{RoleKey: rbac.RoleShopOwner, ScopeKind: rbac.ScopeShop}
		assert.ErrorIs(t, rbac.CanGrant(ownerActor(shopA), owner, &shopA), rbac.ErrGrantNotAllowed)
	})
	t.Run("no escalation: cannot grant a permission the actor lacks", func(t *testing.T) {
		// target carries SettingsManage, which shop_owner does not hold.
		tgt := rbac.GrantTarget{RoleKey: "custom", ScopeKind: rbac.ScopeShop, Permissions: []rbac.Permission{rbac.SettingsManage}}
		assert.ErrorIs(t, rbac.CanGrant(ownerActor(shopA), tgt, &shopA), rbac.ErrGrantNotAllowed)
	})
	t.Run("non-admin without users.assign_role is rejected", func(t *testing.T) {
		// Actor is a member of shopA but holds only CustomersRead — no UsersAssignRole.
		actor := rbac.Actor{
			Permissions:    map[rbac.Permission]struct{}{rbac.CustomersRead: {}},
			AllowedTenants: map[string]struct{}{shopA: {}},
		}
		assert.ErrorIs(t, rbac.CanGrant(actor, staffTarget(), &shopA), rbac.ErrGrantNotAllowed)
	})
	t.Run("shop owner cannot grant platform_admin role", func(t *testing.T) {
		tgt := rbac.GrantTarget{RoleKey: rbac.RolePlatformAdmin, ScopeKind: rbac.ScopeShop}
		assert.ErrorIs(t, rbac.CanGrant(ownerActor(shopA), tgt, &shopA), rbac.ErrGrantNotAllowed)
	})
	t.Run("nil scope tenant with shop role is rejected", func(t *testing.T) {
		tgt := rbac.GrantTarget{RoleKey: rbac.RoleShopStaff, ScopeKind: rbac.ScopeShop}
		assert.ErrorIs(t, rbac.CanGrant(ownerActor(shopA), tgt, nil), rbac.ErrGrantNotAllowed)
	})
}
