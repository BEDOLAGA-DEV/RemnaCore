package middleware

import (
	"fmt"
	"net/http"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

// writeMiddlewareError writes a minimal JSON error response. It is the single
// shared helper used by all middleware in this package, keeping the middleware
// free from handler-level dependencies.
func writeMiddlewareError(w http.ResponseWriter, status int, message string) {
	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":%q}`, message)
}
