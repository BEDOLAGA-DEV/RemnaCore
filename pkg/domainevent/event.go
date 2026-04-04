// Package domainevent provides shared domain event types used across all
// bounded contexts (identity, billing, multisub, etc.).
package domainevent

import (
	"context"
	"time"
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
// Version tracks the schema version of the payload, enabling backward-compatible
// evolution of the 45+ event types. Consumers can branch on Version to handle
// old and new payload shapes gracefully.
//
// EntityID identifies the aggregate instance that produced the event. Consumers
// use it for business-level idempotency ({event_type}:{entity_id}) and
// per-entity serial processing to guarantee ordering.
type Event struct {
	Type      EventType `json:"type"`
	Version   int       `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
	EntityID  string    `json:"entity_id,omitempty"`
}

// New creates an Event with the given type, data, and the current timestamp.
func New(eventType EventType, data any) Event {
	return NewAt(eventType, data, time.Now())
}

// NewAt creates an Event with an explicit timestamp. Use this in aggregate
// constructors and service methods that receive a deterministic time.Time.
func NewAt(eventType EventType, data any, ts time.Time) Event {
	return Event{
		Type:      eventType,
		Version:   DefaultEventVersion,
		Timestamp: ts,
		Data:      data,
	}
}

// NewWithEntity creates an Event tagged with the source aggregate's entity ID.
// Consumers use EntityID for business-level idempotency and per-entity serial
// processing to guarantee ordering within a single aggregate.
func NewWithEntity(eventType EventType, data any, entityID string) Event {
	return Event{
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
		Type:      eventType,
		Version:   DefaultEventVersion,
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
