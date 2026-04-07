package speedtest

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSpeedTestServer(t *testing.T) *SpeedTestServer {
	t.Helper()
	srv, err := NewSpeedTestServer(slog.Default())
	require.NoError(t, err)
	return srv
}

func TestSpeedTestServer_Ping(t *testing.T) {
	srv := newTestSpeedTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()

	srv.Ping(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"pong":true`)
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
}

func TestSpeedTestServer_Download_DefaultSize(t *testing.T) {
	srv := newTestSpeedTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/download", nil)
	w := httptest.NewRecorder()

	srv.Download(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, SpeedTestRandomBufSize, w.Body.Len())
	assert.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
}

func TestSpeedTestServer_Download_CustomSize(t *testing.T) {
	srv := newTestSpeedTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/download?size=512", nil)
	w := httptest.NewRecorder()

	srv.Download(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 512, w.Body.Len())
}

func TestSpeedTestServer_Download_InvalidSize(t *testing.T) {
	srv := newTestSpeedTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/download?size=invalid", nil)
	w := httptest.NewRecorder()

	srv.Download(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSpeedTestServer_Download_ExceedsMax(t *testing.T) {
	srv := newTestSpeedTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/download?size=999999999", nil)
	w := httptest.NewRecorder()

	srv.Download(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, MaxDownloadSize, w.Body.Len())
}

func TestSpeedTestServer_Upload(t *testing.T) {
	srv := newTestSpeedTestServer(t)

	body := strings.NewReader("hello upload test data")
	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	w := httptest.NewRecorder()

	srv.Upload(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"bytes_received":`)
}

func TestSpeedTestServer_Upload_WrongMethod(t *testing.T) {
	srv := newTestSpeedTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/upload", nil)
	w := httptest.NewRecorder()

	srv.Upload(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestSpeedTestServer_RandomBufPreAllocated(t *testing.T) {
	srv := newTestSpeedTestServer(t)

	require.Len(t, srv.randomBuf, SpeedTestRandomBufSize)

	// Verify the buffer is not all zeros (it was filled with crypto/rand).
	allZero := true
	for _, b := range srv.randomBuf[:256] {
		if b != 0 {
			allZero = false
			break
		}
	}
	assert.False(t, allZero, "random buffer should not be all zeros")
}

func TestInProcessLimiter_AllowsWithinLimit(t *testing.T) {
	limiter := newInProcessLimiter(SpeedTestRateLimit, SpeedTestRateLimitWindow)

	for i := range SpeedTestRateLimit {
		allowed := limiter.Allow("192.168.1.1")
		assert.True(t, allowed, "request %d should be allowed", i+1)
	}
}

func TestInProcessLimiter_RejectsOverLimit(t *testing.T) {
	limiter := newInProcessLimiter(SpeedTestRateLimit, SpeedTestRateLimitWindow)

	for range SpeedTestRateLimit {
		limiter.Allow("192.168.1.1")
	}

	allowed := limiter.Allow("192.168.1.1")
	assert.False(t, allowed, "request exceeding limit should be rejected")
}

func TestInProcessLimiter_DifferentIPsIndependent(t *testing.T) {
	limiter := newInProcessLimiter(SpeedTestRateLimit, SpeedTestRateLimitWindow)

	// Exhaust limit for IP A.
	for range SpeedTestRateLimit {
		limiter.Allow("10.0.0.1")
	}
	assert.False(t, limiter.Allow("10.0.0.1"), "IP A should be rate limited")

	// IP B must still be allowed.
	assert.True(t, limiter.Allow("10.0.0.2"), "IP B should not be affected by IP A")
}

func TestWithRateLimit_Returns429(t *testing.T) {
	limiter := newInProcessLimiter(SpeedTestRateLimit, SpeedTestRateLimitWindow)
	handler := withRateLimit(limiter, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Exhaust the limit.
	for range SpeedTestRateLimit {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "1.2.3.4:12345"
		w := httptest.NewRecorder()
		handler(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Next request should be 429.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "1.2.3.4:12345"
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "60", w.Header().Get("Retry-After"))
}

func TestSpeedTestServer_Integration(t *testing.T) {
	srv := newTestSpeedTestServer(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/download", srv.Download)
	mux.HandleFunc("/upload", srv.Upload)
	mux.HandleFunc("/ping", srv.Ping)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Test ping via real HTTP.
	resp, err := http.Get(ts.URL + "/ping")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "pong")
}
