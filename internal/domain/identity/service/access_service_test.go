package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRBACRepo is a hand-written stub (counts calls to assert caching).
// bindingsErr and permsErr are zero-valued (nil) by default so existing tests
// continue to pass without modification.
type fakeRBACRepo struct {
	bindings    map[string][]rbac.Binding
	perms       map[string][]rbac.Permission
	calls       int
	bindingsErr error // if non-nil, ListBindingsForUser returns this error
	permsErr    error // if non-nil, PermissionsForRoles returns this error
}

func (f *fakeRBACRepo) ListBindingsForUser(_ context.Context, userID string) ([]rbac.Binding, error) {
	f.calls++
	if f.bindingsErr != nil {
		return nil, f.bindingsErr
	}
	return f.bindings[userID], nil
}
func (f *fakeRBACRepo) PermissionsForRoles(_ context.Context, roleIDs []string) (map[string][]rbac.Permission, error) {
	if f.permsErr != nil {
		return nil, f.permsErr
	}
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

// TestResolve_FailClosedOnRepoError verifies that when ListBindingsForUser
// returns an error, Resolve propagates it (fail-closed) and does NOT cache
// the empty result — a subsequent call after error recovery must hit the repo
// again rather than serving a cached empty EffectiveAccess.
func TestResolve_FailClosedOnRepoError(t *testing.T) {
	repoErr := errors.New("db unavailable")
	repo := &fakeRBACRepo{
		bindings:    map[string][]rbac.Binding{"u1": nil},
		bindingsErr: repoErr,
	}
	svc := service.NewAccessService(repo, clk(), time.Minute)

	acc, err := svc.Resolve(context.Background(), "u1", nil)

	// Must propagate the error and return an empty (zero-value) EffectiveAccess.
	require.ErrorIs(t, err, repoErr, "error must be propagated to caller")
	assert.False(t, acc.IsPlatformAdmin, "fail-closed: IsPlatformAdmin must be false on error")
	assert.Empty(t, acc.Permissions, "fail-closed: no permissions must be granted on error")
	assert.Equal(t, 1, repo.calls, "repo was called once")

	// Clear the injected error to simulate recovery.
	repo.bindingsErr = nil

	acc2, err2 := svc.Resolve(context.Background(), "u1", nil)
	require.NoError(t, err2)
	assert.Equal(t, 2, repo.calls, "repo must be called again — error result must NOT be cached")
	_ = acc2
}

// TestResolve_ExpiresAfterTTL verifies that a cached entry is re-fetched from
// the repo once the TTL elapses, and that calls within the TTL window are
// served from the cache without additional repo calls.
func TestResolve_ExpiresAfterTTL(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	current := base // mutable clock state
	nowFn := func() time.Time { return current }

	repo := &fakeRBACRepo{bindings: map[string][]rbac.Binding{"u1": nil}}
	ttl := 30 * time.Second
	svc := service.NewAccessService(repo, nowFn, ttl)

	// First call: cache miss → repo called once.
	_, err := svc.Resolve(context.Background(), "u1", nil)
	require.NoError(t, err)
	assert.Equal(t, 1, repo.calls, "first Resolve must hit repo")

	// Second call within TTL: cache hit → still 1 repo call.
	current = base.Add(ttl - time.Second) // just before expiry
	_, err = svc.Resolve(context.Background(), "u1", nil)
	require.NoError(t, err)
	assert.Equal(t, 1, repo.calls, "Resolve within TTL must be served from cache")

	// Advance clock past TTL: cache entry now stale → repo called again.
	current = base.Add(ttl + time.Second) // one second past expiry
	_, err = svc.Resolve(context.Background(), "u1", nil)
	require.NoError(t, err)
	assert.Equal(t, 2, repo.calls, "Resolve after TTL expiry must hit repo again")
}

// TestFlush_ClearsCache verifies that Flush drops all cache entries so the
// next Resolve for any key must go to the repo.
func TestFlush_ClearsCache(t *testing.T) {
	repo := &fakeRBACRepo{bindings: map[string][]rbac.Binding{"u1": nil}}
	svc := service.NewAccessService(repo, clk(), time.Minute)

	// Warm the cache.
	_, err := svc.Resolve(context.Background(), "u1", nil)
	require.NoError(t, err)
	assert.Equal(t, 1, repo.calls, "first Resolve must hit repo")

	// Flush invalidates all entries.
	svc.Flush()

	// Next call must miss the cache and go to repo.
	_, err = svc.Resolve(context.Background(), "u1", nil)
	require.NoError(t, err)
	assert.Equal(t, 2, repo.calls, "Resolve after Flush must hit repo again")
}
