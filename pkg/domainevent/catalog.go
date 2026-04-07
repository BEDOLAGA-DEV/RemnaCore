package domainevent

import "reflect"

// CatalogEntry describes a domain event type for documentation and validation.
// Each entry records the producing bounded context, known consumers, the
// payload struct type, and the current schema version for that event.
type CatalogEntry struct {
	Type        EventType
	Producer    string       // bounded context: "billing", "multisub", "payment", "identity", "reseller", "plugin", "infra"
	Consumers   []string     // contexts that consume this event: e.g. ["multisub", "billing"]
	PayloadType reflect.Type // reflect.TypeOf(SomePayload{}); nil for untyped (map[string]any) payloads
	Version     int          // current schema version
}

// EventCatalog is a registry of all domain event types.
// Built at application startup and used by architecture tests and runtime
// validation to ensure every event type is accounted for.
//
// Thread safety: Register must only be called during initialisation (e.g.,
// inside BuildEventCatalog). After construction the catalog is read-only.
type EventCatalog struct {
	entries map[EventType]CatalogEntry
}

// NewEventCatalog creates an empty catalog.
func NewEventCatalog() *EventCatalog {
	return &EventCatalog{entries: make(map[EventType]CatalogEntry)}
}

// Register adds an event type to the catalog. Panics on duplicate registration
// to catch copy-paste errors at startup rather than at runtime.
func (c *EventCatalog) Register(entry CatalogEntry) {
	if _, exists := c.entries[entry.Type]; exists {
		panic("duplicate event catalog entry for " + string(entry.Type))
	}
	c.entries[entry.Type] = entry
}

// Get returns the catalog entry for an event type, or false if not registered.
func (c *EventCatalog) Get(eventType EventType) (CatalogEntry, bool) {
	e, ok := c.entries[eventType]
	return e, ok
}

// All returns all registered entries keyed by event type.
func (c *EventCatalog) All() map[EventType]CatalogEntry {
	return c.entries
}

// Types returns all registered event type strings.
func (c *EventCatalog) Types() []EventType {
	types := make([]EventType, 0, len(c.entries))
	for t := range c.entries {
		types = append(types, t)
	}
	return types
}

// Len returns the number of registered event types.
func (c *EventCatalog) Len() int {
	return len(c.entries)
}
