package domainevent

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_WithMapData(t *testing.T) {
	e := New("user.registered", map[string]any{"user_id": "u-1"})

	assert.Equal(t, EventType("user.registered"), e.Type)
	assert.Equal(t, DefaultEventVersion, e.Version)
	assert.False(t, e.Timestamp.IsZero())

	m := e.DataAsMap()
	require.NotNil(t, m)
	assert.Equal(t, "u-1", m["user_id"])
}

type testPayload struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

func TestNew_WithTypedPayload(t *testing.T) {
	payload := testPayload{UserID: "u-1", Email: "test@example.com"}
	e := New("user.registered", payload)

	assert.Equal(t, EventType("user.registered"), e.Type)

	// Data should be the typed struct, not a map.
	typed, ok := e.Data.(testPayload)
	require.True(t, ok)
	assert.Equal(t, "u-1", typed.UserID)
	assert.Equal(t, "test@example.com", typed.Email)
}

func TestDataAsMap_WithTypedPayload_ReturnsNil(t *testing.T) {
	e := New("user.registered", testPayload{UserID: "u-1"})

	// DataAsMap returns nil for typed (non-map) payloads.
	assert.Nil(t, e.DataAsMap())
}

func TestDataAsMap_WithNilData_ReturnsNil(t *testing.T) {
	e := New("test.event", nil)
	assert.Nil(t, e.DataAsMap())
}

func TestNewAt_UsesExplicitTimestamp(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	e := NewAt("test.event", map[string]any{"key": "val"}, ts)

	assert.Equal(t, ts, e.Timestamp)
	assert.Equal(t, DefaultEventVersion, e.Version)
	assert.Empty(t, e.EntityID) // no entity for plain NewAt
}

func TestNewWithEntity_SetsEntityID(t *testing.T) {
	e := NewWithEntity("subscription.activated", map[string]any{"sub": "s-1"}, "sub-123")

	assert.Equal(t, EventType("subscription.activated"), e.Type)
	assert.Equal(t, "sub-123", e.EntityID)
	assert.False(t, e.Timestamp.IsZero())
}

func TestNewAtWithEntity_SetsTimestampAndEntityID(t *testing.T) {
	ts := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	e := NewAtWithEntity("invoice.paid", map[string]any{"inv": "i-1"}, ts, "inv-456")

	assert.Equal(t, EventType("invoice.paid"), e.Type)
	assert.Equal(t, ts, e.Timestamp)
	assert.Equal(t, "inv-456", e.EntityID)
}

func TestEvent_JSONRoundTrip_TypedPayload(t *testing.T) {
	original := New("user.registered", testPayload{
		UserID: "u-1",
		Email:  "test@example.com",
	})

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded Event
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Type, decoded.Type)

	// After JSON round-trip, typed payload becomes map[string]any.
	m := decoded.DataAsMap()
	require.NotNil(t, m)
	assert.Equal(t, "u-1", m["user_id"])
	assert.Equal(t, "test@example.com", m["email"])
}

func TestEvent_JSONRoundTrip_MapPayload(t *testing.T) {
	original := New("invoice.paid", map[string]any{
		"invoice_id": "inv-1",
		"amount":     float64(1000),
	})

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded Event
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	m := decoded.DataAsMap()
	require.NotNil(t, m)
	assert.Equal(t, "inv-1", m["invoice_id"])
	assert.Equal(t, float64(1000), m["amount"])
}

func TestEvent_JSONRoundTrip_WithEntityID(t *testing.T) {
	original := NewWithEntity("subscription.activated", testPayload{
		UserID: "u-1",
		Email:  "test@example.com",
	}, "sub-789")

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded Event
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Type, decoded.Type)
	assert.Equal(t, "sub-789", decoded.EntityID)
}

func TestEvent_JSONRoundTrip_EmptyEntityID_OmittedFromJSON(t *testing.T) {
	original := New("test.event", map[string]any{"key": "val"})

	data, err := json.Marshal(original)
	require.NoError(t, err)

	// entity_id should be omitted from JSON when empty.
	assert.NotContains(t, string(data), "entity_id")

	var decoded Event
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Empty(t, decoded.EntityID)
}

// typedTestPayload implements EventPayload for testing NewTyped.
type typedTestPayload struct {
	UserID string `json:"user_id"`
}

func (typedTestPayload) EventType() EventType { return "test.typed" }

func TestNewTyped_SetsTypeFromPayload(t *testing.T) {
	ts := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	payload := typedTestPayload{UserID: "u-1"}
	e := NewTyped(payload, ts, "entity-1")

	assert.Equal(t, EventType("test.typed"), e.Type)
	assert.Equal(t, DefaultEventVersion, e.Version)
	assert.Equal(t, ts, e.Timestamp)
	assert.Equal(t, "entity-1", e.EntityID)

	typed, ok := e.Data.(typedTestPayload)
	require.True(t, ok)
	assert.Equal(t, "u-1", typed.UserID)
}

func TestEvent_JSONRoundTrip_IncludesVersion(t *testing.T) {
	e := New("test.version", map[string]any{"key": "val"})

	data, err := json.Marshal(e)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"version":1`)

	var decoded Event
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, DefaultEventVersion, decoded.Version)
}
