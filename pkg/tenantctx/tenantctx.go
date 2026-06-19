// Package tenantctx provides context-based tenant ID propagation for
// multi-tenant row-level security (RLS). Although consumed by only two layers
// (gateway/middleware sets the tenant ID, adapter/postgres reads it for
// SET app.tenant_id), it lives in pkg/ because both layers must share the
// same context key without importing each other.
package tenantctx

import "context"

type tenantIDKey struct{}

// WithTenantID returns a child context carrying tenantID. Downstream code
// (TxManager) reads this value to set the PostgreSQL session variable
// app.tenant_id for row-level security enforcement.
//
// RLS is fail-closed: an empty (or unset) tenant ID means the GUC is not set
// and policies match ZERO rows. For platform-level scope that sees all tenants,
// use WithPlatformScope (which sets the PlatformScopeSentinel) — never an empty
// string.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey{}, tenantID)
}

// TenantIDFromContext extracts the tenant ID stored by WithTenantID. Returns
// an empty string when no tenant is set; under fail-closed RLS that means the
// GUC stays unset and policies match zero rows (NOT platform-wide visibility).
func TenantIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(tenantIDKey{}).(string)
	return v
}

// PlatformScopeSentinel is the app.tenant_id value that RLS policies treat as
// "all tenants" (platform-admin / system scope). It is a real value the policy
// matches on, NOT an unset GUC — so an unset/empty GUC fails closed.
const PlatformScopeSentinel = "*"

// WithPlatformScope marks ctx as platform-scoped (sees all tenants under the
// sentinel-aware RLS policies). Server-assigned only — never from request input.
func WithPlatformScope(ctx context.Context) context.Context {
	return WithTenantID(ctx, PlatformScopeSentinel)
}
