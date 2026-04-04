package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	nc "github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

// ReadyzTimeout is the maximum time allowed for readiness checks against
// external dependencies (Postgres, Valkey, NATS).
const ReadyzTimeout = 3 * time.Second

// HealthHandler serves liveness and readiness probes.
type HealthHandler struct {
	db     *pgxpool.Pool
	valkey *redis.Client
	nats   *nc.Conn
}

// NewHealthHandler returns a new HealthHandler that pings each dependency
// during readiness checks.
func NewHealthHandler(db *pgxpool.Pool, valkey *redis.Client, nats *nc.Conn) *HealthHandler {
	return &HealthHandler{db: db, valkey: valkey, nats: nats}
}

// Healthz responds with a 200 JSON body indicating the service is alive.
func (h *HealthHandler) Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz pings Postgres, Valkey, and NATS and reports per-dependency health.
// Returns 200 when all dependencies are reachable, 503 otherwise.
func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), ReadyzTimeout)
	defer cancel()

	checks := map[string]error{
		"postgres": h.db.Ping(ctx),
		"valkey":   h.valkey.Ping(ctx).Err(),
	}

	if !h.nats.IsConnected() {
		checks["nats"] = fmt.Errorf("not connected")
	} else {
		checks["nats"] = nil
	}

	allHealthy := true
	status := make(map[string]string, len(checks))
	for name, err := range checks {
		if err != nil {
			allHealthy = false
			status[name] = "unhealthy: " + err.Error()
		} else {
			status[name] = "ok"
		}
	}

	code := http.StatusOK
	if !allHealthy {
		code = http.StatusServiceUnavailable
	}

	readyStatus := "ready"
	if !allHealthy {
		readyStatus = "not ready"
	}

	writeJSON(w, code, map[string]interface{}{
		"status": readyStatus,
		"checks": status,
	})
}
