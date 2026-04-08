package remnawave

import (
	"encoding/json"
	"time"
)

// RemnawaveSquad represents an internal or external squad in the Remnawave panel.
type RemnawaveSquad struct {
	UUID        string          `json:"uuid"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	Extra       json.RawMessage `json:"extra,omitempty"`
}

// CreateSquadRequest is the payload for creating an internal or external squad.
type CreateSquadRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// UpdateSquadRequest is the payload for updating an internal or external squad.
type UpdateSquadRequest struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// BulkUsersRequest is the payload for adding or removing users from a squad.
type BulkUsersRequest struct {
	UserUUIDs []string `json:"userUuids"`
}

