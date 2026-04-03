package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

const (
	// ClaimsContextKey is the typed context key for storing authenticated user claims.
	ClaimsContextKey contextKey = "user_claims"
)

// Auth returns middleware that validates a JWT bearer token from the
// Authorization header. On success the decoded UserClaims are stored in the
// request context. On failure a 401 JSON error is returned.
func Auth(jwt *authutil.JWTIssuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get(httpconst.HeaderAuthorization)
			if header == "" || !strings.HasPrefix(header, httpconst.BearerPrefix) {
				writeAuthError(w, http.StatusUnauthorized, "missing or malformed authorization header")
				return
			}

			tokenString := strings.TrimPrefix(header, httpconst.BearerPrefix)
			claims, err := jwt.Verify(tokenString)
			if err != nil {
				writeAuthError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), ClaimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetClaims extracts authenticated UserClaims from the request context.
// Returns nil if no claims are present (i.e. unauthenticated request).
func GetClaims(ctx context.Context) *authutil.UserClaims {
	claims, _ := ctx.Value(ClaimsContextKey).(*authutil.UserClaims)
	return claims
}

// writeAuthError writes a JSON error response for authentication failures.
func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	w.WriteHeader(status)
	// Minimal JSON — no need to pull in encoding/json for a static format.
	_, _ = w.Write([]byte(`{"error":"` + message + `"}`))
}
