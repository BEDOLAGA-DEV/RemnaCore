package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test doubles ---

// stubDBPinger implements dbPinger for tests.
type stubDBPinger struct {
	err error
}

func (s *stubDBPinger) Ping(_ context.Context) error { return s.err }

// stubValkeyPinger implements valkeyPinger for tests.
type stubValkeyPinger struct {
	err error
}

func (s *stubValkeyPinger) Ping(_ context.Context) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(context.Background())
	if s.err != nil {
		cmd.SetErr(s.err)
	} else {
		cmd.SetVal("PONG")
	}
	return cmd
}

// stubNATSChecker implements natsChecker for tests.
type stubNATSChecker struct {
	connected bool
}

func (s *stubNATSChecker) IsConnected() bool { return s.connected }

// newTestHealthHandler creates a HealthHandler with stub dependencies.
func newTestHealthHandler(dbErr, valkeyErr error, natsConnected bool) *HealthHandler {
	return &HealthHandler{
		db:     &stubDBPinger{err: dbErr},
		valkey: &stubValkeyPinger{err: valkeyErr},
		nats:   &stubNATSChecker{connected: natsConnected},
	}
}

// readyzResponse is the expected JSON shape returned by Readyz.
type readyzResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// --- Healthz tests ---

func TestHealthz_AlwaysReturns200(t *testing.T) {
	h := newTestHealthHandler(nil, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	h.Healthz(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]string
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp["status"])
}

// --- Readyz tests ---

func TestReadyz(t *testing.T) {
	errDown := errors.New("connection refused")

	tests := []struct {
		name           string
		dbErr          error
		valkeyErr      error
		natsConnected  bool
		natsNil        bool
		wantCode       int
		wantStatus     string
		wantPostgres   string
		wantValkey     string
		wantNATS       string
	}{
		{
			name:          "all healthy",
			dbErr:         nil,
			valkeyErr:     nil,
			natsConnected: true,
			wantCode:      http.StatusOK,
			wantStatus:    statusReady,
			wantPostgres:  checkHealthy,
			wantValkey:    checkHealthy,
			wantNATS:      checkHealthy,
		},
		{
			name:          "postgres down",
			dbErr:         errDown,
			valkeyErr:     nil,
			natsConnected: true,
			wantCode:      http.StatusServiceUnavailable,
			wantStatus:    statusNotReady,
			wantPostgres:  checkUnhealthy,
			wantValkey:    checkHealthy,
			wantNATS:      checkHealthy,
		},
		{
			name:          "valkey down",
			dbErr:         nil,
			valkeyErr:     errDown,
			natsConnected: true,
			wantCode:      http.StatusServiceUnavailable,
			wantStatus:    statusNotReady,
			wantPostgres:  checkHealthy,
			wantValkey:    checkUnhealthy,
			wantNATS:      checkHealthy,
		},
		{
			name:          "nats disconnected",
			dbErr:         nil,
			valkeyErr:     nil,
			natsConnected: false,
			wantCode:      http.StatusServiceUnavailable,
			wantStatus:    statusNotReady,
			wantPostgres:  checkHealthy,
			wantValkey:    checkHealthy,
			wantNATS:      checkUnhealthy,
		},
		{
			name:          "nats nil conn",
			dbErr:         nil,
			valkeyErr:     nil,
			natsConnected: false,
			natsNil:       true,
			wantCode:      http.StatusServiceUnavailable,
			wantStatus:    statusNotReady,
			wantPostgres:  checkHealthy,
			wantValkey:    checkHealthy,
			wantNATS:      checkUnhealthy,
		},
		{
			name:          "all down",
			dbErr:         errDown,
			valkeyErr:     errDown,
			natsConnected: false,
			wantCode:      http.StatusServiceUnavailable,
			wantStatus:    statusNotReady,
			wantPostgres:  checkUnhealthy,
			wantValkey:    checkUnhealthy,
			wantNATS:      checkUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h *HealthHandler
			if tt.natsNil {
				h = &HealthHandler{
					db:     &stubDBPinger{err: tt.dbErr},
					valkey: &stubValkeyPinger{err: tt.valkeyErr},
					nats:   nil,
				}
			} else {
				h = newTestHealthHandler(tt.dbErr, tt.valkeyErr, tt.natsConnected)
			}

			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			rec := httptest.NewRecorder()

			h.Readyz(rec, req)

			assert.Equal(t, tt.wantCode, rec.Code)

			var resp readyzResponse
			err := json.NewDecoder(rec.Body).Decode(&resp)
			require.NoError(t, err)

			assert.Equal(t, tt.wantStatus, resp.Status)
			assert.Equal(t, tt.wantPostgres, resp.Checks["postgres"])
			assert.Equal(t, tt.wantValkey, resp.Checks["valkey"])
			assert.Equal(t, tt.wantNATS, resp.Checks["nats"])
		})
	}
}

func TestReadyz_DoesNotLeakErrorDetails(t *testing.T) {
	secretErr := errors.New("FATAL: password authentication failed for user \"remna\"")
	h := newTestHealthHandler(secretErr, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	h.Readyz(rec, req)

	body := rec.Body.String()
	assert.NotContains(t, body, "FATAL")
	assert.NotContains(t, body, "password")
	assert.NotContains(t, body, "remna")
	assert.Contains(t, body, checkUnhealthy)
}
