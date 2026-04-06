package middleware

import (
	"net/http"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

// TenantRLS is a middleware that propagates the resolved tenant ID into the
// context key read by the TxManager. This bridges the gap between the HTTP
// middleware layer (which stores *reseller.Tenant) and the adapter layer
// (which needs a plain tenant ID string for set_config('app.tenant_id', ...)).
//
// TenantRLS MUST run after TenantResolver in the middleware chain.
func TenantRLS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if tenant := GetTenant(ctx); tenant != nil {
			ctx = tenantctx.WithTenantID(ctx, tenant.ID)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}
