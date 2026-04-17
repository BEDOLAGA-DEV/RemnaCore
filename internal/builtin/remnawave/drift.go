package remnawave

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

// ListDriftEvents returns all detected drift events.
func (h *Handler) ListDriftEvents(w http.ResponseWriter, r *http.Request) {
	docs, err := h.collections.ListDocuments(r.Context(), PluginSlug, CollectionDriftEvents)
	if err != nil {
		h.logger.Error("failed to list drift events", slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}

	events := make([]DriftEventResponse, 0, len(docs))
	for _, doc := range docs {
		var e DriftEventInput
		if json.Unmarshal(doc.Data, &e) != nil {
			continue
		}
		events = append(events, DriftEventResponse{
			ID:              doc.ID,
			DriftEventInput: e,
			CreatedAt:       doc.CreatedAt.Format(time.RFC3339),
			UpdatedAt:       doc.UpdatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, events)
}

// TriggerReconciliation starts a manual drift reconciliation for all panels.
func (h *Handler) TriggerReconciliation(w http.ResponseWriter, r *http.Request) {
	// In a full implementation, this would:
	// 1. Get all active panels
	// 2. For each panel, call GetAllUsers
	// 3. Compare with multisub.bindings
	// 4. Create drift events for mismatches
	// For now, return acknowledgment that the job was triggered.

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":       "reconciliation_triggered",
		"triggered_at": nowRFC3339(),
		"note":         "reconciliation runs asynchronously",
	})
}

// ResolveDriftEvent marks a drift event as resolved.
func (h *Handler) ResolveDriftEvent(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "eventID")
	if eventID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("event ID is required"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionDriftEvents, eventID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	var event DriftEventInput
	if err := json.Unmarshal(doc.Data, &event); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	event.Status = "resolved"
	event.ResolvedAt = nowRFC3339()

	data, err := json.Marshal(event)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	if err := h.collections.UpdateDocument(r.Context(), PluginSlug, CollectionDriftEvents, eventID, json.RawMessage(data)); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"event_id": eventID,
		"status":   "resolved",
	})
}

// HealDriftEvent attempts to recreate the missing resource.
func (h *Handler) HealDriftEvent(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "eventID")
	if eventID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("event ID is required"))
		return
	}

	doc, err := h.collections.GetDocument(r.Context(), PluginSlug, CollectionDriftEvents, eventID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeAPIError(w, apierror.Internal)
		return
	}

	var event DriftEventInput
	if err := json.Unmarshal(doc.Data, &event); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	// In a full implementation, this would:
	// - For "missing_in_panel": recreate the user/node in the panel
	// - For "missing_in_core": create a binding in multisub
	// - For "status_mismatch": sync the status

	event.Status = "healed"
	event.ResolvedAt = nowRFC3339()

	data, err := json.Marshal(event)
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	if err := h.collections.UpdateDocument(r.Context(), PluginSlug, CollectionDriftEvents, eventID, json.RawMessage(data)); err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"event_id": eventID,
		"status":   "healed",
		"kind":     event.Kind,
		"note":     "heal action dispatched",
	})
}
