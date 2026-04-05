package nats_test

import (
	"testing"
	"time"

	natsadapter "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/nats"
	"github.com/stretchr/testify/assert"
)

func TestOutboxRelayConstants(t *testing.T) {
	t.Run("relay base interval is positive", func(t *testing.T) {
		assert.Greater(t, natsadapter.OutboxRelayBaseInterval, time.Duration(0))
	})

	t.Run("relay max interval >= base interval", func(t *testing.T) {
		assert.GreaterOrEqual(t, natsadapter.OutboxRelayMaxInterval, natsadapter.OutboxRelayBaseInterval)
	})

	t.Run("batch size is positive", func(t *testing.T) {
		assert.Greater(t, natsadapter.OutboxRelayBatchSize, 0)
	})

	t.Run("cleanup interval is positive", func(t *testing.T) {
		assert.Greater(t, natsadapter.OutboxCleanupInterval, time.Duration(0))
	})

	t.Run("retention period is at least one day", func(t *testing.T) {
		assert.GreaterOrEqual(t, natsadapter.OutboxRetentionPeriod, 24*time.Hour)
	})
}

func TestNewOutboxRelay(t *testing.T) {
	t.Run("nil dependencies do not panic", func(t *testing.T) {
		// NewOutboxRelay should not panic with nil dependencies (constructor only
		// assigns fields).
		relay := natsadapter.NewOutboxRelay(nil, nil, nil, nil, natsadapter.MinOutboxRelayWorkers)
		assert.NotNil(t, relay)
	})

	t.Run("worker count below minimum is clamped", func(t *testing.T) {
		tests := []struct {
			name  string
			input int
		}{
			{name: "zero", input: 0},
			{name: "negative", input: -5},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				relay := natsadapter.NewOutboxRelay(nil, nil, nil, nil, tt.input)
				assert.NotNil(t, relay)
			})
		}
	})

	t.Run("explicit worker count is accepted", func(t *testing.T) {
		relay := natsadapter.NewOutboxRelay(nil, nil, nil, nil, 4)
		assert.NotNil(t, relay)
	})
}
