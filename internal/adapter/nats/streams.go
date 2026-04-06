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
// streams. Set to 10 minutes to cover typical deploy/restart cycles where the
// outbox relay may re-publish events with the same Nats-Msg-Id after recovery.
// The default JetStream dedup window (2 minutes) is too short for the outbox
// relay circuit breaker backoff (up to 60s) plus deploy time.
const DedupWindow = 10 * time.Minute

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
