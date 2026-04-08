package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/health"
)

// ReadyzTimeout is the maximum time allowed for readiness checks against
// external dependencies (Postgres, Valkey, NATS, Outbox).
const ReadyzTimeout = 2 * time.Second

// Readiness response status strings. Internal error details are never exposed
// to avoid leaking infrastructure information.
const (
	statusReady    = "ready"
	statusDegraded = "degraded"
	statusNotReady = "not ready"
)

// HealthHandler serves liveness and readiness probes.
type HealthHandler struct {
	checkers []health.Checker
}

// NewHealthHandler returns a new HealthHandler that aggregates component
// health checks during readiness probes. Checkers are injected via Fx group
// "health.checkers".
func NewHealthHandler(checkers []health.Checker) *HealthHandler {
	return &HealthHandler{checkers: checkers}
}

// Healthz responds with a 200 JSON body indicating the service is alive.
// This is the liveness probe -- it must stay lightweight with no dependency
// checks so the kubelet can distinguish "process alive" from "ready to serve".
func (h *HealthHandler) Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz runs all registered health checkers and reports per-component status.
// Returns 200 when all components are healthy or degraded, 503 when any
// component is unhealthy. Degraded is indicated in the response body but does
// NOT trigger 503 -- Kubernetes keeps the pod but operators can monitor the
// degraded signal.
//
// Response body:
//
//	{
//	  "status": "ready" | "degraded" | "not ready",
//	  "checks": [ { "name": "postgres", "status": "healthy" }, ... ]
//	}
func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), ReadyzTimeout)
	defer cancel()

	checks := make([]health.ComponentCheck, 0, len(h.checkers))
	for _, c := range h.checkers {
		start := time.Now()
		check := c.HealthCheck(ctx)
		check.LatencyMs = float64(time.Since(start).Microseconds()) / 1000.0
		checks = append(checks, check)
	}

	overall := health.Aggregate(checks)

	code := http.StatusOK
	readyStatus := statusReady

	switch overall {
	case health.StatusUnhealthy:
		code = http.StatusServiceUnavailable
		readyStatus = statusNotReady
	case health.StatusDegraded:
		readyStatus = statusDegraded
	}

	writeJSON(w, code, map[string]any{
		"status": readyStatus,
		"checks": checks,
	})
}
