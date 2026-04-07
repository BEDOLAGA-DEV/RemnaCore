package domainevent

// EventRecorder is embedded in aggregate roots to accumulate domain events
// during mutations. Services flush these events after persisting the aggregate,
// ensuring events are never silently lost.
//
// The flushed flag provides double-flush protection: calling DomainEvents twice
// without recording new events in between returns nil on the second call.
// This makes accidental double-flush a safe no-op rather than a silent data loss.
type EventRecorder struct {
	events  []Event
	flushed bool
}

// RecordEvent adds an event to the aggregate's pending events list and resets
// the flushed flag so that a subsequent DomainEvents call returns the new events.
func (r *EventRecorder) RecordEvent(event Event) {
	r.events = append(r.events, event)
	r.flushed = false
}

// DomainEvents returns all accumulated events and clears the internal list.
// The caller is responsible for publishing the returned events.
//
// Calling DomainEvents twice without recording new events in between returns
// nil on the second call. This double-flush protection prevents silent event
// loss when a service layer accidentally flushes the same aggregate twice.
func (r *EventRecorder) DomainEvents() []Event {
	if r.flushed && len(r.events) == 0 {
		return nil
	}
	events := r.events
	r.events = nil
	r.flushed = true
	return events
}

// HasEvents reports whether there are pending unpublished events.
func (r *EventRecorder) HasEvents() bool {
	return len(r.events) > 0
}
