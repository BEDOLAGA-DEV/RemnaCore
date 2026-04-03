// Package domainevent provides shared domain event types used across all
// bounded contexts (identity, billing, multisub, etc.).
package domainevent

import (
	"context"
	"time"
)

// EventType identifies the kind of domain event.
type EventType string

// Event represents a domain event emitted by any bounded context.
// Data accepts any JSON-serialisable value: typed payload structs are preferred
// for compile-time safety, but map[string]any is still accepted for backward
// compatibility and dynamic event sources (webhooks, plugins, infra).
type Event struct {
	Type      EventType `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
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
		Timestamp: ts,
		Data:      data,
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

// Publisher abstracts event dispatching so domain services are not coupled to
// any particular messaging infrastructure.
type Publisher interface {
	Publish(ctx context.Context, event Event) error
}
