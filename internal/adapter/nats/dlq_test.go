package nats

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	t.Run("retry key prefix is non-empty", func(t *testing.T) {
		assert.NotEmpty(t, retryKeyPrefix)
	})
}

func TestExtractStringFromData(t *testing.T) {
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
			result := extractStringFromData(tt.data, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewBillingEventConsumer_WithPublisher(t *testing.T) {
	// NewBillingEventConsumer should not panic with nil dependencies.
	consumer := NewBillingEventConsumer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	assert.NotNil(t, consumer)
}

func TestDLQPayload_EntityIDAndEventTypeSerialization(t *testing.T) {
	tests := []struct {
		name     string
		payload  DLQPayload
		wantID   string
		wantType string
		wantOmit bool // whether entity_id/event_type should be omitted
	}{
		{
			name: "enriched payload includes entity_id and event_type",
			payload: DLQPayload{
				OriginalSubject: "subscription.cancelled",
				OriginalPayload: `{"data":{}}`,
				Error:           "test error",
				MsgID:           "msg-1",
				FailedAt:        "2026-04-06T10:00:00Z",
				RetryCount:      3,
				EntityID:        "sub-123",
				EventType:       "subscription.cancelled",
			},
			wantID:   "sub-123",
			wantType: "subscription.cancelled",
		},
		{
			name: "empty entity_id and event_type are omitted",
			payload: DLQPayload{
				OriginalSubject: "subscription.cancelled",
				OriginalPayload: `{"data":{}}`,
				Error:           "unmarshal error",
				MsgID:           "msg-2",
				FailedAt:        "2026-04-06T10:00:00Z",
				RetryCount:      0,
			},
			wantOmit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			var decoded map[string]any
			require.NoError(t, json.Unmarshal(data, &decoded))

			if tt.wantOmit {
				_, hasEntityID := decoded["entity_id"]
				_, hasEventType := decoded["event_type"]
				assert.False(t, hasEntityID, "entity_id should be omitted when empty")
				assert.False(t, hasEventType, "event_type should be omitted when empty")
			} else {
				assert.Equal(t, tt.wantID, decoded["entity_id"])
				assert.Equal(t, tt.wantType, decoded["event_type"])
			}
		})
	}
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
