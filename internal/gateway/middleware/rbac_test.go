package middleware_test

import (
	"context"
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

type stubRBAC struct{ bindings map[string][]rbac.Binding; perms map[string][]rbac.Permission }

func (s stubRBAC) ListBindingsForUser(_ context.Context, u string) ([]rbac.Binding, error) {
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

func newAccess(b map[string][]rbac.Binding, p map[string][]rbac.Permission) *service.AccessService {
	return service.NewAccessService(stubRBAC{b, p}, time.Now, time.Minute)
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
