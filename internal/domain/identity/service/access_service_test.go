package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRBACRepo is a hand-written stub (counts calls to assert caching).
type fakeRBACRepo struct {
	bindings map[string][]rbac.Binding
	perms    map[string][]rbac.Permission
	calls    int
}

func (f *fakeRBACRepo) ListBindingsForUser(_ context.Context, userID string) ([]rbac.Binding, error) {
	f.calls++
	return f.bindings[userID], nil
}
func (f *fakeRBACRepo) PermissionsForRoles(_ context.Context, roleIDs []string) (map[string][]rbac.Permission, error) {
	out := map[string][]rbac.Permission{}
	for _, id := range roleIDs {
		out[id] = f.perms[id]
	}
	return out, nil
}
func (f *fakeRBACRepo) SyncCatalog(context.Context, []rbac.Definition, []rbac.SystemRole) error {
	return nil
}

func clk() func() time.Time { return func() time.Time { return time.Unix(1_700_000_000, 0) } }

func TestResolve_UnionsGlobalAndActiveShopOnly(t *testing.T) {
	shopA := "11111111-1111-1111-1111-111111111111"
	shopB := "22222222-2222-2222-2222-222222222222"
	repo := &fakeRBACRepo{
		bindings: map[string][]rbac.Binding{
			"u1": {
				{RoleID: "owner", RoleKey: rbac.RoleShopOwner, ScopeKind: rbac.ScopeShop, TenantID: &shopA},
				{RoleID: "staff", RoleKey: rbac.RoleShopStaff, ScopeKind: rbac.ScopeShop, TenantID: &shopB},
			},
		},
		perms: map[string][]rbac.Permission{
			"owner": {rbac.TariffsWrite},
			"staff": {rbac.CustomersRead},
		},
	}
	svc := service.NewAccessService(repo, clk(), time.Minute)

	accA, err := svc.Resolve(context.Background(), "u1", &shopA)
	require.NoError(t, err)
	assert.True(t, svc.Can(accA, rbac.TariffsWrite))   // owner perm in shop A
	assert.False(t, svc.Can(accA, rbac.CustomersRead)) // staff perm belongs to shop B
	assert.Contains(t, accA.AllowedTenants, shopA)
	assert.Contains(t, accA.AllowedTenants, shopB)
}

func TestResolve_PlatformAdminIsAllowAll(t *testing.T) {
	repo := &fakeRBACRepo{
		bindings: map[string][]rbac.Binding{
			"admin": {{RoleID: "pa", RoleKey: rbac.RolePlatformAdmin, ScopeKind: rbac.ScopeGlobal}},
		},
	}
	svc := service.NewAccessService(repo, clk(), time.Minute)
	acc, err := svc.Resolve(context.Background(), "admin", nil)
	require.NoError(t, err)
	assert.True(t, acc.IsPlatformAdmin)
	assert.True(t, svc.Can(acc, rbac.SettingsManage)) // any permission
}

func TestResolve_CachesUntilInvalidate(t *testing.T) {
	repo := &fakeRBACRepo{bindings: map[string][]rbac.Binding{"u1": nil}}
	svc := service.NewAccessService(repo, clk(), time.Minute)
	_, _ = svc.Resolve(context.Background(), "u1", nil)
	_, _ = svc.Resolve(context.Background(), "u1", nil)
	assert.Equal(t, 1, repo.calls, "second Resolve must hit the cache")
	svc.Invalidate("u1")
	_, _ = svc.Resolve(context.Background(), "u1", nil)
	assert.Equal(t, 2, repo.calls, "Invalidate forces a reload")
}
