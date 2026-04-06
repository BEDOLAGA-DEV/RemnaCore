package domainevent

import (
	"context"
	"fmt"
)

// EventSource is implemented by aggregates that embed EventRecorder and
// accumulate domain events during mutations. Services call PublishAll after
// persisting the aggregate to flush and publish all pending events.
//
// EventRecorder satisfies this interface out of the box.
type EventSource interface {
	DomainEvents() []Event
}

// PublishAll flushes all recorded events from the source and publishes them
// through the given publisher. Returns on the first publish error. This
// function should be called inside a transaction to ensure atomicity with
// the aggregate persistence (outbox pattern).
//
// It replaces the per-service publishAggregateEvents method that was
// duplicated across billing, identity, payment, and reseller services.
func PublishAll(ctx context.Context, pub Publisher, src EventSource) error {
	for _, event := range src.DomainEvents() {
		if err := pub.Publish(ctx, event); err != nil {
			return fmt.Errorf("publish %s: %w", event.Type, err)
		}
	}
	return nil
}
