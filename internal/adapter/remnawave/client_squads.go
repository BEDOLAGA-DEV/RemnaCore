package remnawave

import (
	"context"
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
	subPathBulkAddUsers    = "/bulk-actions/add-users"
	subPathBulkRemoveUsers = "/bulk-actions/remove-users"
)

// ---------------------------------------------------------------------------
// Internal Squads
// ---------------------------------------------------------------------------

// GetInternalSquads returns all internal squads from Remnawave.
func (c *Client) GetInternalSquads(ctx context.Context) ([]RemnawaveSquad, error) {
	var resp APIResponse[[]RemnawaveSquad]
	if err := c.do(ctx, http.MethodGet, APIPathInternalSquads, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// CreateInternalSquad provisions a new internal squad in Remnawave.
func (c *Client) CreateInternalSquad(ctx context.Context, req CreateSquadRequest) (*RemnawaveSquad, error) {
	var resp APIResponse[RemnawaveSquad]
	if err := c.do(ctx, http.MethodPost, APIPathInternalSquads, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// UpdateInternalSquad modifies an existing internal squad in Remnawave.
func (c *Client) UpdateInternalSquad(ctx context.Context, req UpdateSquadRequest) (*RemnawaveSquad, error) {
	var resp APIResponse[RemnawaveSquad]
	if err := c.do(ctx, http.MethodPatch, APIPathInternalSquads, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetInternalSquadByUUID retrieves a single internal squad by UUID.
func (c *Client) GetInternalSquadByUUID(ctx context.Context, uuid string) (*RemnawaveSquad, error) {
	var resp APIResponse[RemnawaveSquad]
	if err := c.do(ctx, http.MethodGet, APIPathInternalSquads+uuid, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
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
	return resp.Data, nil
}

// AddUsersToInternalSquad adds users to an internal squad by their UUIDs.
func (c *Client) AddUsersToInternalSquad(ctx context.Context, uuid string, userUUIDs []string) error {
	body := BulkUsersRequest{UserUUIDs: userUUIDs}
	return c.do(ctx, http.MethodPost, APIPathInternalSquads+uuid+subPathBulkAddUsers, body, nil)
}

// RemoveUsersFromInternalSquad removes users from an internal squad by their UUIDs.
func (c *Client) RemoveUsersFromInternalSquad(ctx context.Context, uuid string, userUUIDs []string) error {
	body := BulkUsersRequest{UserUUIDs: userUUIDs}
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
	var resp APIResponse[[]RemnawaveSquad]
	if err := c.do(ctx, http.MethodGet, APIPathExternalSquads, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// CreateExternalSquad provisions a new external squad in Remnawave.
func (c *Client) CreateExternalSquad(ctx context.Context, req CreateSquadRequest) (*RemnawaveSquad, error) {
	var resp APIResponse[RemnawaveSquad]
	if err := c.do(ctx, http.MethodPost, APIPathExternalSquads, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// UpdateExternalSquad modifies an existing external squad in Remnawave.
func (c *Client) UpdateExternalSquad(ctx context.Context, req UpdateSquadRequest) (*RemnawaveSquad, error) {
	var resp APIResponse[RemnawaveSquad]
	if err := c.do(ctx, http.MethodPatch, APIPathExternalSquads, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetExternalSquadByUUID retrieves a single external squad by UUID.
func (c *Client) GetExternalSquadByUUID(ctx context.Context, uuid string) (*RemnawaveSquad, error) {
	var resp APIResponse[RemnawaveSquad]
	if err := c.do(ctx, http.MethodGet, APIPathExternalSquads+uuid, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
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
	return resp.Data, nil
}

// AddUsersToExternalSquad adds users to an external squad by their UUIDs.
func (c *Client) AddUsersToExternalSquad(ctx context.Context, uuid string, userUUIDs []string) error {
	body := BulkUsersRequest{UserUUIDs: userUUIDs}
	return c.do(ctx, http.MethodPost, APIPathExternalSquads+uuid+subPathBulkAddUsers, body, nil)
}

// RemoveUsersFromExternalSquad removes users from an external squad by their UUIDs.
func (c *Client) RemoveUsersFromExternalSquad(ctx context.Context, uuid string, userUUIDs []string) error {
	body := BulkUsersRequest{UserUUIDs: userUUIDs}
	return c.do(ctx, http.MethodDelete, APIPathExternalSquads+uuid+subPathBulkRemoveUsers, body, nil)
}

// ReorderExternalSquads sets the display order for external squads.
func (c *Client) ReorderExternalSquads(ctx context.Context, uuids []string) error {
	body := ReorderRequest{UUIDs: uuids}
	return c.do(ctx, http.MethodPost, APIPathExternalSquads+subPathActionsReorder, body, nil)
}
