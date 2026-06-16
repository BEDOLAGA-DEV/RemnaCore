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

// metricsSQL computes the derived business metrics in a single round-trip.
// All monetary values are integer cents (pkg/money stores amounts as int64
// cents). MRR normalizes each active subscription's plan price to a monthly
// figure by billing interval (month=base, quarter=base/3, year=base/12). The
// "today" boundary uses the 3-arg date_trunc(..., 'UTC') so it is independent
// of the database session timezone. churn_30d is a cohort approximation, not
// true retention churn: cancelled-in-last-30d over the base that existed 30d
// ago (no full status history is available).
const metricsSQL = `
WITH active_subs AS (
    SELECT s.id, s.plan_id
    FROM billing.subscriptions s
    WHERE s.status = 'active'
),
mrr AS (
    SELECT coalesce(sum(
        CASE p.billing_interval
            WHEN 'month'   THEN p.base_price_amount
            WHEN 'quarter' THEN p.base_price_amount / 3
            WHEN 'year'    THEN p.base_price_amount / 12
            ELSE p.base_price_amount
        END
    ), 0) AS mrr_cents
    FROM active_subs a
    JOIN billing.plans p ON p.id = a.plan_id
),
revenue AS (
    SELECT coalesce(sum(total_amount), 0) AS total_revenue_cents,
           count(DISTINCT user_id)        AS paying_users
    FROM billing.invoices
    WHERE status = 'paid'
),
churn AS (
    SELECT
        (SELECT count(*) FROM billing.subscriptions
           WHERE status = 'cancelled'
             AND cancelled_at IS NOT NULL
             AND cancelled_at >= now() - interval '30 days') AS churned_30d,
        (SELECT count(*) FROM billing.subscriptions
           WHERE created_at <= now() - interval '30 days'
             AND (cancelled_at IS NULL OR cancelled_at >= now() - interval '30 days')) AS base_30d_ago
)
SELECT
    (SELECT mrr_cents FROM mrr)                                                          AS mrr_cents,
    coalesce((SELECT total_revenue_cents FROM revenue)::float8
             / NULLIF((SELECT paying_users FROM revenue), 0), 0)                         AS arpu_cents,
    coalesce((SELECT churned_30d FROM churn)::float8
             / NULLIF((SELECT base_30d_ago FROM churn), 0), 0)                           AS churn_30d,
    (SELECT count(*) FROM billing.subscriptions
        WHERE created_at >= date_trunc('day', now(), 'UTC'))                             AS subs_today,
    (SELECT count(*) FROM billing.subscriptions WHERE status = 'active')                 AS active_subs,
    (SELECT count(*) FROM billing.subscriptions WHERE status = 'cancelled')              AS cancelled_subs,
    (SELECT count(*) FROM billing.subscriptions WHERE status = 'paused')                 AS paused_subs,
    (SELECT count(*) FROM billing.subscriptions WHERE status = 'pending')                AS pending_subs,
    (SELECT count(*) FROM billing.subscriptions)                                         AS total_subs,
    (SELECT total_revenue_cents FROM revenue)                                            AS total_revenue_cents,
    (SELECT paying_users FROM revenue)                                                   AS paying_users
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

// AdminMetrics is the response shape for GET /api/admin/metrics. Monetary
// fields are integer cents, except ARPUCents which is a float (revenue/users
// may be fractional). Churn30d is a ratio in [0,1]. These are global,
// cross-tenant platform aggregates (same isolation model as GET /api/admin/stats).
type AdminMetrics struct {
	MRRCents          int64   `json:"mrr_cents"`
	ARPUCents         float64 `json:"arpu_cents"`
	Churn30d          float64 `json:"churn_30d"`
	SubsToday         int64   `json:"subs_today"`
	ActiveSubs        int64   `json:"active_subs"`
	CancelledSubs     int64   `json:"cancelled_subs"`
	PausedSubs        int64   `json:"paused_subs"`
	PendingSubs       int64   `json:"pending_subs"`
	TotalSubs         int64   `json:"total_subs"`
	TotalRevenueCents int64   `json:"total_revenue_cents"`
	PayingUsers       int64   `json:"paying_users"`
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

// GetMetrics handles GET /api/admin/metrics -- returns derived business metrics
// (MRR, ARPU, churn-30d, subs-today) plus echoed subscription counts, computed
// server-side in a single SQL query. Cross-tenant platform aggregate.
func (h *StatsHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	var m AdminMetrics
	if err := h.pool.QueryRow(r.Context(), metricsSQL).Scan(
		&m.MRRCents,
		&m.ARPUCents,
		&m.Churn30d,
		&m.SubsToday,
		&m.ActiveSubs,
		&m.CancelledSubs,
		&m.PausedSubs,
		&m.PendingSubs,
		&m.TotalSubs,
		&m.TotalRevenueCents,
		&m.PayingUsers,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compute metrics")
		return
	}

	writeJSON(w, http.StatusOK, m)
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
