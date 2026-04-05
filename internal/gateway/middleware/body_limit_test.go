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
