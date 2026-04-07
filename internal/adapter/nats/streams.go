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
)

// DedupWindow is the JetStream message deduplication window for durable
// streams. Set to 1 hour to cover relay lag scenarios where MarkPublishedBatch
// rolls back after NATS publish success. The previous 10-minute window was too
// short — if the relay falls behind by more than 10 minutes, the dedup window
// expires and retransmissions are no longer deduplicated. One hour covers
// realistic lag scenarios without significant NATS memory overhead: each dedup
// entry is approximately 64 bytes (msg ID), so 10K events/hour costs ~640KB.
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
			Name:       StreamDLQ,
			Subjects:   []string{"dlq.>"},
			Storage:    jetstream.FileStorage,
			MaxAge:     RetentionMonth,
			Duplicates: DedupWindow,
		},
	}
}
