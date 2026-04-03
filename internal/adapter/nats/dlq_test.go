package nats

import (
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/stretchr/testify/assert"
)

func TestGetRetryCount_Empty(t *testing.T) {
	msg := message.NewMessage("test-id", nil)
	assert.Equal(t, 0, getRetryCount(msg))
}

func TestGetRetryCount_ValidValue(t *testing.T) {
	msg := message.NewMessage("test-id", nil)
	msg.Metadata.Set(MetadataRetryCount, "2")
	assert.Equal(t, 2, getRetryCount(msg))
}

func TestGetRetryCount_InvalidValue(t *testing.T) {
	msg := message.NewMessage("test-id", nil)
	msg.Metadata.Set(MetadataRetryCount, "not-a-number")
	assert.Equal(t, 0, getRetryCount(msg))
}

func TestDLQConstants(t *testing.T) {
	t.Run("max retries is positive", func(t *testing.T) {
		assert.Greater(t, MaxMessageRetries, 0)
	})

	t.Run("DLQ prefix is non-empty", func(t *testing.T) {
		assert.NotEmpty(t, DLQSubjectPrefix)
	})

	t.Run("DLQ subject prefix ends with dot", func(t *testing.T) {
		assert.Equal(t, "dlq.", DLQSubjectPrefix)
	})
}

func TestExtractString(t *testing.T) {
	tests := []struct {
		name     string
		data     any
		key      string
		expected string
	}{
		{
			name:     "valid map with key",
			data:     map[string]any{"user_id": "u-1"},
			key:      "user_id",
			expected: "u-1",
		},
		{
			name:     "map without key",
			data:     map[string]any{"user_id": "u-1"},
			key:      "missing",
			expected: "",
		},
		{
			name:     "nil data",
			data:     nil,
			key:      "key",
			expected: "",
		},
		{
			name:     "non-map data",
			data:     "a string",
			key:      "key",
			expected: "",
		},
		{
			name:     "non-string value",
			data:     map[string]any{"count": 42},
			key:      "count",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractString(tt.data, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewBillingEventConsumer_WithPublisher(t *testing.T) {
	// NewBillingEventConsumer should not panic with nil dependencies.
	consumer := NewBillingEventConsumer(nil, nil, nil, nil, nil, nil)
	assert.NotNil(t, consumer)
}

func TestStreamDLQ_InStreamConfigs(t *testing.T) {
	configs := StreamConfigs()

	var found bool
	for _, cfg := range configs {
		if cfg.Name == StreamDLQ {
			found = true
			assert.Contains(t, cfg.Subjects, "dlq.>")
			assert.Greater(t, cfg.MaxAge.Hours(), float64(0))
			break
		}
	}
	assert.True(t, found, "DLQ stream must be present in StreamConfigs")
}
