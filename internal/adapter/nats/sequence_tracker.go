package nats

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/observability"
)

// sequenceTrackerTTL is the duration after which an idle entity's sequence
// entry is eligible for eviction. This prevents unbounded memory growth when
// many distinct entities produce events over time but only a subset remains
// active.
const sequenceTrackerTTL = 30 * time.Minute

// sequenceTrackerEvictInterval is how often the background eviction loop runs.
const sequenceTrackerEvictInterval = 5 * time.Minute

// sequenceEntry holds the last-seen sequence number and a timestamp for TTL
// eviction.
type sequenceEntry struct {
	seq        int64
	lastUpdate int64 // unix nano
}

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
//
// Entries are evicted after sequenceTrackerTTL of inactivity to prevent
// unbounded memory growth.
type sequenceTracker struct {
	mu       sync.Mutex
	lastSeen map[string]sequenceEntry
	logger   *slog.Logger
	metrics  *observability.Metrics
}

// newSequenceTracker creates a sequenceTracker with the given dependencies.
// metrics may be nil (safe for tests).
func newSequenceTracker(logger *slog.Logger, metrics *observability.Metrics) *sequenceTracker {
	return &sequenceTracker{
		lastSeen: make(map[string]sequenceEntry),
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

	now := time.Now().UnixNano()

	t.mu.Lock()
	defer t.mu.Unlock()

	if entry, ok := t.lastSeen[entityID]; ok {
		gap := seq - entry.seq - 1
		if gap > 0 {
			t.logger.Warn("outbox sequence gap detected",
				slog.String("entity_id", entityID),
				slog.Int64("last_seen", entry.seq),
				slog.Int64("current", seq),
				slog.Int64("gap", gap),
			)
			if t.metrics != nil {
				t.metrics.OutboxSequenceGaps.Inc()
			}
		}
	}

	t.lastSeen[entityID] = sequenceEntry{seq: seq, lastUpdate: now}
}

// evictStale removes entries that have not been updated within
// sequenceTrackerTTL. This is called periodically by the consumer's
// background eviction loop to bound memory usage.
func (t *sequenceTracker) evictStale() int {
	cutoff := time.Now().Add(-sequenceTrackerTTL).UnixNano()

	t.mu.Lock()
	defer t.mu.Unlock()

	var evicted int
	for key, entry := range t.lastSeen {
		if entry.lastUpdate < cutoff {
			delete(t.lastSeen, key)
			evicted++
		}
	}

	return evicted
}

// runEviction periodically evicts stale entries until the context is
// cancelled. It is intended to be called as a background goroutine from the
// consumer's Start method.
func (t *sequenceTracker) runEviction(ctx context.Context) {
	ticker := time.NewTicker(sequenceTrackerEvictInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			evicted := t.evictStale()
			if evicted > 0 {
				t.mu.Lock()
				remaining := len(t.lastSeen)
				t.mu.Unlock()

				t.logger.Debug("evicted stale sequence tracker entries",
					slog.Int("evicted", evicted),
					slog.Int("remaining", remaining),
				)
			}
		}
	}
}
