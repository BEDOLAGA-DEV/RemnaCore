package domainevent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePublisher records published events and optionally returns an error.
type fakePublisher struct {
	published []Event
	failOn    EventType // return error when this event type is encountered
}

func (p *fakePublisher) Publish(_ context.Context, event Event) error {
	if p.failOn != "" && event.Type == p.failOn {
		return errors.New("publish failed")
	}
	p.published = append(p.published, event)
	return nil
}

func TestPublishAll_FlushesAndPublishes(t *testing.T) {
	var r EventRecorder
	r.RecordEvent(NewAt("event.one", nil, time.Now()))
	r.RecordEvent(NewAt("event.two", nil, time.Now()))

	pub := &fakePublisher{}
	err := PublishAll(context.Background(), pub, &r)

	require.NoError(t, err)
	require.Len(t, pub.published, 2)
	assert.Equal(t, EventType("event.one"), pub.published[0].Type)
	assert.Equal(t, EventType("event.two"), pub.published[1].Type)

	// Events should be cleared after flush.
	assert.False(t, r.HasEvents())
}

func TestPublishAll_NoEvents_NoOp(t *testing.T) {
	var r EventRecorder
	pub := &fakePublisher{}

	err := PublishAll(context.Background(), pub, &r)

	require.NoError(t, err)
	assert.Empty(t, pub.published)
}

func TestPublishAll_StopsOnFirstError(t *testing.T) {
	var r EventRecorder
	r.RecordEvent(NewAt("event.ok", nil, time.Now()))
	r.RecordEvent(NewAt("event.fail", nil, time.Now()))
	r.RecordEvent(NewAt("event.unreachable", nil, time.Now()))

	pub := &fakePublisher{failOn: "event.fail"}
	err := PublishAll(context.Background(), pub, &r)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "publish event.fail")
	// Only the first event was successfully published.
	assert.Len(t, pub.published, 1)
}

// Verify EventRecorder satisfies EventSource at compile time.
var _ EventSource = (*EventRecorder)(nil)
