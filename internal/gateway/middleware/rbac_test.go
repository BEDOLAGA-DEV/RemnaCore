package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubRBAC struct {
	bindings    map[string][]rbac.Binding
	perms       map[string][]rbac.Permission
	bindingsErr error // non-nil causes ListBindingsForUser to fail
}

func (s stubRBAC) ListBindingsForUser(_ context.Context, u string) ([]rbac.Binding, error) {
	if s.bindingsErr != nil {
		return nil, s.bindingsErr
	}
	return s.bindings[u], nil
}
func (s stubRBAC) PermissionsForRoles(_ context.Context, ids []string) (map[string][]rbac.Permission, error) {
	out := map[string][]rbac.Permission{}
	for _, id := range ids {
		out[id] = s.perms[id]
	}
	return out, nil
}
func (s stubRBAC) SyncCatalog(context.Context, []rbac.Definition, []rbac.SystemRole) error { return nil }
func (s stubRBAC) AssignRole(_ context.Context, _, _ string, _ *string, _ string) error  { return nil }
func (s stubRBAC) RevokeRole(_ context.Context, _, _ string, _ *string) (int64, error)   { return 0, nil }
func (s stubRBAC) GetRole(_ context.Context, _ string) (rbac.Role, error)                 { return rbac.Role{}, nil }
func (s stubRBAC) CountPlatformAdmins(_ context.Context) (int, error)                    { return 0, nil }

func newAccess(b map[string][]rbac.Binding, p map[string][]rbac.Permission) *service.AccessService {
	return service.NewAccessService(stubRBAC{bindings: b, perms: p}, time.Now, time.Minute)
}

func newAccessWithErr(err error) *service.AccessService {
	return service.NewAccessService(stubRBAC{bindingsErr: err}, time.Now, time.Minute)
}

func withClaims(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.ClaimsContextKey, &authutil.UserClaims{UserID: userID, Email: "x@y.z"})
	return r.WithContext(ctx)
}

func TestRequirePermission_403WithoutPermission(t *testing.T) {
	access := newAccess(map[string][]rbac.Binding{"u1": nil}, nil)
	h := middleware.RequirePermission(access, rbac.SettingsManage)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withClaims(httptest.NewRequest(http.MethodGet, "/x", nil), "u1"))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequirePermission_401WithoutClaims(t *testing.T) {
	access := newAccess(nil, nil)
	h := middleware.RequirePermission(access, rbac.SettingsManage)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestShopResolver_RejectsUnboundShop(t *testing.T) {
	shopA := "11111111-1111-1111-1111-111111111111"
	access := newAccess(map[string][]rbac.Binding{"u1": nil}, nil) // no bindings -> no tenants
	var reached bool
	h := middleware.ShopResolver(access)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { reached = true }))
	req := withClaims(httptest.NewRequest(http.MethodGet, "/x", nil), "u1")
	req.Header.Set(httpconst.HeaderShopID, shopA)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.False(t, reached, "must not reach handler for an unbound shop")
}

func TestShopResolver_AllowsBoundShopAndSetsTenant(t *testing.T) {
	shopA := "11111111-1111-1111-1111-111111111111"
	access := newAccess(
		map[string][]rbac.Binding{"u1": {{RoleID: "owner", RoleKey: rbac.RoleShopOwner, ScopeKind: rbac.ScopeShop, TenantID: &shopA}}},
		map[string][]rbac.Permission{"owner": {rbac.TariffsWrite}},
	)
	var gotTenant string
	h := middleware.ShopResolver(access)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotTenant = middleware.ActiveTenantID(r.Context())
	}))
	req := withClaims(httptest.NewRequest(http.MethodGet, "/x", nil), "u1")
	req.Header.Set(httpconst.HeaderShopID, shopA)
	h.ServeHTTP(httptest.NewRecorder(), req)
	require.Equal(t, shopA, gotTenant)
}

// TestShopResolver_PlatformAdminEntersAnyShop verifies the IsPlatformAdmin bypass:
// a user with only a global platform_admin binding (so AllowedTenants is empty)
// can still enter any shop and ActiveTenantID is set to the requested shop ID.
func TestShopResolver_PlatformAdminEntersAnyShop(t *testing.T) {
	arbitraryShop := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	adminRoleID := "admin-role-id"
	access := newAccess(
		map[string][]rbac.Binding{
			"admin1": {
				{
					RoleID:    adminRoleID,
					RoleKey:   rbac.RolePlatformAdmin,
					ScopeKind: rbac.ScopeGlobal,
					TenantID:  nil, // global binding — AllowedTenants will be empty
				},
			},
		},
		map[string][]rbac.Permission{adminRoleID: nil}, // platform_admin carries no explicit perms
	)
	var (
		reached   bool
		gotTenant string
	)
	h := middleware.ShopResolver(access)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		reached = true
		gotTenant = middleware.ActiveTenantID(r.Context())
	}))
	req := withClaims(httptest.NewRequest(http.MethodGet, "/x", nil), "admin1")
	req.Header.Set(httpconst.HeaderShopID, arbitraryShop)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.True(t, reached, "platform admin must reach the handler even without a shop membership")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, arbitraryShop, gotTenant, "active tenant must be set to the requested shop ID")
}

// TestRequirePermission_AllowsWithPermission verifies the happy path: a user
// whose global role grants the required permission gets a 200 and the handler runs.
func TestRequirePermission_AllowsWithPermission(t *testing.T) {
	roleID := "settings-role"
	access := newAccess(
		map[string][]rbac.Binding{
			"u2": {
				{
					RoleID:    roleID,
					RoleKey:   "", // custom or system role; key is irrelevant here
					ScopeKind: rbac.ScopeGlobal,
					TenantID:  nil,
				},
			},
		},
		map[string][]rbac.Permission{roleID: {rbac.SettingsManage}},
	)
	var reached bool
	h := middleware.RequirePermission(access, rbac.SettingsManage)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withClaims(httptest.NewRequest(http.MethodGet, "/x", nil), "u2"))
	require.True(t, reached, "handler must be reached when permission is granted")
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestRequirePermission_500OnResolveError verifies fail-closed behaviour:
// a repo error during resolution must produce 500 and must NOT invoke the handler.
func TestRequirePermission_500OnResolveError(t *testing.T) {
	access := newAccessWithErr(errors.New("db unavailable"))
	var reached bool
	h := middleware.RequirePermission(access, rbac.SettingsManage)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withClaims(httptest.NewRequest(http.MethodGet, "/x", nil), "u1"))
	assert.Equal(t, http.StatusInternalServerError, rec.Code, "must be fail-closed on resolution error")
	assert.False(t, reached, "handler must NOT be reached when resolution fails")
}

// TestShopResolver_500OnResolveError verifies that ShopResolver is also fail-closed:
// a repo error when X-Shop-Id is present must yield 500 and must NOT set the tenant
// or invoke the handler.
func TestShopResolver_500OnResolveError(t *testing.T) {
	shopA := "11111111-1111-1111-1111-111111111111"
	access := newAccessWithErr(errors.New("db unavailable"))
	var reached bool
	h := middleware.ShopResolver(access)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
	}))
	req := withClaims(httptest.NewRequest(http.MethodGet, "/x", nil), "u1")
	req.Header.Set(httpconst.HeaderShopID, shopA)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusInternalServerError, rec.Code, "must be fail-closed on resolution error")
	assert.False(t, reached, "handler must NOT be reached when resolution fails")
}
