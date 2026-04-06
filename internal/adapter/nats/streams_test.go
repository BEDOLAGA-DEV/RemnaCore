package nats_test

import (
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"

	natsadapter "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/nats"
)

func TestStreamConfigs_DedupWindow(t *testing.T) {
	// All non-INFRA streams must have the explicit dedup window set.
	configs := natsadapter.StreamConfigs()

	// Build a lookup by stream name for readable assertions.
	byName := make(map[string]jetstream.StreamConfig, len(configs))
	for _, cfg := range configs {
		byName[cfg.Name] = cfg
	}

	t.Run("non-infra streams have DedupWindow set", func(t *testing.T) {
		nonInfraStreams := []string{
			natsadapter.StreamIdentity,
			natsadapter.StreamBilling,
			natsadapter.StreamRemnawave,
			natsadapter.StreamPayment,
			natsadapter.StreamReseller,
			natsadapter.StreamPlugins,
			natsadapter.StreamDLQ,
		}

		for _, name := range nonInfraStreams {
			t.Run(name, func(t *testing.T) {
				cfg, ok := byName[name]
				assert.True(t, ok, "stream %s must exist in StreamConfigs", name)
				assert.Equal(t, natsadapter.DedupWindow, cfg.Duplicates,
					"stream %s must have Duplicates set to DedupWindow", name)
			})
		}
	})

	t.Run("INFRA stream does not have DedupWindow set", func(t *testing.T) {
		cfg, ok := byName[natsadapter.StreamInfra]
		assert.True(t, ok, "INFRA stream must exist in StreamConfigs")
		assert.Zero(t, cfg.Duplicates,
			"INFRA stream (MemoryStorage) must not have an explicit dedup window")
	})

	t.Run("DedupWindow is 10 minutes", func(t *testing.T) {
		assert.Equal(t, 10*60_000_000_000, int(natsadapter.DedupWindow),
			"DedupWindow must be 10 minutes in nanoseconds")
	})
}
