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

// ShopResolver reads X-Shop-Id (panel/JWT path), validates that the
// authenticated user is bound to that shop (or is a platform admin), and only
// then sets it as the active tenant for RLS. It MUST run after Auth. Requests
// without X-Shop-Id pass through unchanged (the storefront TenantResolver path,
// which ran pre-Auth, is unaffected).
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
