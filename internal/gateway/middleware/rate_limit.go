package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/valkey"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

const (
	// ForwardedForHeader is the standard header for identifying client IPs
	// behind proxies.
	ForwardedForHeader = "X-Forwarded-For"

	// RateLimitedMessage is the JSON body returned when a client exceeds the
	// rate limit.
	RateLimitedMessage = `{"error":"rate limit exceeded"}`
)

// RateLimit returns middleware that applies per-key rate limiting. Authenticated
// requests are keyed by user ID; unauthenticated requests are keyed by client
// IP. The limiter follows a fail-open policy: if the rate limiter itself returns
// an error, the request is allowed through.
func RateLimit(limiter valkey.RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := rateLimitKey(r)

			allowed, err := limiter.Allow(r.Context(), key)
			if err != nil {
				// Fail-open: allow the request when the limiter is unavailable.
				next.ServeHTTP(w, r)
				return
			}

			if !allowed {
				w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(RateLimitedMessage))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// rateLimitKey returns the user ID if the request is authenticated, otherwise
// falls back to the client IP address.
func rateLimitKey(r *http.Request) string {
	if claims := GetClaims(r.Context()); claims != nil {
		return "user:" + claims.UserID
	}
	return "ip:" + clientIP(r)
}

// clientIP extracts the client IP from X-Forwarded-For (first entry) or falls
// back to RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get(ForwardedForHeader); xff != "" {
		// X-Forwarded-For may contain a comma-separated list; take the first.
		if idx := strings.IndexByte(xff, ','); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// RemoteAddr is "host:port"; strip the port.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
