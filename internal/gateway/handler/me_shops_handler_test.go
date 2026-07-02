package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
)

// --- stubs ---

type stubUserBindingLister struct {
	bindings []rbac.Binding
	err      error
}

func (s *stubUserBindingLister) ListBindingsForUser(_ context.Context, _ string) ([]rbac.Binding, error) {
	return s.bindings, s.err
}

type stubShopNameGetter struct {
	names  map[string]string
	errFor map[string]error
}

func (s *stubShopNameGetter) GetTenantName(_ context.Context, id string) (string, error) {
	if err, ok := s.errFor[id]; ok {
		return "", err
	}
	return s.names[id], nil
}

// withShopClaims stamps a userID into r's context as JWT claims, mirroring the
// middleware.Auth behaviour so MeShopsHandler.MyShops can read the caller's ID.
func withShopClaims(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.ClaimsContextKey, &authutil.UserClaims{
		UserID: userID,
	})
	return r.WithContext(ctx)
}

// newMeShopsHandlerForTest builds a MeShopsHandler wired to the given stubs.
func newMeShopsHandlerForTest(bl *stubUserBindingLister, sg *stubShopNameGetter) *MeShopsHandler {
	return NewMeShopsHandler(bl, sg, slog.Default())
}

// --- tests ---

// TestMeShopsHandler_TwoShops_NameSorted verifies that two roles in the same
// shop produce one entry (dedup), a global binding is ignored, and the result
// is sorted by tenant name.
func TestMeShopsHandler_TwoShops_NameSorted(t *testing.T) {
	shopA := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	shopB := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	tenantA, tenantB := shopA, shopB

	bl := &stubUserBindingLister{
		bindings: []rbac.Binding{
			{RoleID: "r1", ScopeKind: rbac.ScopeShop, TenantID: &tenantA},
			{RoleID: "r2", ScopeKind: rbac.ScopeShop, TenantID: &tenantA}, // duplicate shop — must dedup
			{RoleID: "r3", ScopeKind: rbac.ScopeGlobal, TenantID: nil},    // global binding — must skip
			{RoleID: "r4", ScopeKind: rbac.ScopeShop, TenantID: &tenantB},
		},
	}
	sg := &stubShopNameGetter{
		names: map[string]string{
			shopA: "Zebra Shop", // comes last alphabetically
			shopB: "Alpha Shop", // comes first alphabetically
		},
	}

	h := newMeShopsHandlerForTest(bl, sg)
	rec := httptest.NewRecorder()
	req := withShopClaims(httptest.NewRequest(http.MethodGet, "/api/me/shops", nil), "user-1")
	h.MyShops(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var got []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 2, "expected exactly two distinct shops")

	// sorted by name: "Alpha Shop" < "Zebra Shop"
	assert.Equal(t, shopB, got[0]["id"])
	assert.Equal(t, "Alpha Shop", got[0]["name"])
	assert.Equal(t, shopA, got[1]["id"])
	assert.Equal(t, "Zebra Shop", got[1]["name"])
}

// TestMeShopsHandler_NoBindings_EmptyArray asserts that a user with no
// bindings receives a literal JSON empty array (not null) with status 200.
func TestMeShopsHandler_NoBindings_EmptyArray(t *testing.T) {
	h := newMeShopsHandlerForTest(
		&stubUserBindingLister{bindings: nil},
		&stubShopNameGetter{names: map[string]string{}},
	)
	rec := httptest.NewRecorder()
	req := withShopClaims(httptest.NewRequest(http.MethodGet, "/api/me/shops", nil), "user-no-shops")
	h.MyShops(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	// Must serialize as [] not null.
	var got []any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.NotNil(t, got, "response must be [] not null")
	assert.Len(t, got, 0)
}

// TestMeShopsHandler_UnresolvableTenantSkipped verifies that a tenant whose
// name lookup fails is silently omitted from the response (the rest still appear).
func TestMeShopsHandler_UnresolvableTenantSkipped(t *testing.T) {
	shopA := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	shopB := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	tenantA, tenantB := shopA, shopB

	bl := &stubUserBindingLister{
		bindings: []rbac.Binding{
			{RoleID: "r1", ScopeKind: rbac.ScopeShop, TenantID: &tenantA},
			{RoleID: "r2", ScopeKind: rbac.ScopeShop, TenantID: &tenantB},
		},
	}
	sg := &stubShopNameGetter{
		names:  map[string]string{shopA: "Active Shop"},
		errFor: map[string]error{shopB: errors.New("tenant deleted")},
	}

	h := newMeShopsHandlerForTest(bl, sg)
	rec := httptest.NewRecorder()
	req := withShopClaims(httptest.NewRequest(http.MethodGet, "/api/me/shops", nil), "user-1")
	h.MyShops(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var got []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 1, "unresolvable tenant must be skipped")
	assert.Equal(t, shopA, got[0]["id"])
	assert.Equal(t, "Active Shop", got[0]["name"])
}
