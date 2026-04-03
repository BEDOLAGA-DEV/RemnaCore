package nats_test

import (
	"testing"
	"time"

	natsadapter "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/nats"
	"github.com/stretchr/testify/assert"
)

func TestOutboxRelayConstants(t *testing.T) {
	t.Run("relay interval is positive", func(t *testing.T) {
		assert.Greater(t, natsadapter.OutboxRelayInterval, time.Duration(0))
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
	// NewOutboxRelay should not panic with nil dependencies (constructor only
	// assigns fields).
	relay := natsadapter.NewOutboxRelay(nil, nil, nil)
	assert.NotNil(t, relay)
}
