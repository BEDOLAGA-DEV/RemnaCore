package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// listRecentActivitySQL reads the newest outbox events for the admin activity
// feed. The projection is intentionally narrow (no payload) to keep the scan
// over the range-partitioned outbox cheap; parsePagination caps the limit.
const listRecentActivitySQL = `
SELECT id, event_type, created_at
FROM public.outbox
ORDER BY created_at DESC
LIMIT $1
`

const (
	activityLevelInfo = "info"
	activityLevelOK   = "ok"
	activityLevelWarn = "warn"
)

// activityLevels maps known domain event types to a log level. Unknown /
// open-ended types (plugin.*, remnawave.*) fall through to "info".
var activityLevels = map[string]string{
	"subscription.created":       activityLevelOK,
	"subscription.activated":     activityLevelOK,
	"subscription.renewed":       activityLevelOK,
	"subscription.resumed":       activityLevelOK,
	"subscription.trial_started": activityLevelOK,
	"subscription.upgraded":      activityLevelOK,
	"invoice.paid":               activityLevelOK,
	"payment.charge_completed":   activityLevelOK,
	"payment.refund_completed":   activityLevelOK,
	"plan.created":               activityLevelOK,
	"user.registered":            activityLevelOK,
	"user.email_verified":        activityLevelOK,
	"binding.provisioned":        activityLevelOK,

	"subscription.cancelled":   activityLevelWarn,
	"subscription.expired":     activityLevelWarn,
	"subscription.past_due":    activityLevelWarn,
	"subscription.paused":      activityLevelWarn,
	"subscription.downgraded":  activityLevelWarn,
	"invoice.failed":           activityLevelWarn,
	"invoice.refunded":         activityLevelWarn,
	"payment.charge_failed":    activityLevelWarn,
	"binding.failed":           activityLevelWarn,
	"binding.sync_failed":      activityLevelWarn,
	"binding.traffic_exceeded": activityLevelWarn,
	"binding.limited":          activityLevelWarn,
	"binding.disabled":         activityLevelWarn,
	"infra.node_disabled":      activityLevelWarn,
	"infra.node_deleted":       activityLevelWarn,
	"node.health.changed":      activityLevelWarn,
	"plugin.error":             activityLevelWarn,
}

// ActivityEntry is one line in the admin activity / system-log feed.
type ActivityEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}

// ActivityListResponse is the response shape for GET /api/admin/activity.
type ActivityListResponse struct {
	Activity []ActivityEntry `json:"activity"`
}

// ActivityHandler serves the admin activity feed from the transactional outbox.
type ActivityHandler struct {
	pool *pgxpool.Pool
}

// NewActivityHandler creates an ActivityHandler.
func NewActivityHandler(pool *pgxpool.Pool) *ActivityHandler {
	return &ActivityHandler{pool: pool}
}

// ListActivity handles GET /api/admin/activity -- returns the most recent
// platform events (cross-tenant, admin-only) as a terminal-style feed.
func (h *ActivityHandler) ListActivity(w http.ResponseWriter, r *http.Request) {
	limit, _ := parsePagination(r)

	rows, err := h.pool.Query(r.Context(), listRecentActivitySQL, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list activity")
		return
	}
	defer rows.Close()

	entries := make([]ActivityEntry, 0)
	for rows.Next() {
		var e ActivityEntry
		if err := rows.Scan(&e.ID, &e.EventType, &e.Timestamp); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan activity")
			return
		}
		e.Level, e.Message = levelAndMessage(e.EventType)
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to iterate activity")
		return
	}

	writeJSON(w, http.StatusOK, ActivityListResponse{Activity: entries})
}

// levelAndMessage derives a log level and human-readable message from an event
// type. Unknown types default to "info" with a humanized message.
func levelAndMessage(eventType string) (level, message string) {
	level = activityLevels[eventType]
	if level == "" {
		level = activityLevelInfo
	}
	return level, humanizeEventType(eventType)
}

// humanizeEventType turns "subscription.trial_started" into
// "Subscription trial started". Safe on empty input.
func humanizeEventType(s string) string {
	if s == "" {
		return s
	}
	spaced := strings.NewReplacer(".", " ", "_", " ").Replace(s)
	r := []rune(spaced)
	r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	return string(r)
}
