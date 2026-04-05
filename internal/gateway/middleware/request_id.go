package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

type contextKey string

const (
	// RequestIDKey is the context key for the request ID.
	RequestIDKey contextKey = "request_id"

	// RequestIDHeader is kept as an alias for backward compatibility. New code
	// should reference httpconst.HeaderRequestID directly.
	RequestIDHeader = httpconst.HeaderRequestID
)

// RequestID injects a unique request ID into the context and response header.
// If the incoming request already carries an X-Request-ID header, that value is
// reused; otherwise a new UUID is generated.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" {
			id = uuid.Must(uuid.NewV7()).String()
		}

		w.Header().Set(RequestIDHeader, id)
		ctx := context.WithValue(r.Context(), RequestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
