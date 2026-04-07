package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/health"
)

// --- Test doubles ---

// stubChecker implements health.Checker for tests.
type stubChecker struct {
	check health.ComponentCheck
}

func (s *stubChecker) HealthCheck(_ context.Context) health.ComponentCheck { return s.check }

// newTestHealthHandler creates a HealthHandler with the given stub checkers.
func newTestHealthHandler(checks ...health.ComponentCheck) *HealthHandler {
	checkers := make([]health.Checker, 0, len(checks))
	for _, c := range checks {
		checkers = append(checkers, &stubChecker{check: c})
	}
	return NewHealthHandler(checkers)
}

// readyzResponse is the expected JSON shape returned by Readyz.
type readyzResponse struct {
	Status string                 `json:"status"`
	Checks []health.ComponentCheck `json:"checks"`
}

// --- Healthz tests ---

func TestHealthz_AlwaysReturns200(t *testing.T) {
	h := newTestHealthHandler()

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
	tests := []struct {
		name       string
		checks     []health.ComponentCheck
		wantCode   int
		wantStatus string
	}{
		{
			name: "all healthy",
			checks: []health.ComponentCheck{
				{Name: "postgres", Status: health.StatusHealthy},
				{Name: "valkey", Status: health.StatusHealthy},
				{Name: "nats", Status: health.StatusHealthy},
				{Name: "outbox", Status: health.StatusHealthy},
			},
			wantCode:   http.StatusOK,
			wantStatus: statusReady,
		},
		{
			name: "one component degraded",
			checks: []health.ComponentCheck{
				{Name: "postgres", Status: health.StatusHealthy},
				{Name: "valkey", Status: health.StatusHealthy},
				{Name: "nats", Status: health.StatusDegraded, Message: "reconnecting"},
				{Name: "outbox", Status: health.StatusHealthy},
			},
			wantCode:   http.StatusOK,
			wantStatus: statusDegraded,
		},
		{
			name: "one component unhealthy",
			checks: []health.ComponentCheck{
				{Name: "postgres", Status: health.StatusUnhealthy},
				{Name: "valkey", Status: health.StatusHealthy},
				{Name: "nats", Status: health.StatusHealthy},
				{Name: "outbox", Status: health.StatusHealthy},
			},
			wantCode:   http.StatusServiceUnavailable,
			wantStatus: statusNotReady,
		},
		{
			name: "unhealthy overrides degraded",
			checks: []health.ComponentCheck{
				{Name: "postgres", Status: health.StatusDegraded, Message: "pool utilisation 92%"},
				{Name: "valkey", Status: health.StatusUnhealthy},
				{Name: "nats", Status: health.StatusHealthy},
			},
			wantCode:   http.StatusServiceUnavailable,
			wantStatus: statusNotReady,
		},
		{
			name: "outbox backlog degraded",
			checks: []health.ComponentCheck{
				{Name: "postgres", Status: health.StatusHealthy},
				{Name: "valkey", Status: health.StatusHealthy},
				{Name: "nats", Status: health.StatusHealthy},
				{Name: "outbox", Status: health.StatusDegraded, Message: "backlog: 12000 events"},
			},
			wantCode:   http.StatusOK,
			wantStatus: statusDegraded,
		},
		{
			name: "all unhealthy",
			checks: []health.ComponentCheck{
				{Name: "postgres", Status: health.StatusUnhealthy},
				{Name: "valkey", Status: health.StatusUnhealthy},
				{Name: "nats", Status: health.StatusUnhealthy},
			},
			wantCode:   http.StatusServiceUnavailable,
			wantStatus: statusNotReady,
		},
		{
			name:       "no checkers",
			checks:     nil,
			wantCode:   http.StatusOK,
			wantStatus: statusReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHealthHandler(tt.checks...)

			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			rec := httptest.NewRecorder()

			h.Readyz(rec, req)

			assert.Equal(t, tt.wantCode, rec.Code)

			var resp readyzResponse
			err := json.NewDecoder(rec.Body).Decode(&resp)
			require.NoError(t, err)

			assert.Equal(t, tt.wantStatus, resp.Status)
			assert.Len(t, resp.Checks, len(tt.checks))
		})
	}
}

func TestReadyz_DoesNotLeakErrorDetails(t *testing.T) {
	// Unhealthy components should never expose internal error messages.
	h := newTestHealthHandler(
		health.ComponentCheck{Name: "postgres", Status: health.StatusUnhealthy},
		health.ComponentCheck{Name: "valkey", Status: health.StatusHealthy},
		health.ComponentCheck{Name: "nats", Status: health.StatusHealthy},
	)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	h.Readyz(rec, req)

	body := rec.Body.String()
	assert.NotContains(t, body, "FATAL")
	assert.NotContains(t, body, "password")
	assert.Contains(t, body, string(health.StatusUnhealthy))
}

func TestReadyz_DegradedReturns200(t *testing.T) {
	// Degraded state should return 200, not 503. Kubernetes keeps the pod
	// running but operators can monitor the degraded signal.
	h := newTestHealthHandler(
		health.ComponentCheck{Name: "nats", Status: health.StatusDegraded, Message: "reconnecting"},
	)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	h.Readyz(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp readyzResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, statusDegraded, resp.Status)
}
