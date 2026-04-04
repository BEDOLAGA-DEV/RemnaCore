package domainevent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventRecorder_RecordEvent(t *testing.T) {
	var r EventRecorder

	e := NewAt("test.event", map[string]any{"key": "val"}, time.Now())
	r.RecordEvent(e)

	assert.True(t, r.HasEvents())
}

func TestEventRecorder_DomainEvents_ReturnsAndClears(t *testing.T) {
	var r EventRecorder

	e1 := NewAt("event.one", nil, time.Now())
	e2 := NewAt("event.two", nil, time.Now())
	r.RecordEvent(e1)
	r.RecordEvent(e2)

	events := r.DomainEvents()

	require.Len(t, events, 2)
	assert.Equal(t, EventType("event.one"), events[0].Type)
	assert.Equal(t, EventType("event.two"), events[1].Type)

	// List is cleared after retrieval.
	assert.False(t, r.HasEvents())
	assert.Empty(t, r.DomainEvents())
}

func TestEventRecorder_HasEvents_EmptyByDefault(t *testing.T) {
	var r EventRecorder

	assert.False(t, r.HasEvents())
}

func TestEventRecorder_DomainEvents_EmptyReturnsNil(t *testing.T) {
	var r EventRecorder

	events := r.DomainEvents()

	assert.Nil(t, events)
}

func TestEventRecorder_MultipleFlushes(t *testing.T) {
	var r EventRecorder

	// First batch.
	r.RecordEvent(NewAt("batch.one", nil, time.Now()))
	first := r.DomainEvents()
	require.Len(t, first, 1)

	// Second batch after flush.
	r.RecordEvent(NewAt("batch.two", nil, time.Now()))
	r.RecordEvent(NewAt("batch.three", nil, time.Now()))
	second := r.DomainEvents()
	require.Len(t, second, 2)

	// Empty after second flush.
	assert.False(t, r.HasEvents())
}
