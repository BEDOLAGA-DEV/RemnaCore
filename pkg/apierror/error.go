// Package apierror provides structured API error types with machine-readable
// codes and HTTP status mapping. Handlers use these instead of bare strings to
// give API consumers a stable contract for error handling.
package apierror

import "fmt"

// Error represents a structured API error with a machine-readable code.
type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
	Details    any    `json:"details,omitempty"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// New creates a new API error with the given code, message, and HTTP status.
func New(code, message string, status int) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: status}
}

// WithDetails returns a shallow copy of the error with the given details
// attached. The original error is not mutated.
func (e *Error) WithDetails(details any) *Error {
	cp := *e
	cp.Details = details
	return &cp
}
