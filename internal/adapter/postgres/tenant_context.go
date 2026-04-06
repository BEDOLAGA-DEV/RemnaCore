package postgres

import "context"

// tenantIDCtxKey is the context key for the current tenant's UUID string.
// It is separate from the middleware's TenantContextKey (which stores the
// full *reseller.Tenant) to maintain package isolation.
type tenantIDCtxKey struct{}

// WithTenantID returns a child context carrying tenantID. Downstream code
// (TxManager) reads this value to set the PostgreSQL session variable
// app.tenant_id for row-level security enforcement.
//
// Pass an empty string for platform-level (superuser) requests that should
// see all rows regardless of tenant.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDCtxKey{}, tenantID)
}

// TenantIDFromContext extracts the tenant ID stored by WithTenantID. Returns
// an empty string when no tenant is set (platform-level request).
func TenantIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(tenantIDCtxKey{}).(string)
	return v
}
