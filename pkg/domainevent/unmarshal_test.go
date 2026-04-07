package domainevent

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unmarshalTestPayload is a typed payload used in UnmarshalPayload tests.
type unmarshalTestPayload struct {
	ID     string `json:"id"`
	Amount int64  `json:"amount"`
	Active bool   `json:"active"`
}

func TestUnmarshalPayload(t *testing.T) {
	now := time.Now()

	t.Run("already typed", func(t *testing.T) {
		want := unmarshalTestPayload{ID: "p-1", Amount: 42, Active: true}
		event := Event{
			Type:      "test.event",
			Timestamp: now,
			Data:      want,
		}

		got, err := UnmarshalPayload[unmarshalTestPayload](event)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("from map[string]any", func(t *testing.T) {
		event := Event{
			Type:      "test.event",
			Timestamp: now,
			Data: map[string]any{
				"id":     "p-2",
				"amount": float64(100), // JSON numbers are float64 in map[string]any
				"active": false,
			},
		}

		got, err := UnmarshalPayload[unmarshalTestPayload](event)
		require.NoError(t, err)
		assert.Equal(t, unmarshalTestPayload{ID: "p-2", Amount: 100, Active: false}, got)
	})

	t.Run("from json.RawMessage", func(t *testing.T) {
		raw := json.RawMessage(`{"id":"p-3","amount":55,"active":true}`)
		event := Event{
			Type:      "test.event",
			Timestamp: now,
			Data:      raw,
		}

		got, err := UnmarshalPayload[unmarshalTestPayload](event)
		require.NoError(t, err)
		assert.Equal(t, unmarshalTestPayload{ID: "p-3", Amount: 55, Active: true}, got)
	})

	t.Run("nil data returns zero value", func(t *testing.T) {
		event := Event{
			Type:      "test.event",
			Timestamp: now,
			Data:      nil,
		}

		got, err := UnmarshalPayload[unmarshalTestPayload](event)
		require.NoError(t, err)
		assert.Equal(t, unmarshalTestPayload{}, got)
	})

	t.Run("incompatible data returns error", func(t *testing.T) {
		event := Event{
			Type:      "test.event",
			Timestamp: now,
			Data:      "not a payload",
		}

		_, err := UnmarshalPayload[unmarshalTestPayload](event)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal test.event payload")
	})

	t.Run("round-trip through JSON matches original", func(t *testing.T) {
		original := unmarshalTestPayload{ID: "p-rt", Amount: 999, Active: true}
		event := NewAt("test.event", original, now)

		// Simulate NATS: marshal event to JSON, then unmarshal back.
		data, err := json.Marshal(event)
		require.NoError(t, err)

		var deserialized Event
		require.NoError(t, json.Unmarshal(data, &deserialized))

		// After JSON round-trip, Data is map[string]any.
		got, err := UnmarshalPayload[unmarshalTestPayload](deserialized)
		require.NoError(t, err)
		assert.Equal(t, original, got)
	})
}
