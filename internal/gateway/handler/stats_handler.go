package handler

import (
	"net/http"
	"time"

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

// listActiveSessionsSQL returns active sessions joined with user email.
const listActiveSessionsSQL = `
SELECT s.id, s.user_id, u.email, s.ip_address, s.user_agent, s.expires_at, s.created_at
FROM identity.sessions s
JOIN identity.platform_users u ON u.id = s.user_id
WHERE s.expires_at > now()
ORDER BY s.created_at DESC
LIMIT $1 OFFSET $2
`

// countActiveSessionsSQL counts non-expired sessions.
const countActiveSessionsSQL = `SELECT count(*) FROM identity.sessions WHERE expires_at > now()`

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

// SessionListResponse is the response shape for GET /api/admin/sessions.
type SessionListResponse struct {
	Sessions []SessionEntry `json:"sessions"`
	Total    int64          `json:"total"`
}

// SessionEntry represents a single active session in the admin sessions list.
type SessionEntry struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	UserEmail string    `json:"user_email"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// StatsHandler serves the admin dashboard statistics endpoint.
type StatsHandler struct {
	pool *pgxpool.Pool
}

// NewStatsHandler creates a StatsHandler.
func NewStatsHandler(pool *pgxpool.Pool) *StatsHandler {
	return &StatsHandler{pool: pool}
}

// GetStats handles GET /api/admin/stats -- returns dashboard aggregates
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

// ListSessions handles GET /api/admin/sessions -- returns a paginated list
// of active sessions with user email, IP address, and user-agent.
func (h *StatsHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	var total int64
	if err := h.pool.QueryRow(r.Context(), countActiveSessionsSQL).Scan(&total); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count sessions")
		return
	}

	rows, err := h.pool.Query(r.Context(), listActiveSessionsSQL, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}
	defer rows.Close()

	entries := make([]SessionEntry, 0)
	for rows.Next() {
		var e SessionEntry
		if err := rows.Scan(
			&e.ID,
			&e.UserID,
			&e.UserEmail,
			&e.IPAddress,
			&e.UserAgent,
			&e.ExpiresAt,
			&e.CreatedAt,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan session")
			return
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to iterate sessions")
		return
	}

	writeJSON(w, http.StatusOK, SessionListResponse{
		Sessions: entries,
		Total:    total,
	})
}
