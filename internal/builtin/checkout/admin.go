package checkout

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
)

// AdminListSessions returns all checkout sessions with optional status filter.
func (h *Handler) AdminListSessions(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, collectionSessions)
	if err != nil {
		h.logger.Error("failed to list sessions", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	statusFilter := r.URL.Query().Get("status")
	sessions := make([]json.RawMessage, 0, len(docs))
	for _, doc := range docs {
		if statusFilter != "" {
			var s CheckoutSession
			if json.Unmarshal(doc.Data, &s) == nil && string(s.Status) != statusFilter {
				continue
			}
		}
		sessions = append(sessions, doc.Data)
	}

	writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions, "total": len(sessions)})
}

// AdminGetSession returns a specific session by ID.
func (h *Handler) AdminGetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("session ID is required"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, collectionSessions, sessionID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, json.RawMessage(doc.Data))
}

// AdminAbandonment returns sessions that are candidates for abandonment recovery.
func (h *Handler) AdminAbandonment(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, collectionSessions)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	oneHourAgo := time.Now().Add(-1 * time.Hour)
	abandoned := make([]json.RawMessage, 0)
	for _, doc := range docs {
		var s CheckoutSession
		if json.Unmarshal(doc.Data, &s) != nil {
			continue
		}
		if s.Status == SessionPending && s.UpdatedAt.Before(oneHourAgo) {
			abandoned = append(abandoned, doc.Data)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"abandoned_sessions": abandoned,
		"total":              len(abandoned),
	})
}

// AdminFunnel returns conversion funnel statistics.
func (h *Handler) AdminFunnel(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, collectionSessions)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	counts := map[string]int{
		"pending":    0,
		"processing": 0,
		"completed":  0,
		"failed":     0,
		"expired":    0,
		"abandoned":  0,
		"cancelled":  0,
	}

	for _, doc := range docs {
		var s CheckoutSession
		if json.Unmarshal(doc.Data, &s) == nil {
			counts[string(s.Status)]++
		}
	}

	total := 0
	for _, c := range counts {
		total += c
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total":  total,
		"counts": counts,
		"conversion_rate": func() float64 {
			if total == 0 {
				return 0
			}
			return float64(counts["completed"]) / float64(total) * 100
		}(),
	})
}

// AdminMethodsBreakdown returns payment method distribution.
func (h *Handler) AdminMethodsBreakdown(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, collectionSessions)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	methods := make(map[string]int)
	for _, doc := range docs {
		var s CheckoutSession
		if json.Unmarshal(doc.Data, &s) != nil {
			continue
		}
		for _, src := range s.Sources {
			methods[src.Kind]++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"methods": methods})
}
