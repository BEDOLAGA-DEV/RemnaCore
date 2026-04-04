package domainevent

// EventRecorder is embedded in aggregate roots to accumulate domain events
// during mutations. Services flush these events after persisting the aggregate,
// ensuring events are never silently lost.
type EventRecorder struct {
	events []Event
}

// RecordEvent adds an event to the aggregate's pending events list.
func (r *EventRecorder) RecordEvent(event Event) {
	r.events = append(r.events, event)
}

// DomainEvents returns all accumulated events and clears the internal list.
// The caller is responsible for publishing the returned events.
func (r *EventRecorder) DomainEvents() []Event {
	events := r.events
	r.events = nil
	return events
}

// HasEvents reports whether there are pending unpublished events.
func (r *EventRecorder) HasEvents() bool {
	return len(r.events) > 0
}
