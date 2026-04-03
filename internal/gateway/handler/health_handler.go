package handler

import "net/http"

// HealthHandler serves liveness and readiness probes.
type HealthHandler struct{}

// NewHealthHandler returns a new HealthHandler.
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Healthz responds with a 200 JSON body indicating the service is alive.
func (h *HealthHandler) Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz responds with a 200 JSON body indicating the service is ready to
// accept traffic.
func (h *HealthHandler) Readyz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
