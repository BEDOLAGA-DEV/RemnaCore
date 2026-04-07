package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

// domainErrorMappers is the ordered list of per-context error mappers. Each
// mapper returns a non-nil *apierror.Error when the error belongs to its
// bounded context, or nil to pass through to the next mapper.
var domainErrorMappers = []func(error) *apierror.Error{
	identity.MapToAPIError,
	billing.MapToAPIError,
	aggregate.MapToAPIError,
	multisub.MapToAPIError,
	payment.MapToAPIError,
	reseller.MapToAPIError,
	plugin.MapToAPIError,
}

// writeJSON marshals data as JSON and writes it with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	w.WriteHeader(status)
	// Encode error cannot be handled after headers are sent.
	_ = json.NewEncoder(w).Encode(data)
}

// writeError writes a JSON error response with the given HTTP status code.
// Retained for backward compatibility with middleware and simple cases.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// writeAPIError writes a structured API error as JSON.
func writeAPIError(w http.ResponseWriter, apiErr *apierror.Error) {
	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	w.WriteHeader(apiErr.HTTPStatus)
	// Encode error cannot be handled after headers are sent.
	_ = json.NewEncoder(w).Encode(apiErr)
}

// writeErrorFromDomain maps a domain error to a structured API error and writes
// it as JSON. Unknown errors are mapped to COMMON.INTERNAL without leaking
// implementation details.
func writeErrorFromDomain(w http.ResponseWriter, err error) {
	writeAPIError(w, mapDomainError(err))
}

// writeValidationError writes a structured validation error, detecting
// MaxBytesError to return COMMON.BODY_TOO_LARGE instead of generic validation.
func writeValidationError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeAPIError(w, apierror.BodyTooLarge)
		return
	}
	writeAPIError(w, apierror.ValidationFailed)
}

// mapDomainError converts a domain sentinel error to a structured API error by
// delegating to per-context mappers. Each bounded context owns its own
// error-to-API-code mapping. Unknown errors are mapped to COMMON.INTERNAL
// without leaking details.
func mapDomainError(err error) *apierror.Error {
	for _, m := range domainErrorMappers {
		if apiErr := m(err); apiErr != nil {
			return apiErr
		}
	}
	return apierror.Internal
}

// mapServiceError translates a domain-level error into an HTTP status code and
// user-facing message. Deprecated: prefer writeErrorFromDomain which returns
// structured error codes. Retained for backward compatibility.
func mapServiceError(err error) (status int, message string) {
	apiErr := mapDomainError(err)
	return apiErr.HTTPStatus, apiErr.Message
}
