package remnawave

import (
	"encoding/json"
	"time"
)

// RemnawaveSquad represents an internal or external squad in the Remnawave panel.
type RemnawaveSquad struct {
	UUID      string          `json:"uuid"`
	Name      string          `json:"name"`
	Info      json.RawMessage `json:"info,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

// internalSquadsResponse matches Remnawave's actual response format:
// {"response": {"total": N, "internalSquads": [...]}}
type internalSquadsResponse struct {
	Response struct {
		Total          int              `json:"total"`
		InternalSquads []RemnawaveSquad `json:"internalSquads"`
	} `json:"response"`
}

// externalSquadsResponse matches Remnawave's actual response format:
// {"response": {"total": N, "externalSquads": [...]}}
type externalSquadsResponse struct {
	Response struct {
		Total          int              `json:"total"`
		ExternalSquads []RemnawaveSquad `json:"externalSquads"`
	} `json:"response"`
}

// CreateSquadRequest is the payload for creating an internal or external squad.
// Inbounds is required for internal squads (array of inbound UUIDs).
type CreateSquadRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Inbounds    []string `json:"inbounds,omitempty"`
}

// UpdateSquadRequest is the payload for updating an internal or external squad.
type UpdateSquadRequest struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// BulkUsersRequest is the payload for adding or removing specific users from a
// squad. Remnawave 3 addresses users by numeric id, and the "many-users"
// endpoints are the ones that take a list — the plain add-users/remove-users
// pair operates on EVERY user and carries no body at all.
type BulkUsersRequest struct {
	UserIDs []int64 `json:"userIds"`
}

// setExternalSquadRequest is the minimal PATCH /api/users body used to move a
// user between external squads. A nil ExternalSquadUUID serialises to null,
// which clears the assignment. It is deliberately separate from
// UpdateUserRequest so a squad change never rewrites unrelated user fields.
type setExternalSquadRequest struct {
	ID                int64   `json:"id"`
	ExternalSquadUUID *string `json:"externalSquadUuid"`
}
