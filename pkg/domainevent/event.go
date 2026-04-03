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
type Event struct {
	Type      EventType      `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data"`
}

// New creates an Event with the given type, data, and the current timestamp.
func New(eventType EventType, data map[string]any) Event {
	return NewAt(eventType, data, time.Now())
}

// NewAt creates an Event with an explicit timestamp. Use this in aggregate
// constructors and service methods that receive a deterministic time.Time.
func NewAt(eventType EventType, data map[string]any, ts time.Time) Event {
	return Event{
		Type:      eventType,
		Timestamp: ts,
		Data:      data,
	}
}

// Publisher abstracts event dispatching so domain services are not coupled to
// any particular messaging infrastructure.
type Publisher interface {
	Publish(ctx context.Context, event Event) error
}
