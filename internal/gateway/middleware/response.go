package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
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

// writeMiddlewareAPIError writes a structured apierror.Error as JSON, matching
// the handler-layer writeAPIError shape so middleware-originated errors share
// the API's stable {code,message} contract. Used for cases (e.g. a shop-scoped
// permission with no active tenant) that need a machine-readable code the
// flat writeMiddlewareError cannot express.
func writeMiddlewareAPIError(w http.ResponseWriter, apiErr *apierror.Error) {
	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	w.WriteHeader(apiErr.HTTPStatus)
	_ = json.NewEncoder(w).Encode(apiErr)
}
