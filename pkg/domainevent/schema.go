package domainevent

import (
	"slices"
	"strconv"
)

// Upcaster transforms an event's data payload from one schema version to the
// next. Each upcaster handles exactly one version transition (e.g., v1->v2).
type Upcaster interface {
	// FromVersion returns the version this upcaster reads.
	FromVersion() int
	// Upcast transforms the payload from FromVersion to FromVersion+1.
	Upcast(data map[string]any) map[string]any
}

// SchemaRegistry holds registered upcasters per event type and provides a
// single Upcast method that brings any event to the latest schema version.
type SchemaRegistry struct {
	upcasters map[EventType][]Upcaster // sorted by FromVersion
}

// NewSchemaRegistry creates an empty schema registry.
func NewSchemaRegistry() *SchemaRegistry {
	return &SchemaRegistry{
		upcasters: make(map[EventType][]Upcaster),
	}
}

// Register adds an upcaster for a specific event type. Upcasters must be
// registered in order (v1->v2 before v2->v3). Panics on duplicate version.
func (r *SchemaRegistry) Register(eventType EventType, u Upcaster) {
	chain := r.upcasters[eventType]
	for _, existing := range chain {
		if existing.FromVersion() == u.FromVersion() {
			panic("duplicate upcaster for " + string(eventType) + " version " +
				strconv.Itoa(u.FromVersion()))
		}
	}
	r.upcasters[eventType] = append(chain, u)
	// Sort by FromVersion to ensure correct application order.
	slices.SortFunc(r.upcasters[eventType], func(a, b Upcaster) int {
		return a.FromVersion() - b.FromVersion()
	})
}

// LatestVersion returns the highest known version for an event type.
// Returns DefaultEventVersion if no upcasters are registered (all events
// start at v1).
func (r *SchemaRegistry) LatestVersion(eventType EventType) int {
	chain := r.upcasters[eventType]
	if len(chain) == 0 {
		return DefaultEventVersion
	}
	return chain[len(chain)-1].FromVersion() + 1
}

// Upcast applies all necessary upcasters to bring the event's data to the
// latest schema version. If no upcasters are registered for the event type,
// or the event is already at the latest version, returns the event unchanged.
//
// The event's Data field must be map[string]any (the result of JSON
// unmarshalling). If it is not, the event is returned unchanged.
func (r *SchemaRegistry) Upcast(event Event) Event {
	chain := r.upcasters[event.Type]
	if len(chain) == 0 {
		return event
	}

	data, ok := event.Data.(map[string]any)
	if !ok {
		return event
	}

	for _, u := range chain {
		if event.Version > u.FromVersion() {
			continue // already past this version
		}
		if event.Version == u.FromVersion() {
			data = u.Upcast(data)
			event.Version = u.FromVersion() + 1
		}
	}

	event.Data = data
	return event
}
