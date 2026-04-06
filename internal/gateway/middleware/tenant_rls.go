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
//
// Tenant isolation flow:
//
//  1. TenantResolver resolves tenant from API key or Host/Origin header
//  2. TenantRLS extracts tenant.ID and stores it via tenantctx.WithTenantID
//  3. TxManager.RunInTx reads tenantctx and calls SET LOCAL 'app.tenant_id'
//  4. PostgreSQL RLS policies on Tier 1 tables (identity.platform_users,
//     reseller.reseller_accounts) enforce row visibility using app.tenant_id
//  5. Tier 2 tables (billing.subscriptions, billing.invoices,
//     multisub.remnawave_bindings, payment.payment_records) are isolated via
//     application-layer user_id from JWT claims — all handler queries filter
//     by user_id, which itself is RLS-protected through platform_users
//  6. Tier 3 tables (billing.plans, billing.plan_addons) are shared across
//     tenants and have no RLS policies
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
