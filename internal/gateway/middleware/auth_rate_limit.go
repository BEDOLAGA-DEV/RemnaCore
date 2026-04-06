package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

const (
	// AuthRateLimitKeyPrefix is the Valkey key prefix for per-endpoint auth
	// rate limit counters.
	AuthRateLimitKeyPrefix = "auth:"
)

// AuthRateLimitConfig configures per-endpoint auth rate limiting.
type AuthRateLimitConfig struct {
	// Limit is the maximum number of requests allowed per window per IP.
	Limit int
	// Window is the sliding window duration.
	Window time.Duration
}

// AuthRateLimiters groups the per-endpoint rate limiters and their
// configurations for auth endpoints. It is provided to the router via Fx.
type AuthRateLimiters struct {
	Login         RateLimiter
	LoginCfg      AuthRateLimitConfig
	ForgotPwd     RateLimiter
	ForgotPwdCfg  AuthRateLimitConfig
}

// AuthRateLimit returns middleware that rate-limits auth endpoints by client IP
// using a dedicated RateLimiter instance with endpoint-specific thresholds.
//
// The limiter follows a fail-open policy: if the rate limiter returns an error
// the request is allowed through. When the limit is exceeded the middleware
// returns 429 with a Retry-After header set to the window duration in seconds.
func AuthRateLimit(limiter RateLimiter, cfg AuthRateLimitConfig, endpointName string) func(http.Handler) http.Handler {
	retryAfterSeconds := fmt.Sprintf("%d", int(cfg.Window.Seconds()))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			key := fmt.Sprintf("%s%s:%s", AuthRateLimitKeyPrefix, endpointName, ip)

			allowed, err := limiter.Allow(r.Context(), key)
			if err != nil {
				// Fail-open: allow the request when the limiter is unavailable.
				next.ServeHTTP(w, r)
				return
			}

			if !allowed {
				w.Header().Set(httpconst.HeaderRetryAfter, retryAfterSeconds)
				writeMiddlewareError(w, http.StatusTooManyRequests, "too many requests")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
