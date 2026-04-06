package domainevent

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requireValidUUIDv7 asserts that s is a valid UUID and its version is 7.
func requireValidUUIDv7(t *testing.T, s string) {
	t.Helper()
	require.NotEmpty(t, s, "event ID must not be empty")
	parsed, err := uuid.Parse(s)
	require.NoError(t, err, "event ID must be a valid UUID")
	assert.Equal(t, uuid.Version(7), parsed.Version(), "event ID must be UUIDv7")
}

func TestNew_WithMapData(t *testing.T) {
	e := New("user.registered", map[string]any{"user_id": "u-1"})

	requireValidUUIDv7(t, e.ID)
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

	requireValidUUIDv7(t, e.ID)
	assert.Equal(t, ts, e.Timestamp)
	assert.Equal(t, DefaultEventVersion, e.Version)
	assert.Empty(t, e.EntityID) // no entity for plain NewAt
}

func TestNewWithEntity_SetsEntityID(t *testing.T) {
	e := NewWithEntity("subscription.activated", map[string]any{"sub": "s-1"}, "sub-123")

	requireValidUUIDv7(t, e.ID)
	assert.Equal(t, EventType("subscription.activated"), e.Type)
	assert.Equal(t, "sub-123", e.EntityID)
	assert.False(t, e.Timestamp.IsZero())
}

func TestNewAtWithEntity_SetsTimestampAndEntityID(t *testing.T) {
	ts := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	e := NewAtWithEntity("invoice.paid", map[string]any{"inv": "i-1"}, ts, "inv-456")

	requireValidUUIDv7(t, e.ID)
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

	requireValidUUIDv7(t, e.ID)
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

func TestEvent_ConstructorsGenerateUniqueIDs(t *testing.T) {
	// Two events from the same constructor with identical parameters must get
	// different IDs. This is the core property that fixes the false-duplicate
	// idempotency bug.
	e1 := NewWithEntity("subscription.renewed", map[string]any{"subscription_id": "sub-1"}, "sub-1")
	e2 := NewWithEntity("subscription.renewed", map[string]any{"subscription_id": "sub-1"}, "sub-1")

	requireValidUUIDv7(t, e1.ID)
	requireValidUUIDv7(t, e2.ID)
	assert.NotEqual(t, e1.ID, e2.ID,
		"two events with identical parameters must have different IDs")
}

func TestNewAtVersioned_SetsIDAndVersion(t *testing.T) {
	ts := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	customVersion := 3
	e := NewAtVersioned("user.upgraded", customVersion, map[string]any{"tier": "pro"}, ts, "user-1")

	requireValidUUIDv7(t, e.ID)
	assert.Equal(t, EventType("user.upgraded"), e.Type)
	assert.Equal(t, customVersion, e.Version)
	assert.Equal(t, ts, e.Timestamp)
	assert.Equal(t, "user-1", e.EntityID)
}

func TestEvent_JSONRoundTrip_PreservesID(t *testing.T) {
	original := NewWithEntity("subscription.activated", testPayload{
		UserID: "u-1",
		Email:  "test@example.com",
	}, "sub-789")

	data, err := json.Marshal(original)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id":"`)

	var decoded Event
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.ID, decoded.ID, "ID must survive JSON round-trip")
}

func TestEvent_JSONRoundTrip_BackwardCompat_EmptyID(t *testing.T) {
	// Simulate an old event without an ID field (pre-migration).
	oldJSON := `{"type":"subscription.cancelled","version":1,"timestamp":"2026-01-01T00:00:00Z","data":{"subscription_id":"sub-1"},"entity_id":"sub-1"}`

	var decoded Event
	err := json.Unmarshal([]byte(oldJSON), &decoded)
	require.NoError(t, err)

	assert.Empty(t, decoded.ID, "old events without ID must decode with empty ID")
	assert.Equal(t, EventType("subscription.cancelled"), decoded.Type)
	assert.Equal(t, "sub-1", decoded.EntityID)
}

func TestEvent_TraceParent_OmittedWhenEmpty(t *testing.T) {
	e := New("test.event", map[string]any{"key": "val"})

	data, err := json.Marshal(e)
	require.NoError(t, err)

	assert.NotContains(t, string(data), "trace_parent",
		"trace_parent must be omitted from JSON when empty")
}

func TestEvent_TraceParent_SurvivesJSONRoundTrip(t *testing.T) {
	tp := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	e := NewWithEntity("subscription.activated", map[string]any{"sub": "s-1"}, "sub-123")
	e.TraceParent = tp

	data, err := json.Marshal(e)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"trace_parent":"00-`)

	var decoded Event
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, tp, decoded.TraceParent,
		"TraceParent must survive JSON round-trip")
}

func TestEvent_TraceParent_BackwardCompat_MissingField(t *testing.T) {
	// Old events without trace_parent field must decode without error.
	oldJSON := `{"id":"abc","type":"test.event","version":1,"timestamp":"2026-01-01T00:00:00Z","data":null}`

	var decoded Event
	err := json.Unmarshal([]byte(oldJSON), &decoded)
	require.NoError(t, err)

	assert.Empty(t, decoded.TraceParent,
		"events without trace_parent must decode with empty TraceParent")
}
