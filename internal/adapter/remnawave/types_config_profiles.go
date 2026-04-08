package remnawave

import (
	"encoding/json"
	"time"
)

// RemnawaveConfigProfile represents a configuration profile in the Remnawave panel.
type RemnawaveConfigProfile struct {
	UUID      string          `json:"uuid"`
	Name      string          `json:"name"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	Extra     json.RawMessage `json:"extra,omitempty"`
}

// RemnawaveInbound represents an inbound proxy entry in the Remnawave panel.
type RemnawaveInbound struct {
	UUID string `json:"uuid"`
	Tag  string `json:"tag"`
	Type string `json:"type"`
}

// CreateConfigProfileRequest is the payload for creating a config profile.
type CreateConfigProfileRequest struct {
	Name string `json:"name"`
}

// UpdateConfigProfileRequest is the payload for updating a config profile.
type UpdateConfigProfileRequest struct {
	UUID string `json:"uuid"`
	Name string `json:"name,omitempty"`
}
