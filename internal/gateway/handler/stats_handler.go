package handler

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// statsSQL computes dashboard aggregates in a single round-trip.
const statsSQL = `
SELECT
    (SELECT count(*) FROM identity.platform_users)                               AS total_users,
    (SELECT count(*) FROM identity.sessions WHERE expires_at > now())            AS active_sessions,
    (SELECT count(*) FROM billing.subscriptions WHERE status = 'active')         AS active_subs,
    (SELECT count(*) FROM billing.subscriptions WHERE status = 'cancelled')      AS cancelled_subs,
    (SELECT count(*) FROM billing.subscriptions WHERE status = 'paused')         AS paused_subs,
    (SELECT count(*) FROM billing.subscriptions WHERE status = 'pending')        AS pending_subs,
    (SELECT count(*) FROM billing.subscriptions)                                 AS total_subs,
    (SELECT coalesce(sum(total_amount), 0) FROM billing.invoices WHERE status = 'paid') AS total_revenue
`

// DashboardStats is the response shape for GET /api/admin/stats.
type DashboardStats struct {
	TotalUsers     int64 `json:"total_users"`
	ActiveSessions int64 `json:"active_sessions"`
	ActiveSubs     int64 `json:"active_subs"`
	CancelledSubs  int64 `json:"cancelled_subs"`
	PausedSubs     int64 `json:"paused_subs"`
	PendingSubs    int64 `json:"pending_subs"`
	TotalSubs      int64 `json:"total_subs"`
	TotalRevenue   int64 `json:"total_revenue"`
}

// StatsHandler serves the admin dashboard statistics endpoint.
type StatsHandler struct {
	pool *pgxpool.Pool
}

// NewStatsHandler creates a StatsHandler.
func NewStatsHandler(pool *pgxpool.Pool) *StatsHandler {
	return &StatsHandler{pool: pool}
}

// GetStats handles GET /api/admin/stats — returns dashboard aggregates
// computed server-side in a single SQL query.
func (h *StatsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	var s DashboardStats
	if err := h.pool.QueryRow(r.Context(), statsSQL).Scan(
		&s.TotalUsers,
		&s.ActiveSessions,
		&s.ActiveSubs,
		&s.CancelledSubs,
		&s.PausedSubs,
		&s.PendingSubs,
		&s.TotalSubs,
		&s.TotalRevenue,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compute stats")
		return
	}

	writeJSON(w, http.StatusOK, s)
}

