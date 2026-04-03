package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
)

func TestRequireAdmin(t *testing.T) {
	// nextCalled tracks whether the wrapped handler was invoked.
	var nextCalled bool
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name           string
		claims         *authutil.UserClaims // nil means unauthenticated
		wantStatus     int
		wantNextCalled bool
		wantError      string
	}{
		{
			name:           "admin role passes through",
			claims:         &authutil.UserClaims{UserID: "u-1", Email: "admin@example.com", Role: AdminRole},
			wantStatus:     http.StatusOK,
			wantNextCalled: true,
		},
		{
			name:           "customer role returns 403",
			claims:         &authutil.UserClaims{UserID: "u-2", Email: "user@example.com", Role: "customer"},
			wantStatus:     http.StatusForbidden,
			wantNextCalled: false,
			wantError:      "admin access required",
		},
		{
			name:           "reseller role returns 403",
			claims:         &authutil.UserClaims{UserID: "u-3", Email: "reseller@example.com", Role: "reseller"},
			wantStatus:     http.StatusForbidden,
			wantNextCalled: false,
			wantError:      "admin access required",
		},
		{
			name:           "empty role returns 403",
			claims:         &authutil.UserClaims{UserID: "u-4", Email: "norole@example.com", Role: ""},
			wantStatus:     http.StatusForbidden,
			wantNextCalled: false,
			wantError:      "admin access required",
		},
		{
			name:           "no claims (unauthenticated) returns 401",
			claims:         nil,
			wantStatus:     http.StatusUnauthorized,
			wantNextCalled: false,
			wantError:      "authentication required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled = false

			req := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
			if tt.claims != nil {
				ctx := context.WithValue(req.Context(), ClaimsContextKey, tt.claims)
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()

			RequireAdmin(next).ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, tt.wantNextCalled, nextCalled)

			if tt.wantError != "" {
				var body map[string]string
				err := json.Unmarshal(rec.Body.Bytes(), &body)
				require.NoError(t, err)
				assert.Equal(t, tt.wantError, body["error"])
			}
		})
	}
}
