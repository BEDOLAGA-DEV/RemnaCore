package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

const (
	// APIKeyHeader is kept as an alias for backward compatibility. New code
	// should reference httpconst.HeaderAPIKey directly.
	APIKeyHeader = httpconst.HeaderAPIKey

	// TenantContextKey is the typed context key for storing the resolved tenant.
	TenantContextKey contextKey = "tenant"
)

// TenantResolver returns middleware that resolves the current tenant from an
// API key header or the request Host/Origin header for domain matching. If a
// tenant is resolved, it is stored in the request context. If no tenant
// identifier is present, the request proceeds without a tenant (anonymous).
func TenantResolver(resellerSvc *reseller.ResellerService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// 1. Check X-API-Key header.
			if apiKey := r.Header.Get(APIKeyHeader); apiKey != "" {
				tenant, err := resellerSvc.ValidateAPIKey(ctx, apiKey)
				if err != nil {
					writeTenantError(w, http.StatusUnauthorized, "invalid API key")
					return
				}
				ctx = context.WithValue(ctx, TenantContextKey, tenant)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// 2. Check Origin or Host header for domain matching.
			domain := extractDomain(r)
			if domain != "" {
				tenant, err := resellerSvc.GetTenantByDomain(ctx, domain)
				if err == nil && tenant.IsActive {
					ctx = context.WithValue(ctx, TenantContextKey, tenant)
				}
				// If domain lookup fails, proceed without a tenant context.
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetTenant extracts the resolved tenant from the request context.
// Returns nil if no tenant was resolved (i.e. platform-level request).
func GetTenant(ctx context.Context) *reseller.Tenant {
	tenant, _ := ctx.Value(TenantContextKey).(*reseller.Tenant)
	return tenant
}

// extractDomain returns the host portion of the Origin header (preferred) or
// the Host header, stripping any port suffix.
func extractDomain(r *http.Request) string {
	origin := r.Header.Get("Origin")
	if origin != "" {
		// Strip scheme (http:// or https://).
		if idx := strings.Index(origin, "://"); idx >= 0 {
			origin = origin[idx+3:]
		}
		// Strip port.
		if idx := strings.IndexByte(origin, ':'); idx >= 0 {
			origin = origin[:idx]
		}
		return origin
	}

	host := r.Host
	if host == "" {
		return ""
	}
	// Strip port.
	if idx := strings.IndexByte(host, ':'); idx >= 0 {
		host = host[:idx]
	}
	return host
}

// writeTenantError writes a JSON error response for tenant resolution failures.
func writeTenantError(w http.ResponseWriter, status int, message string) {
	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + message + `"}`))
}
