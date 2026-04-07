package nats

import (
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// Stream name constants identify each JetStream stream in the platform.
const (
	StreamIdentity  = "IDENTITY"
	StreamBilling   = "BILLING"
	StreamRemnawave = "REMNAWAVE"
	StreamPayment   = "PAYMENT"
	StreamInfra     = "INFRA"
	StreamReseller  = "RESELLER"
	StreamPlugins   = "PLUGINS"
	StreamDLQ       = "DLQ"
)

// Retention duration constants used by stream configurations.
const (
	RetentionDay   = 24 * time.Hour
	RetentionWeek  = 7 * RetentionDay
	RetentionMonth = 30 * RetentionDay

	// RetentionQuarter is 90 days — used for DLQ where failed events must
	// survive long enough for investigation and manual replay.
	RetentionQuarter = 90 * RetentionDay
)

// DedupWindow is the JetStream message deduplication window.
//
// Set to 60 minutes to cover relay lag scenarios:
//   - If MarkPublishedBatch succeeds but TX COMMIT fails, events are
//     re-published on next tick. Dedup prevents consumer duplicates.
//   - If the relay falls behind by more than DedupWindow, duplicates MAY
//     reach consumers. Consumer-side IdempotencyChecker provides
//     additional protection for billing events.
//
// Memory cost: ~64 bytes per msg ID.
//   - At 10K events/hour = ~640KB/hour.
//   - At 100K events/hour = ~6.4MB -- well within NATS server capacity.
//
// If the relay consistently lags > 60 minutes, this indicates a systemic
// issue that should trigger the OutboxBackpressureThreshold alert, not be
// masked by a larger dedup window.
//
// The default JetStream dedup window (2 minutes) is far too short for the
// outbox relay circuit breaker backoff (up to 60s) plus deploy time.
const DedupWindow = 60 * time.Minute

// StreamConfigs returns every JetStream stream configuration the platform
// requires. EnsureStreams iterates this slice on startup to create or update
// each stream idempotently.
func StreamConfigs() []jetstream.StreamConfig {
	return []jetstream.StreamConfig{
		{
			Name:       StreamIdentity,
			Subjects:   []string{"user.>"},
			Storage:    jetstream.FileStorage,
			MaxAge:     RetentionWeek,
			Duplicates: DedupWindow,
		},
		{
			Name:       StreamBilling,
			Subjects:   []string{"invoice.>", "subscription.>", "family.>"},
			Storage:    jetstream.FileStorage,
			MaxAge:     RetentionMonth,
			Duplicates: DedupWindow,
		},
		{
			Name:       StreamRemnawave,
			Subjects:   []string{"remnawave.>", "binding.>"},
			Storage:    jetstream.FileStorage,
			MaxAge:     RetentionWeek,
			Duplicates: DedupWindow,
		},
		{
			Name:       StreamPayment,
			Subjects:   []string{"payment.>"},
			Storage:    jetstream.FileStorage,
			MaxAge:     RetentionMonth,
			Duplicates: DedupWindow,
		},
		{
			Name:     StreamInfra,
			Subjects: []string{"infra.>", "node.>"},
			Storage:  jetstream.MemoryStorage,
			MaxAge:   RetentionDay,
		},
		{
			Name:       StreamReseller,
			Subjects:   []string{"reseller.>"},
			Storage:    jetstream.FileStorage,
			MaxAge:     RetentionMonth,
			Duplicates: DedupWindow,
		},
		{
			Name:       StreamPlugins,
			Subjects:   []string{"plugin.>"},
			Storage:    jetstream.FileStorage,
			MaxAge:     RetentionWeek,
			Duplicates: DedupWindow,
		},
		{
			// DLQ retention is longer than domain streams (90 days vs 30 days).
			// Failed events that are not replayed within 30 days would be silently
			// lost at domain stream retention. DLQ preserves them for investigation.
			Name:       StreamDLQ,
			Subjects:   []string{"dlq.>"},
			Storage:    jetstream.FileStorage,
			MaxAge:     RetentionQuarter,
			Duplicates: DedupWindow,
		},
	}
}
