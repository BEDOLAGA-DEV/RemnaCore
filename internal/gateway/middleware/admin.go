package middleware

import (
	"net/http"
)

// AdminRole is the role value that grants access to admin-only endpoints.
const AdminRole = "admin"

// RequireAdmin is a chi-compatible middleware that checks the authenticated user
// has the admin role. It MUST be placed after Auth in the middleware chain so
// that UserClaims are already present in the request context.
//
// Returns 401 Unauthorized when no claims exist (unauthenticated request) and
// 403 Forbidden when the user is authenticated but lacks the admin role.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := GetClaims(r.Context())
		if claims == nil {
			writeMiddlewareError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if claims.Role != AdminRole {
			writeMiddlewareError(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
