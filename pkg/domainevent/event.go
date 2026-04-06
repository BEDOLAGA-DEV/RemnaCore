// Package domainevent is a shared kernel package providing the canonical Event
// type, Publisher interface, and EventType constants used across all bounded
// contexts (identity, billing, multisub, payment, reseller). Every context
// produces and/or consumes domain events through these types, making this
// package the single most widely imported piece of the shared kernel.
package domainevent

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// EventType identifies the kind of domain event.
type EventType string

// DefaultEventVersion is the schema version assigned to all events unless
// explicitly overridden. Increment when a payload schema changes in a
// backward-incompatible way.
const DefaultEventVersion = 1

// Event represents a domain event emitted by any bounded context.
// Data accepts any JSON-serialisable value: typed payload structs are preferred
// for compile-time safety, but map[string]any is still accepted for backward
// compatibility and dynamic event sources (webhooks, plugins, infra).
//
// ID is a UUIDv7 that uniquely identifies each event instance. It is generated
// at construction time and used as the outbox row ID, the NATS dedup key, and
// the consumer-side idempotency key. Two identical business operations produce
// two events with different IDs, enabling correct deduplication without
// blocking legitimate repeated operations on the same aggregate.
//
// Version tracks the schema version of the payload, enabling backward-compatible
// evolution of the 45+ event types. Consumers can branch on Version to handle
// old and new payload shapes gracefully.
//
// EntityID identifies the aggregate instance that produced the event. Consumers
// use it for per-entity serial processing to guarantee ordering.
//
// TraceParent carries the W3C traceparent header value from the originating span.
// It enables distributed tracing correlation across the outbox → NATS → consumer
// pipeline. The field is set by the OutboxPublisher when an active trace span
// exists in the publish context, and propagated by the OutboxRelay as NATS
// message metadata so consumers can link their processing spans to the original
// business operation that produced the event.
type Event struct {
	ID          string    `json:"id"`
	Type        EventType `json:"type"`
	Version     int       `json:"version"`
	Timestamp   time.Time `json:"timestamp"`
	Data        any       `json:"data"`
	EntityID    string    `json:"entity_id,omitempty"`
	TraceParent string    `json:"trace_parent,omitempty"`
}

// newEventID generates a UUIDv7 string for use as an event instance identifier.
// UUIDv7 is time-ordered, making outbox row inserts append-only and enabling
// efficient B-tree index usage on the partitioned outbox table.
func newEventID() string {
	return uuid.Must(uuid.NewV7()).String()
}

// New creates an Event with the given type, data, and the current timestamp.
func New(eventType EventType, data any) Event {
	return NewAt(eventType, data, time.Now())
}

// NewAt creates an Event with an explicit timestamp. Use this in aggregate
// constructors and service methods that receive a deterministic time.Time.
func NewAt(eventType EventType, data any, ts time.Time) Event {
	return Event{
		ID:        newEventID(),
		Type:      eventType,
		Version:   DefaultEventVersion,
		Timestamp: ts,
		Data:      data,
	}
}

// NewWithEntity creates an Event tagged with the source aggregate's entity ID.
// Consumers use EntityID for per-entity serial processing to guarantee ordering
// within a single aggregate.
func NewWithEntity(eventType EventType, data any, entityID string) Event {
	return Event{
		ID:        newEventID(),
		Type:      eventType,
		Version:   DefaultEventVersion,
		Timestamp: time.Now(),
		Data:      data,
		EntityID:  entityID,
	}
}

// NewAtWithEntity creates an Event with an explicit timestamp and entity ID.
func NewAtWithEntity(eventType EventType, data any, ts time.Time, entityID string) Event {
	return Event{
		ID:        newEventID(),
		Type:      eventType,
		Version:   DefaultEventVersion,
		Timestamp: ts,
		Data:      data,
		EntityID:  entityID,
	}
}

// NewAtVersioned creates an Event with explicit version, timestamp, and entity ID.
// Use this when publishing events with a schema version different from DefaultEventVersion.
func NewAtVersioned(eventType EventType, version int, data any, ts time.Time, entityID string) Event {
	return Event{
		ID:        newEventID(),
		Type:      eventType,
		Version:   version,
		Timestamp: ts,
		Data:      data,
		EntityID:  entityID,
	}
}

// DataAsMap is a backward-compatible helper that returns the Data field as
// map[string]any. It returns nil if Data is not a map. Consumers that
// already use typed payloads should type-assert directly instead.
func (e Event) DataAsMap() map[string]any {
	if m, ok := e.Data.(map[string]any); ok {
		return m
	}
	return nil
}

// EventPayload is an optional interface that typed payload structs can implement
// for compile-time safety. Payloads that implement this interface can be used
// with NewTyped/NewTypedAt constructors, which set the event type automatically.
type EventPayload interface {
	EventType() EventType
}

// NewTyped creates an Event from a typed payload that knows its own event type.
// This is the preferred constructor for aggregate-level event recording.
func NewTyped(payload EventPayload, ts time.Time, entityID string) Event {
	return Event{
		ID:        newEventID(),
		Type:      payload.EventType(),
		Version:   DefaultEventVersion,
		Timestamp: ts,
		Data:      payload,
		EntityID:  entityID,
	}
}

// Publisher abstracts event dispatching so domain services are not coupled to
// any particular messaging infrastructure.
type Publisher interface {
	Publish(ctx context.Context, event Event) error
}
