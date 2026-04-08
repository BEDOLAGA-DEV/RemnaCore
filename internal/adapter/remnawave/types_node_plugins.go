package remnawave

import "encoding/json"

// NodePlugin represents a plugin attached to a node in Remnawave.
type NodePlugin struct {
	UUID      string          `json:"uuid"`
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Config    json.RawMessage `json:"config,omitempty"`
	IsEnabled bool            `json:"isEnabled"`
	Order     int             `json:"order"`
}

// CreateNodePluginRequest is the payload to create a new node plugin.
type CreateNodePluginRequest struct {
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Config    json.RawMessage `json:"config,omitempty"`
	IsEnabled bool            `json:"isEnabled"`
}

// UpdateNodePluginRequest is the payload to update an existing node plugin.
type UpdateNodePluginRequest struct {
	UUID      string          `json:"uuid"`
	Name      string          `json:"name,omitempty"`
	Type      string          `json:"type,omitempty"`
	Config    json.RawMessage `json:"config,omitempty"`
	IsEnabled *bool           `json:"isEnabled,omitempty"`
}

// ReorderNodePluginsRequest is the payload to reorder node plugins.
type ReorderNodePluginsRequest struct {
	UUIDs []string `json:"uuids"`
}
