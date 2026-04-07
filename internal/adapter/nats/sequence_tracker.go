package nats

import (
	"log/slog"
	"sync"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/observability"
)

// sequenceTracker tracks the last-seen outbox sequence number per entity for
// consumer-side gap detection. This is observability-only — it logs warnings
// and increments a Prometheus counter when gaps are detected, but never blocks
// event processing.
//
// A gap (seq > lastSeen+1) may indicate:
//   - Out-of-order delivery (events for the same entity arrived via different
//     relay workers before worker affinity was enabled)
//   - Lost events (partial commit edge case in the outbox relay)
//   - Consumer restart (lastSeen map was cleared)
//
// The tracker is designed for use with per-entity serial processing (via
// entityLocks), so concurrent access for the same entity_id is not expected
// but guarded against with a mutex.
type sequenceTracker struct {
	mu       sync.Mutex
	lastSeen map[string]int64 // entity_id -> last sequence number
	logger   *slog.Logger
	metrics  *observability.Metrics
}

// newSequenceTracker creates a sequenceTracker with the given dependencies.
// metrics may be nil (safe for tests).
func newSequenceTracker(logger *slog.Logger, metrics *observability.Metrics) *sequenceTracker {
	return &sequenceTracker{
		lastSeen: make(map[string]int64),
		logger:   logger,
		metrics:  metrics,
	}
}

// checkAndUpdate records the sequence number for the given entity and checks
// for gaps. If a gap is detected (seq > lastSeen+1), a warning is logged and
// the OutboxSequenceGaps counter is incremented.
//
// Sequence numbers <= 0 are ignored (backward compat: events without sequence
// numbers). The tracker never blocks processing — it is purely observational.
func (t *sequenceTracker) checkAndUpdate(entityID string, seq int64) {
	if seq <= 0 || entityID == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if last, ok := t.lastSeen[entityID]; ok {
		gap := seq - last - 1
		if gap > 0 {
			t.logger.Warn("outbox sequence gap detected",
				slog.String("entity_id", entityID),
				slog.Int64("last_seen", last),
				slog.Int64("current", seq),
				slog.Int64("gap", gap),
			)
			if t.metrics != nil {
				t.metrics.OutboxSequenceGaps.Inc()
			}
		}
	}

	t.lastSeen[entityID] = seq
}
