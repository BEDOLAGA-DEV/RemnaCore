package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
)

func TestMaxBodySize(t *testing.T) {
	const limitBytes int64 = 16

	tests := []struct {
		name       string
		bodySize   int
		wantErr    bool
		wantStatus int
	}{
		{
			name:       "body within limit is read successfully",
			bodySize:   10,
			wantErr:    false,
			wantStatus: http.StatusOK,
		},
		{
			name:       "body at exact limit is read successfully",
			bodySize:   int(limitBytes),
			wantErr:    false,
			wantStatus: http.StatusOK,
		},
		{
			name:       "body exceeding limit causes read error",
			bodySize:   int(limitBytes) + 1,
			wantErr:    true,
			wantStatus: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var readErr error

			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, readErr = io.ReadAll(r.Body)
				if readErr != nil {
					w.WriteHeader(http.StatusRequestEntityTooLarge)
					return
				}
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware.MaxBodySize(limitBytes)(inner)

			body := strings.NewReader(strings.Repeat("x", tt.bodySize))
			req := httptest.NewRequest(http.MethodPost, "/test", body)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if tt.wantErr {
				require.Error(t, readErr, "expected read error for oversized body")
				assert.ErrorAs(t, readErr, new(*http.MaxBytesError))
			} else {
				require.NoError(t, readErr, "expected no read error for body within limit")
			}

			assert.Equal(t, tt.wantStatus, rr.Code)
		})
	}
}

func TestMaxBodySize_DefaultConstant(t *testing.T) {
	const expectedOneMB int64 = 1 << 20
	assert.Equal(t, expectedOneMB, middleware.DefaultMaxBodyBytes)
}

// drainBody returns a handler that reads the request body and a pointer to the
// resulting error, so a test can assert whether the limit cut the read short.
func drainBody() (http.Handler, *error) {
	var readErr error
	h := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	})
	return h, &readErr
}

// Plugin installation uploads a multi-megabyte WASM module. The ceiling has to
// be decided by the outermost middleware: it wraps r.Body first, so a larger
// limit applied further down the chain is still capped by whatever this one
// let through. Hence a per-request limit rather than a per-route one.
func TestMaxBodySizeFunc_AppliesPerRequestLimit(t *testing.T) {
	const uploadPath = "/api/admin/plugins"
	const smallLimit int64 = 16

	limitFor := func(r *http.Request) int64 {
		if r.Method == http.MethodPost && r.URL.Path == uploadPath {
			return 1 << 20
		}
		return smallLimit
	}

	tests := []struct {
		name    string
		method  string
		path    string
		wantErr bool
	}{
		{name: "upload route gets the larger ceiling", method: http.MethodPost, path: uploadPath, wantErr: false},
		{name: "same path, other method keeps the default", method: http.MethodPut, path: uploadPath, wantErr: true},
		{name: "unrelated route keeps the default", method: http.MethodPost, path: "/api/auth/login", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner, readErr := drainBody()
			body := strings.NewReader(strings.Repeat("x", int(smallLimit)*8))
			req := httptest.NewRequest(tt.method, tt.path, body)

			middleware.MaxBodySizeFunc(limitFor)(inner).ServeHTTP(httptest.NewRecorder(), req)

			if tt.wantErr {
				require.Error(t, *readErr)
				assert.ErrorAs(t, *readErr, new(*http.MaxBytesError))
			} else {
				require.NoError(t, *readErr)
			}
		})
	}
}

// A Go-compiled plugin runs to megabytes — the sample bot committed to this
// repository is ~3.2 MB, and the install payload base64-encodes it. The upload
// ceiling has to clear that with room to spare or no real plugin installs.
func TestMaxUploadBodyBytes_ClearsRealPluginSize(t *testing.T) {
	const committedSampleBot int64 = 4 << 20

	assert.Greater(t, middleware.MaxUploadBodyBytes, committedSampleBot)
	assert.Greater(t, middleware.MaxUploadBodyBytes, middleware.DefaultMaxBodyBytes)
}
