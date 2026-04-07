package domainevent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePublisher records published events and optionally returns an error from
// PublishBatch when any event in the batch matches failOn.
type fakePublisher struct {
	published      []Event
	batchPublished []Event
	failOn         EventType // return error when this event type is in a batch
}

func (p *fakePublisher) Publish(_ context.Context, event Event) error {
	if p.failOn != "" && event.Type == p.failOn {
		return errors.New("publish failed")
	}
	p.published = append(p.published, event)
	return nil
}

func (p *fakePublisher) PublishBatch(_ context.Context, events []Event) error {
	for _, event := range events {
		if p.failOn != "" && event.Type == p.failOn {
			return errors.New("publish batch failed")
		}
	}
	p.batchPublished = append(p.batchPublished, events...)
	return nil
}

func TestPublishAll_FlushesAndPublishesBatch(t *testing.T) {
	var r EventRecorder
	r.RecordEvent(NewAt("event.one", nil, time.Now()))
	r.RecordEvent(NewAt("event.two", nil, time.Now()))

	pub := &fakePublisher{}
	err := PublishAll(context.Background(), pub, &r)

	require.NoError(t, err)
	require.Len(t, pub.batchPublished, 2)
	assert.Equal(t, EventType("event.one"), pub.batchPublished[0].Type)
	assert.Equal(t, EventType("event.two"), pub.batchPublished[1].Type)

	// Events should be cleared after flush.
	assert.False(t, r.HasEvents())
}

func TestPublishAll_NoEvents_NoOp(t *testing.T) {
	var r EventRecorder
	pub := &fakePublisher{}

	err := PublishAll(context.Background(), pub, &r)

	require.NoError(t, err)
	assert.Empty(t, pub.batchPublished)
}

func TestPublishAll_BatchError_NoPartialPublish(t *testing.T) {
	var r EventRecorder
	r.RecordEvent(NewAt("event.ok", nil, time.Now()))
	r.RecordEvent(NewAt("event.fail", nil, time.Now()))
	r.RecordEvent(NewAt("event.unreachable", nil, time.Now()))

	pub := &fakePublisher{failOn: "event.fail"}
	err := PublishAll(context.Background(), pub, &r)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "publish batch failed")
	// No events should be published — batch is all-or-nothing.
	assert.Empty(t, pub.batchPublished)
}

// Verify EventRecorder satisfies EventSource at compile time.
var _ EventSource = (*EventRecorder)(nil)

// Verify fakePublisher satisfies Publisher at compile time.
var _ Publisher = (*fakePublisher)(nil)
