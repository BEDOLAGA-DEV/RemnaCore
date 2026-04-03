package middleware

import (
	"net/http"
)

// ResellerRole is the role value that grants access to reseller self-service endpoints.
const ResellerRole = "reseller"

// RequireReseller is a chi-compatible middleware that checks the authenticated user
// has the reseller or admin role. Admins are granted access so they can inspect
// reseller views without role-switching. It MUST be placed after Auth in the
// middleware chain so that UserClaims are already present in the request context.
//
// Returns 401 Unauthorized when no claims exist (unauthenticated request) and
// 403 Forbidden when the user is authenticated but lacks the required role.
func RequireReseller(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := GetClaims(r.Context())
		if claims == nil {
			writeMiddlewareError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if claims.Role != ResellerRole && claims.Role != AdminRole {
			writeMiddlewareError(w, http.StatusForbidden, "reseller access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
