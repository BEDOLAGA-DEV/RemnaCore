package middleware

import "net/http"

const (
	// DefaultMaxBodyBytes is the maximum allowed request body size (1 MB).
	// Requests exceeding this limit receive a read error from
	// http.MaxBytesReader, which surfaces as a decode failure in handlers.
	DefaultMaxBodyBytes int64 = 1 << 20 // 1 MB
)

// MaxBodySize returns middleware that limits the size of incoming request
// bodies. If the client sends more than maxBytes, http.MaxBytesReader causes
// the next Read to return an *http.MaxBytesError. Downstream handlers that
// decode the body (e.g. via json.NewDecoder) will observe this as a decode
// failure and should respond with an appropriate HTTP error.
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
