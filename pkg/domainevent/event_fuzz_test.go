package domainevent

import (
	"encoding/json"
	"testing"
)

func FuzzEventJSONRoundTrip(f *testing.F) {
	f.Add([]byte(`{"type":"test","timestamp":"2026-01-01T00:00:00Z","data":{"key":"value"}}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`{"type":"","data":null}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var event Event
		if err := json.Unmarshal(data, &event); err != nil {
			return
		}
		// Re-marshal must never panic.
		_, _ = json.Marshal(event)
	})
}
