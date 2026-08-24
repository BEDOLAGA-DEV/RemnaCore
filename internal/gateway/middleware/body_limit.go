package middleware

import "net/http"

const (
	// DefaultMaxBodyBytes is the maximum allowed request body size (1 MB).
	// Requests exceeding this limit receive a read error from
	// http.MaxBytesReader, which surfaces as a decode failure in handlers.
	DefaultMaxBodyBytes int64 = 1 << 20 // 1 MB

	// MaxUploadBodyBytes is the ceiling for the endpoints that accept a binary
	// upload. A Go-compiled WASM plugin runs to several megabytes — the sample
	// bot committed to this repository is 3.2 MB, and the install payload
	// base64-encodes it, adding a third on top — so the default limit rejects
	// every realistic plugin before the handler ever sees it.
	MaxUploadBodyBytes int64 = 32 << 20 // 32 MB
)

// MaxBodySize returns middleware that limits the size of incoming request
// bodies. If the client sends more than maxBytes, http.MaxBytesReader causes
// the next Read to return an *http.MaxBytesError. Downstream handlers that
// decode the body (e.g. via json.NewDecoder) will observe this as a decode
// failure and should respond with an appropriate HTTP error.
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return MaxBodySizeFunc(func(*http.Request) int64 { return maxBytes })
}

// MaxBodySizeFunc is MaxBodySize with a per-request ceiling. The limit has to
// be decided here rather than on the individual route: this middleware runs
// first and wraps r.Body, so a larger limit applied further down the chain is
// still capped by whatever this one allowed through.
func MaxBodySizeFunc(limitFor func(*http.Request) int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, limitFor(r))
			next.ServeHTTP(w, r)
		})
	}
}
