package domainevent

import (
	"encoding/json"
	"fmt"
)

// UnmarshalPayload extracts a typed payload from an event's Data field.
// Data may arrive as:
//   - T (already the target type — e.g., when the event was constructed in-process)
//   - map[string]any (from JSON unmarshal of NATS messages)
//   - any other JSON-serialisable type (fallback re-marshal path)
//
// This generic helper eliminates the need for per-handler extractString/extractInt
// calls and gives compile-time safety on the resulting payload fields.
func UnmarshalPayload[T any](event Event) (T, error) {
	var zero T

	// Fast path: Data is already the target type (in-process events).
	if typed, ok := event.Data.(T); ok {
		return typed, nil
	}

	// Slow path: re-marshal through JSON. This covers map[string]any (from
	// NATS JSON unmarshal) and any other serialisable representation.
	raw, err := json.Marshal(event.Data)
	if err != nil {
		return zero, fmt.Errorf("marshal event data for %s: %w", event.Type, err)
	}

	var payload T
	if err := json.Unmarshal(raw, &payload); err != nil {
		return zero, fmt.Errorf("unmarshal %s payload: %w", event.Type, err)
	}

	return payload, nil
}
