package middleware_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/valkey"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

const (
	testLoginLimit       = 20
	testForgotPwdLimit   = 3
	testLoginWindow      = 15 * time.Minute
	testForgotPwdWindow  = 1 * time.Hour
	testEndpointLogin    = "login"
	testEndpointForgotPw = "forgot_password"
)

func TestAuthRateLimit_LoginAllowsWithinLimit(t *testing.T) {
	rl := valkey.NewInMemoryRateLimiter(testLoginLimit)
	cfg := middleware.AuthRateLimitConfig{
		Limit:  testLoginLimit,
		Window: testLoginWindow,
	}
	handler := middleware.AuthRateLimit(rl, cfg, testEndpointLogin)(okHandler(t))

	for i := range testLoginLimit {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code, "request %d should be allowed", i+1)
	}
}

func TestAuthRateLimit_LoginBlocksOverLimit(t *testing.T) {
	rl := valkey.NewInMemoryRateLimiter(testLoginLimit)
	cfg := middleware.AuthRateLimitConfig{
		Limit:  testLoginLimit,
		Window: testLoginWindow,
	}
	handler := middleware.AuthRateLimit(rl, cfg, testEndpointLogin)(okHandler(t))

	// Exhaust the limit.
	for range testLoginLimit {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// The next request should be blocked.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Contains(t, rec.Body.String(), "too many requests")
	assert.Equal(t, "900", rec.Header().Get(httpconst.HeaderRetryAfter))
}

func TestAuthRateLimit_ForgotPasswordAllowsWithinLimit(t *testing.T) {
	rl := valkey.NewInMemoryRateLimiter(testForgotPwdLimit)
	cfg := middleware.AuthRateLimitConfig{
		Limit:  testForgotPwdLimit,
		Window: testForgotPwdWindow,
	}
	handler := middleware.AuthRateLimit(rl, cfg, testEndpointForgotPw)(okHandler(t))

	for i := range testForgotPwdLimit {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", nil)
		req.RemoteAddr = "10.0.0.1:54321"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code, "request %d should be allowed", i+1)
	}
}

func TestAuthRateLimit_ForgotPasswordBlocksOverLimit(t *testing.T) {
	rl := valkey.NewInMemoryRateLimiter(testForgotPwdLimit)
	cfg := middleware.AuthRateLimitConfig{
		Limit:  testForgotPwdLimit,
		Window: testForgotPwdWindow,
	}
	handler := middleware.AuthRateLimit(rl, cfg, testEndpointForgotPw)(okHandler(t))

	// Exhaust the limit.
	for range testForgotPwdLimit {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", nil)
		req.RemoteAddr = "10.0.0.1:54321"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// The next request should be blocked.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", nil)
	req.RemoteAddr = "10.0.0.1:54321"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Contains(t, rec.Body.String(), "too many requests")
	assert.Equal(t, "3600", rec.Header().Get(httpconst.HeaderRetryAfter))
}

func TestAuthRateLimit_DifferentIPsAreIndependent(t *testing.T) {
	rl := valkey.NewInMemoryRateLimiter(testForgotPwdLimit)
	cfg := middleware.AuthRateLimitConfig{
		Limit:  testForgotPwdLimit,
		Window: testForgotPwdWindow,
	}
	handler := middleware.AuthRateLimit(rl, cfg, testEndpointForgotPw)(okHandler(t))

	// Exhaust limit for IP-A.
	for range testForgotPwdLimit {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", nil)
		req.RemoteAddr = "10.0.0.1:54321"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// IP-A should be blocked.
	reqBlocked := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", nil)
	reqBlocked.RemoteAddr = "10.0.0.1:54321"
	recBlocked := httptest.NewRecorder()
	handler.ServeHTTP(recBlocked, reqBlocked)
	assert.Equal(t, http.StatusTooManyRequests, recBlocked.Code)

	// IP-B should still be allowed.
	reqAllowed := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", nil)
	reqAllowed.RemoteAddr = "10.0.0.2:54321"
	recAllowed := httptest.NewRecorder()
	handler.ServeHTTP(recAllowed, reqAllowed)
	assert.Equal(t, http.StatusOK, recAllowed.Code)
}

func TestAuthRateLimit_FailOpen(t *testing.T) {
	rl := &failingRateLimiter{}
	cfg := middleware.AuthRateLimitConfig{
		Limit:  testLoginLimit,
		Window: testLoginWindow,
	}
	handler := middleware.AuthRateLimit(rl, cfg, testEndpointLogin)(okHandler(t))

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, "should fail open on limiter error")
}

func TestAuthRateLimit_RetryAfterHeader(t *testing.T) {
	tests := []struct {
		name           string
		window         time.Duration
		wantRetryAfter string
	}{
		{
			name:           "15 minute window",
			window:         15 * time.Minute,
			wantRetryAfter: "900",
		},
		{
			name:           "1 hour window",
			window:         1 * time.Hour,
			wantRetryAfter: "3600",
		},
		{
			name:           "30 second window",
			window:         30 * time.Second,
			wantRetryAfter: "30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := valkey.NewInMemoryRateLimiter(1)
			cfg := middleware.AuthRateLimitConfig{
				Limit:  1,
				Window: tt.window,
			}
			handler := middleware.AuthRateLimit(rl, cfg, testEndpointLogin)(okHandler(t))

			// First request: allowed.
			req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code)

			// Second request: blocked with Retry-After.
			req = httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusTooManyRequests, rec.Code)
			assert.Equal(t, tt.wantRetryAfter, rec.Header().Get(httpconst.HeaderRetryAfter))
		})
	}
}

func TestAuthRateLimit_UsesXForwardedFor(t *testing.T) {
	rl := valkey.NewInMemoryRateLimiter(1)
	cfg := middleware.AuthRateLimitConfig{
		Limit:  1,
		Window: testLoginWindow,
	}
	handler := middleware.AuthRateLimit(rl, cfg, testEndpointLogin)(okHandler(t))

	// First request with X-Forwarded-For: allowed.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set(httpconst.HeaderForwardedFor, "203.0.113.50")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Second request from same forwarded IP: blocked.
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set(httpconst.HeaderForwardedFor, "203.0.113.50")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	// Different forwarded IP: allowed.
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set(httpconst.HeaderForwardedFor, "203.0.113.51")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// okHandler returns a handler that always responds with 200 OK.
func okHandler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// failingRateLimiter always returns an error from Allow.
type failingRateLimiter struct{}

func (f *failingRateLimiter) Allow(_ context.Context, _ string) (bool, error) {
	return false, fmt.Errorf("limiter unavailable")
}
