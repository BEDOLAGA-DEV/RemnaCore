package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	nc "github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

// ReadyzTimeout is the maximum time allowed for readiness checks against
// external dependencies (Postgres, Valkey, NATS).
const ReadyzTimeout = 2 * time.Second

// Readiness check result constants. Internal error details are never exposed
// to avoid leaking infrastructure information.
const (
	checkHealthy   = "healthy"
	checkUnhealthy = "unhealthy"
	statusReady    = "ready"
	statusNotReady = "not ready"
)

// dbPinger is the subset of *pgxpool.Pool used by HealthHandler.
type dbPinger interface {
	Ping(ctx context.Context) error
}

// valkeyPinger is the subset of *redis.Client used by HealthHandler.
type valkeyPinger interface {
	Ping(ctx context.Context) *redis.StatusCmd
}

// natsChecker is the subset of *nc.Conn used by HealthHandler.
type natsChecker interface {
	IsConnected() bool
}

// HealthHandler serves liveness and readiness probes.
type HealthHandler struct {
	db     dbPinger
	valkey valkeyPinger
	nats   natsChecker
}

// NewHealthHandler returns a new HealthHandler that pings each dependency
// during readiness checks.
func NewHealthHandler(db *pgxpool.Pool, valkey *redis.Client, nats *nc.Conn) *HealthHandler {
	return &HealthHandler{db: db, valkey: valkey, nats: nats}
}

// Healthz responds with a 200 JSON body indicating the service is alive.
// This is the liveness probe -- it must stay lightweight with no dependency
// checks so the kubelet can distinguish "process alive" from "ready to serve".
func (h *HealthHandler) Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz pings Postgres, Valkey, and NATS and reports per-dependency health.
// Returns 200 when all dependencies are reachable, 503 otherwise. Error
// details are intentionally omitted to prevent leaking infrastructure info.
func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), ReadyzTimeout)
	defer cancel()

	status := make(map[string]string, 3)
	allOK := true

	// Postgres
	if err := h.db.Ping(ctx); err != nil {
		status["postgres"] = checkUnhealthy
		allOK = false
	} else {
		status["postgres"] = checkHealthy
	}

	// Valkey
	if err := h.valkey.Ping(ctx).Err(); err != nil {
		status["valkey"] = checkUnhealthy
		allOK = false
	} else {
		status["valkey"] = checkHealthy
	}

	// NATS
	if h.nats == nil || !h.nats.IsConnected() {
		status["nats"] = checkUnhealthy
		allOK = false
	} else {
		status["nats"] = checkHealthy
	}

	code := http.StatusOK
	readyStatus := statusReady
	if !allOK {
		code = http.StatusServiceUnavailable
		readyStatus = statusNotReady
	}

	writeJSON(w, code, map[string]any{
		"status": readyStatus,
		"checks": status,
	})
}
