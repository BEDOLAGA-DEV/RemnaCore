package remnawave

import (
	"context"
	"fmt"
	"net/http"
)

// Remnawave API path constants for squad endpoints.
const (
	APIPathInternalSquads = "/api/internal-squads/"
	APIPathExternalSquads = "/api/external-squads/"
)

// Squad sub-path constants.
const (
	subPathAccessibleNodes = "/accessible-nodes"
	// "many-users" variants target the listed users. The bare add-users /
	// remove-users endpoints apply to ALL users and take no body.
	subPathBulkAddUsers    = "/bulk-actions/add-many-users"
	subPathBulkRemoveUsers = "/bulk-actions/remove-many-users"
)

// ---------------------------------------------------------------------------
// Internal Squads
// ---------------------------------------------------------------------------

// GetInternalSquads returns all internal squads from Remnawave.
func (c *Client) GetInternalSquads(ctx context.Context) ([]RemnawaveSquad, error) {
	var resp internalSquadsResponse
	if err := c.do(ctx, http.MethodGet, APIPathInternalSquads, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Response.InternalSquads, nil
}

// CreateInternalSquad provisions a new internal squad in Remnawave.
func (c *Client) CreateInternalSquad(ctx context.Context, req CreateSquadRequest) (*RemnawaveSquad, error) {
	var resp APIResponse[RemnawaveSquad]
	if err := c.do(ctx, http.MethodPost, APIPathInternalSquads, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// UpdateInternalSquad modifies an existing internal squad in Remnawave.
func (c *Client) UpdateInternalSquad(ctx context.Context, req UpdateSquadRequest) (*RemnawaveSquad, error) {
	var resp APIResponse[RemnawaveSquad]
	if err := c.do(ctx, http.MethodPatch, APIPathInternalSquads, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// GetInternalSquadByUUID retrieves a single internal squad by UUID.
func (c *Client) GetInternalSquadByUUID(ctx context.Context, uuid string) (*RemnawaveSquad, error) {
	var resp APIResponse[RemnawaveSquad]
	if err := c.do(ctx, http.MethodGet, APIPathInternalSquads+uuid, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// DeleteInternalSquad removes an internal squad from Remnawave.
func (c *Client) DeleteInternalSquad(ctx context.Context, uuid string) error {
	return c.do(ctx, http.MethodDelete, APIPathInternalSquads+uuid, nil, nil)
}

// GetInternalSquadNodes returns the accessible nodes for an internal squad.
func (c *Client) GetInternalSquadNodes(ctx context.Context, uuid string) ([]RemnawaveNode, error) {
	var resp APIResponse[[]RemnawaveNode]
	if err := c.do(ctx, http.MethodGet, APIPathInternalSquads+uuid+subPathAccessibleNodes, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Response, nil
}

// AddUsersToInternalSquad adds the listed users to an internal squad.
func (c *Client) AddUsersToInternalSquad(ctx context.Context, uuid string, userIDs []int64) error {
	body := BulkUsersRequest{UserIDs: userIDs}
	return c.do(ctx, http.MethodPost, APIPathInternalSquads+uuid+subPathBulkAddUsers, body, nil)
}

// RemoveUsersFromInternalSquad removes the listed users from an internal squad.
func (c *Client) RemoveUsersFromInternalSquad(ctx context.Context, uuid string, userIDs []int64) error {
	body := BulkUsersRequest{UserIDs: userIDs}
	return c.do(ctx, http.MethodDelete, APIPathInternalSquads+uuid+subPathBulkRemoveUsers, body, nil)
}

// ReorderInternalSquads sets the display order for internal squads.
func (c *Client) ReorderInternalSquads(ctx context.Context, uuids []string) error {
	body := ReorderRequest{UUIDs: uuids}
	return c.do(ctx, http.MethodPost, APIPathInternalSquads+subPathActionsReorder, body, nil)
}

// ---------------------------------------------------------------------------
// External Squads
// ---------------------------------------------------------------------------

// GetExternalSquads returns all external squads from Remnawave.
func (c *Client) GetExternalSquads(ctx context.Context) ([]RemnawaveSquad, error) {
	var resp externalSquadsResponse
	if err := c.do(ctx, http.MethodGet, APIPathExternalSquads, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Response.ExternalSquads, nil
}

// CreateExternalSquad provisions a new external squad in Remnawave.
func (c *Client) CreateExternalSquad(ctx context.Context, req CreateSquadRequest) (*RemnawaveSquad, error) {
	var resp APIResponse[RemnawaveSquad]
	if err := c.do(ctx, http.MethodPost, APIPathExternalSquads, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// UpdateExternalSquad modifies an existing external squad in Remnawave.
func (c *Client) UpdateExternalSquad(ctx context.Context, req UpdateSquadRequest) (*RemnawaveSquad, error) {
	var resp APIResponse[RemnawaveSquad]
	if err := c.do(ctx, http.MethodPatch, APIPathExternalSquads, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// GetExternalSquadByUUID retrieves a single external squad by UUID.
func (c *Client) GetExternalSquadByUUID(ctx context.Context, uuid string) (*RemnawaveSquad, error) {
	var resp APIResponse[RemnawaveSquad]
	if err := c.do(ctx, http.MethodGet, APIPathExternalSquads+uuid, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// DeleteExternalSquad removes an external squad from Remnawave.
func (c *Client) DeleteExternalSquad(ctx context.Context, uuid string) error {
	return c.do(ctx, http.MethodDelete, APIPathExternalSquads+uuid, nil, nil)
}

// GetExternalSquadNodes returns the accessible nodes for an external squad.
func (c *Client) GetExternalSquadNodes(ctx context.Context, uuid string) ([]RemnawaveNode, error) {
	var resp APIResponse[[]RemnawaveNode]
	if err := c.do(ctx, http.MethodGet, APIPathExternalSquads+uuid+subPathAccessibleNodes, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Response, nil
}

// setUserExternalSquad points a single user at an external squad, or clears the
// assignment when squadUUID is nil.
func (c *Client) setUserExternalSquad(ctx context.Context, userID int64, squadUUID *string) error {
	body := setExternalSquadRequest{ID: userID, ExternalSquadUUID: squadUUID}
	return c.do(ctx, http.MethodPatch, APIPathUsers, body, nil)
}

// AddUsersToExternalSquad assigns each listed user to an external squad.
//
// Remnawave 3 has no per-user bulk endpoint for external squads — the bare
// add-users endpoint applies to EVERY user and takes no body. The assignment
// now lives on the user record as externalSquadUuid, so this patches the
// users one at a time and fails on the first error rather than half-applying
// silently.
func (c *Client) AddUsersToExternalSquad(ctx context.Context, uuid string, userIDs []int64) error {
	for _, id := range userIDs {
		if err := c.setUserExternalSquad(ctx, id, &uuid); err != nil {
			return fmt.Errorf("assign user %d to external squad %s: %w", id, uuid, err)
		}
	}
	return nil
}

// RemoveUsersFromExternalSquad clears the external squad on each listed user.
// See AddUsersToExternalSquad for why this is a per-user patch.
func (c *Client) RemoveUsersFromExternalSquad(ctx context.Context, uuid string, userIDs []int64) error {
	for _, id := range userIDs {
		if err := c.setUserExternalSquad(ctx, id, nil); err != nil {
			return fmt.Errorf("clear external squad %s on user %d: %w", uuid, id, err)
		}
	}
	return nil
}

// ReorderExternalSquads sets the display order for external squads.
func (c *Client) ReorderExternalSquads(ctx context.Context, uuids []string) error {
	body := ReorderRequest{UUIDs: uuids}
	return c.do(ctx, http.MethodPost, APIPathExternalSquads+subPathActionsReorder, body, nil)
}
