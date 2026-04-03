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

// StreamConfigs returns every JetStream stream configuration the platform
// requires. EnsureStreams iterates this slice on startup to create or update
// each stream idempotently.
func StreamConfigs() []jetstream.StreamConfig {
	return []jetstream.StreamConfig{
		{
			Name:     StreamIdentity,
			Subjects: []string{"user.>"},
			Storage:  jetstream.FileStorage,
			MaxAge:   RetentionWeek,
		},
		{
			Name:     StreamBilling,
			Subjects: []string{"invoice.>", "subscription.>", "family.>"},
			Storage:  jetstream.FileStorage,
			MaxAge:   RetentionMonth,
		},
		{
			Name:     StreamRemnawave,
			Subjects: []string{"remnawave.>", "binding.>"},
			Storage:  jetstream.FileStorage,
			MaxAge:   RetentionWeek,
		},
		{
			Name:     StreamPayment,
			Subjects: []string{"payment.>"},
			Storage:  jetstream.FileStorage,
			MaxAge:   RetentionMonth,
		},
		{
			Name:    StreamInfra,
			Subjects: []string{"infra.>", "node.>"},
			Storage: jetstream.MemoryStorage,
			MaxAge:  RetentionDay,
		},
		{
			Name:     StreamReseller,
			Subjects: []string{"reseller.>"},
			Storage:  jetstream.FileStorage,
			MaxAge:   RetentionMonth,
		},
		{
			Name:     StreamPlugins,
			Subjects: []string{"plugin.>"},
			Storage:  jetstream.FileStorage,
			MaxAge:   RetentionWeek,
		},
		{
			Name:     StreamDLQ,
			Subjects: []string{"dlq.>"},
			Storage:  jetstream.FileStorage,
			MaxAge:   RetentionMonth,
		},
	}
}
