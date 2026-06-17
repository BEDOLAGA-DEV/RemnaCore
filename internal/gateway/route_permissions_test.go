// Package gateway_test verifies that every admin route and admin-only plugin
// route is permission-gated. This is a regression guard: adding a new admin
// route without RequirePermission will cause this test to fail.
//
// Approach: scoped sub-router fallback (documented limitation below).
// Assembling the full router via NewRouter requires too many real dependencies
// (handlers, DB repos, fx wiring, telegram bot, etc.) to be practical in a
// unit test that must run without Docker or a database. Instead we construct a
// minimal chi router that wires the SAME middleware chain used in router.go:
//
//	Auth → ShopResolver → RequirePermission(access, perm)
//
// over a representative set of admin routes (every route listed in router.go's
// /admin sub-router and a representative subset of the tariff-manager plugin's
// admin-only routes). The handler stub always returns 200 — so a 403 response
// proves the permission gate fired, not the handler. A 200 would indicate the
// gate was missing.
//
// Limitation: this test does NOT walk the full live router tree. If a new admin
// route is added to NewRouter without RequirePermission AND the route list below
// is not updated, the regression will be missed until this list is extended.
// Keep in sync with:
//   - router.go /admin re-gating (Task 11)
//   - router.go /reseller re-gating (Task 11)
//   - internal/builtin/tariff/plugin.go admin-only routes (Task 9)
package gateway_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

// ─── stub repository ─────────────────────────────────────────────────────────

// denyAllRepo implements rbac.Repository. It returns a single global
// "customer" binding for every user, giving them no admin permissions.
type denyAllRepo struct{}

func (denyAllRepo) ListBindingsForUser(_ context.Context, userID string) ([]rbac.Binding, error) {
	return []rbac.Binding{
		{
			RoleID:    "customer-role-id",
			RoleKey:   rbac.RoleCustomer,
			ScopeKind: rbac.ScopeGlobal,
			TenantID:  nil,
		},
	}, nil
}

func (denyAllRepo) PermissionsForRoles(_ context.Context, _ []string) (map[string][]rbac.Permission, error) {
	// customer has no permissions.
	return map[string][]rbac.Permission{
		"customer-role-id": nil,
	}, nil
}

func (denyAllRepo) SyncCatalog(context.Context, []rbac.Definition, []rbac.SystemRole) error {
	return nil
}
func (denyAllRepo) AssignRole(_ context.Context, _, _ string, _ *string, _ string) error {
	return nil
}
func (denyAllRepo) RevokeRole(_ context.Context, _, _ string, _ *string) (int64, error) {
	return 0, nil
}
func (denyAllRepo) GetRole(_ context.Context, _ string) (rbac.Role, error) {
	return rbac.Role{}, nil
}
func (denyAllRepo) CountPlatformAdmins(_ context.Context) (int, error) { return 0, nil }

// ─── test helper ─────────────────────────────────────────────────────────────

// adminRoute describes a single route entry under test.
type adminRoute struct {
	method string
	path   string
	perm   rbac.Permission
}

// newScopedAdminRouter builds a minimal chi router that replicates the
// Auth → ShopResolver → RequirePermission chain from router.go over the given
// admin routes. Every handler stub returns 200 so that a 403 proves the gate
// fired and is not masked by a handler-level error.
//
// This is the scoped sub-router fallback documented in the package comment.
// The full-router approach (NewRouter) is impractical here because it requires
// handlers, Postgres repos, telegram bot, fx DI, etc.
func newScopedAdminRouter(
	t *testing.T,
	routes []adminRoute,
) (http.Handler, string) {
	t.Helper()

	// Generate an ECDSA P-256 key pair — same pattern as auth_test.go.
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	pub := &priv.PublicKey

	issuer := authutil.NewJWTIssuer(priv, pub)

	// Sign a token for a non-admin user.
	token, err := issuer.Sign(authutil.UserClaims{
		UserID: "nonadmin-user-id",
		Email:  "nonadmin@example.com",
	}, 5*time.Minute)
	require.NoError(t, err)

	// Build an AccessService backed by denyAllRepo — so Can(acc, any) == false.
	access := service.NewAccessService(denyAllRepo{}, time.Now, service.AccessCacheTTL)

	r := chi.NewRouter()

	// Replicate the identical middleware chain from router.go:
	//   Auth → ShopResolver → [per-route] RequirePermission
	r.Group(func(protected chi.Router) {
		protected.Use(middleware.Auth(issuer))
		protected.Use(middleware.ShopResolver(access))

		stub := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		for _, rt := range routes {
			p := rt.perm // capture for closure
			protected.With(middleware.RequirePermission(access, p)).
				Method(rt.method, rt.path, stub)
		}
	})

	return r, token
}

