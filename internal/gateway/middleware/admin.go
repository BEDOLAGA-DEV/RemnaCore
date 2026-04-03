package middleware

import (
	"net/http"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
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
			writeAdminError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if claims.Role != AdminRole {
			writeAdminError(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// writeAdminError writes a minimal JSON error response. It follows the same
// pattern used by writeAuthError and writeTenantError in sibling middleware
// files, keeping the middleware package free from handler dependencies.
func writeAdminError(w http.ResponseWriter, status int, message string) {
	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + message + `"}`))
}
