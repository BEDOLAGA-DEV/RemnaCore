package middleware

import (
	"context"
	"net/http"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
	"github.com/google/uuid"
)

// ActiveTenantID returns the active shop/tenant ID resolved for this request
// (set by ShopResolver for the panel path, or by TenantRLS for the storefront
// path). Empty string means a global/platform-level request.
func ActiveTenantID(ctx context.Context) string {
	return tenantctx.TenantIDFromContext(ctx)
}

// ShopResolver resolves the active tenant for the panel/JWT path and sets the
// RLS tenant context. It MUST run after Auth. Rules:
//   - X-Shop-Id present: must parse as a UUID, else 400 for EVERY actor (this
//     rejects the platform sentinel "*" and any non-UUID before Resolve, so
//     request input can never become the GUC sentinel). Non-platform-admins
//     must be members of the shop (403 otherwise). The parsed UUID becomes the
//     active tenant — never the raw header.
//   - X-Shop-Id absent, platform admin: server-assigned platform scope
//     (tenantctx.WithPlatformScope) — never echoed from a header.
//   - X-Shop-Id absent, non-admin: passthrough with NO tenant (fail-closed;
//     shop-scoped routes 403 at the scope gate).
//   - No claims: passthrough (Auth already rejected unauthenticated requests).
func ShopResolver(access *service.AccessService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			shopID := r.Header.Get(httpconst.HeaderShopID)
			claims := GetClaims(r.Context())
			if claims == nil {
				// Auth runs before ShopResolver and rejects unauthenticated
				// requests, so a nil-claims passthrough here is safe.
				next.ServeHTTP(w, r)
				return
			}
			if shopID == "" {
				// No active shop selected. Resolve with a nil tenant to learn
				// whether this principal is a platform admin.
				acc, err := access.Resolve(r.Context(), claims.UserID, nil)
				if err != nil {
					writeMiddlewareError(w, http.StatusInternalServerError, "authorization unavailable")
					return
				}
				if acc.IsPlatformAdmin {
					// Server-assigned platform scope (sees all tenants). The
					// sentinel is NEVER derived from a request header.
					ctx := tenantctx.WithPlatformScope(r.Context())
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				// Non-admin without a shop: pass through with NO tenant set, so
				// shop-scoped routes fail closed at the scope gate.
				next.ServeHTTP(w, r)
				return
			}
			// X-Shop-Id must be a real UUID. This rejects the platform sentinel
			// ("*") and any other non-UUID for EVERY actor (incl. platform_admin)
			// BEFORE Resolve, so request input can never become the GUC sentinel.
			parsed, parseErr := uuid.Parse(shopID)
			if parseErr != nil {
				writeMiddlewareError(w, http.StatusBadRequest, "invalid shop id")
				return
			}
			shopID = parsed.String()
			acc, err := access.Resolve(r.Context(), claims.UserID, &shopID)
			if err != nil {
				writeMiddlewareError(w, http.StatusInternalServerError, "authorization unavailable")
				return
			}
			if !acc.IsPlatformAdmin {
				if _, ok := acc.AllowedTenants[shopID]; !ok {
					writeMiddlewareError(w, http.StatusForbidden, "not a member of the requested shop")
					return
				}
			}
			ctx := tenantctx.WithTenantID(r.Context(), shopID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePermission gates a route on a single permission, scoped to the active
// tenant. It MUST run after Auth (and, for shop-scoped routes, after
// ShopResolver). Fail-closed: any resolution error returns 500 and never grants.
func RequirePermission(access *service.AccessService, p rbac.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r.Context())
			if claims == nil {
				writeMiddlewareError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			var tenantID *string
			if id := tenantctx.TenantIDFromContext(r.Context()); id != "" {
				tenantID = &id
			}
			acc, err := access.Resolve(r.Context(), claims.UserID, tenantID)
			if err != nil {
				writeMiddlewareError(w, http.StatusInternalServerError, "authorization unavailable")
				return
			}
			if !access.Can(acc, p) {
				writeMiddlewareError(w, http.StatusForbidden, "permission denied")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
