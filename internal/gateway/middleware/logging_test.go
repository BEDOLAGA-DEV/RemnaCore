package middleware_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
)

func TestRequestLogger_AttachesRequestID(t *testing.T) {
	const testRequestID = "test-req-id-123"

	var captured *slog.Logger

	handler := middleware.RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = middleware.LoggerFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	// Build a request with a pre-set request ID in context (as RequestID middleware would).
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := context.WithValue(req.Context(), middleware.RequestIDKey, testRequestID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	require.NotNil(t, captured, "logger should be set in context")

	// Write a log message and verify request_id is present.
	var buf bytes.Buffer
	testHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	testLogger := slog.New(captured.Handler().WithAttrs([]slog.Attr{}))
	_ = testLogger
	// Instead, create a logger from the captured handler's underlying attrs.
	// Simpler approach: write via the captured logger and check output.
	// Since captured wraps slog.Default(), let's use a custom default for this test.

	// Replace default logger with one that writes to buffer to capture output.
	originalDefault := slog.Default()
	slog.SetDefault(slog.New(testHandler))
	t.Cleanup(func() { slog.SetDefault(originalDefault) })

	// Re-run with the new default so the middleware's With() uses our handler.
	var captured2 *slog.Logger
	handler2 := middleware.RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured2 = middleware.LoggerFromContext(r.Context())
		captured2.Info("test message")
		w.WriteHeader(http.StatusOK)
	}))

	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx2 := context.WithValue(req2.Context(), middleware.RequestIDKey, testRequestID)
	req2 = req2.WithContext(ctx2)
	handler2.ServeHTTP(httptest.NewRecorder(), req2)

	output := buf.String()
	assert.Contains(t, output, `"request_id"`)
	assert.Contains(t, output, testRequestID)
	assert.Contains(t, output, "test message")
}

func TestLoggerFromContext_ReturnsDefault_WhenNoLogger(t *testing.T) {
	logger := middleware.LoggerFromContext(context.Background())
	assert.Equal(t, slog.Default(), logger)
}

func TestRequestLogger_HandlesEmptyRequestID(t *testing.T) {
	var captured *slog.Logger

	handler := middleware.RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = middleware.LoggerFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	// No RequestIDKey in context -- empty string is used.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	require.NotNil(t, captured, "logger should still be set even with empty request_id")
}
