package domainevent

import "context"

// EventSource is implemented by aggregates that embed EventRecorder and
// accumulate domain events during mutations. Services call PublishAll after
// persisting the aggregate to flush and publish all pending events.
//
// EventRecorder satisfies this interface out of the box.
type EventSource interface {
	DomainEvents() []Event
}

// PublishAll flushes all recorded events from the source and publishes them
// through the given publisher using a single PublishBatch call. This avoids
// partial-publish failures where the Nth individual Publish fails after N-1
// rows are already committed to the outbox. The function should be called
// inside a transaction to ensure atomicity with aggregate persistence
// (outbox pattern).
//
// It replaces the per-service publishAggregateEvents method that was
// duplicated across billing, identity, payment, and reseller services.
func PublishAll(ctx context.Context, pub Publisher, src EventSource) error {
	events := src.DomainEvents()
	if len(events) == 0 {
		return nil
	}
	return pub.PublishBatch(ctx, events)
}