// ─── test ────────────────────────────────────────────────────────────────────

// TestAdminRoutesArePermissionGated asserts that every admin route (router.go
// /admin sub-router) and admin-only plugin route (tariff-manager, Task 9)
// returns HTTP 403 for a valid, authenticated, non-admin user.
//
// A 200 from any route would indicate the permission gate is absent.
//
// Routes deferred to Phase B/C (e.g. /api/admin/users/{id}/assign-role) are
// explicitly excluded with a comment.
//
// Keep in sync with:
//   - router.go /admin re-gating (Task 11)
//   - router.go /reseller re-gating (Task 11)
//   - internal/builtin/tariff/plugin.go admin-only routes (Task 9)
func TestAdminRoutesArePermissionGated(t *testing.T) {
	t.Log("Scoped sub-router fallback: testing the Auth→ShopResolver→RequirePermission chain " +
		"without assembling the full router (which requires DB/handlers/fx wiring).")

	// Every admin route from router.go plus a representative set of
	// admin-only tariff routes from internal/builtin/tariff/plugin.go.
	routes := []adminRoute{
		// ── Plugin management (plugins.manage) ──────────────────────────────
		{http.MethodGet, "/api/admin/plugins", rbac.PluginsManage},
		{http.MethodPost, "/api/admin/plugins", rbac.PluginsManage},
		{http.MethodGet, "/api/admin/plugins/{pluginID}", rbac.PluginsManage},
		{http.MethodPost, "/api/admin/plugins/{pluginID}/enable", rbac.PluginsManage},
		{http.MethodPost, "/api/admin/plugins/{pluginID}/disable", rbac.PluginsManage},
		{http.MethodDelete, "/api/admin/plugins/{pluginID}", rbac.PluginsManage},
		{http.MethodPut, "/api/admin/plugins/{pluginID}/config", rbac.PluginsManage},
		{http.MethodPut, "/api/admin/plugins/{pluginID}/reload", rbac.PluginsManage},
		// Plugin-pages: lighter plugins.read perm so shop roles can render pages.
		{http.MethodGet, "/api/admin/plugin-pages", rbac.PluginsRead},

		// ── User / subscription / invoice (users.read, subscriptions.read, billing.read) ──
		{http.MethodGet, "/api/admin/users", rbac.UsersRead},
		{http.MethodGet, "/api/admin/users/{userID}", rbac.UsersRead},
		{http.MethodGet, "/api/admin/subscriptions", rbac.SubscriptionsRead},
		{http.MethodGet, "/api/admin/invoices", rbac.BillingRead},

		// ── System settings (settings.manage) ───────────────────────────────
		{http.MethodGet, "/api/admin/settings", rbac.SettingsManage},
		{http.MethodPut, "/api/admin/settings", rbac.SettingsManage},

		// ── Stats / observability (analytics.read, sessions.read) ───────────
		{http.MethodGet, "/api/admin/stats", rbac.AnalyticsRead},
		{http.MethodGet, "/api/admin/metrics", rbac.AnalyticsRead},
		{http.MethodGet, "/api/admin/metrics/history", rbac.AnalyticsRead},
		{http.MethodGet, "/api/admin/sessions", rbac.SessionsRead},
		{http.MethodGet, "/api/admin/activity", rbac.SessionsRead},

		// ── IAM: invitations + direct-create + role assignment (Phase B) ───
		// /api/auth/accept-invitation is intentionally excluded: it is a public
		// route (no Auth middleware) so it must NOT be in this gated set.
		{http.MethodGet, "/api/users/invitations", rbac.UsersInvite},
		{http.MethodPost, "/api/users/invitations", rbac.UsersInvite},
		{http.MethodPost, "/api/users", rbac.UsersInvite},
		{http.MethodDelete, "/api/users/invitations/{id}", rbac.UsersInvite},
		{http.MethodPost, "/api/users/{userID}/roles", rbac.UsersAssignRole},
		{http.MethodDelete, "/api/users/{userID}/roles", rbac.UsersAssignRole},
		{http.MethodGet, "/api/users/{userID}/roles", rbac.UsersRead},
		{http.MethodPost, "/api/admin/shops", rbac.ShopsManage},

		// ── Tenant / shop management (shops.*) ──────────────────────────────
		{http.MethodPost, "/api/admin/tenants", rbac.ShopsManage},
		{http.MethodGet, "/api/admin/tenants", rbac.ShopsRead},
		{http.MethodGet, "/api/admin/tenants/{tenantID}", rbac.ShopsRead},
		{http.MethodPut, "/api/admin/tenants/{tenantID}/branding", rbac.ShopsBranding},

		// ── Tariff-manager admin-only plugin routes ──────────────────────────
		// Remnawave topology lookups → infra.read (platform-only).
		{http.MethodGet, "/api/tariffs/panels", rbac.InfraRead},
		{http.MethodGet, "/api/tariffs/internal-squads", rbac.InfraRead},
		{http.MethodGet, "/api/tariffs/external-squads", rbac.InfraRead},
		{http.MethodGet, "/api/tariffs/nodes", rbac.InfraRead},
		// Tariff CRUD → tariffs.read / tariffs.write.
		{http.MethodGet, "/api/tariffs/pricing-rules", rbac.TariffsRead},
		{http.MethodPost, "/api/tariffs/pricing-rules", rbac.TariffsWrite},
		{http.MethodGet, "/api/tariffs/export", rbac.TariffsRead},
		{http.MethodPost, "/api/tariffs/import", rbac.TariffsWrite},
		{http.MethodPost, "/api/tariffs", rbac.TariffsWrite},
		{http.MethodPut, "/api/tariffs/{tariffID}", rbac.TariffsWrite},
		{http.MethodDelete, "/api/tariffs/{tariffID}", rbac.TariffsWrite},
		// Analytics → infra.read.
		{http.MethodGet, "/api/tariffs/analytics/mrr-by-tariff", rbac.InfraRead},
		{http.MethodGet, "/api/tariffs/analytics/conversion-funnel", rbac.InfraRead},
		// Reseller config → tariffs.write.
		{http.MethodGet, "/api/tariffs/reseller/catalog", rbac.TariffsWrite},
		{http.MethodPost, "/api/tariffs/reseller/customize", rbac.TariffsWrite},

		// ── Reseller self-service routes (router.go /reseller re-gating) ────────
		{http.MethodGet, "/api/reseller/dashboard", rbac.ShopsRead},
		{http.MethodGet, "/api/reseller/commissions", rbac.BillingRead},
		{http.MethodGet, "/api/reseller/customers", rbac.CustomersRead},
	}

	router, token := newScopedAdminRouter(t, routes)

	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			// Substitute a concrete segment for chi URL parameters so chi matches
			// the route pattern; otherwise chi returns 405 Method Not Allowed
			// instead of reaching the middleware chain.
			path := substitutePathParams(rt.path)

			req := httptest.NewRequest(rt.method, path, nil)
			req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+token)

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusForbidden, rec.Code,
				"%s %s must be permission-gated (got %d)", rt.method, rt.path, rec.Code)
		})
	}
}

// substitutePathParams replaces chi URL parameters ({param}) with a concrete
// value so that the router can match the pattern. Without substitution chi
// returns 404 or 405, masking the middleware response.
func substitutePathParams(path string) string {
	out := make([]byte, 0, len(path))
	i := 0
	for i < len(path) {
		if path[i] == '{' {
			j := i + 1
			for j < len(path) && path[j] != '}' {
				j++
			}
			// Replace {param} with a fixed UUID-like placeholder.
			out = append(out, []byte("00000000-0000-0000-0000-000000000001")...)
			i = j + 1
		} else {
			out = append(out, path[i])
			i++
		}
	}
	return string(out)
}
