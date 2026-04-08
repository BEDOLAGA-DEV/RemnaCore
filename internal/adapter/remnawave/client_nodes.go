package remnawave

import (
	"context"
	"net/http"
)

// Remnawave API path constants for node action endpoints.
const (
	APIPathNodesTags      = "/api/nodes/tags"
	APIPathNodesActions   = "/api/nodes/actions/"
	APIPathNodeEnable     = "enable"
	APIPathNodeDisable    = "disable"
	APIPathNodeRestart    = "restart"
	APIPathNodeRestartAll = "restart-all"
	APIPathNodeReorder    = "reorder"
)

// CreateNode creates a new proxy node in Remnawave.
func (c *Client) CreateNode(ctx context.Context, req CreateNodeRequest) (*RemnawaveNode, error) {
	var resp APIResponse[RemnawaveNode]
	if err := c.do(ctx, http.MethodPost, APIPathNodes, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetNodeByUUID retrieves a single proxy node by its UUID.
func (c *Client) GetNodeByUUID(ctx context.Context, uuid string) (*RemnawaveNode, error) {
	var resp APIResponse[RemnawaveNode]
	if err := c.do(ctx, http.MethodGet, APIPathNodes+uuid, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// UpdateNode modifies an existing proxy node.
func (c *Client) UpdateNode(ctx context.Context, req UpdateNodeRequest) (*RemnawaveNode, error) {
	var resp APIResponse[RemnawaveNode]
	if err := c.do(ctx, http.MethodPatch, APIPathNodes, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// DeleteNode removes a proxy node from Remnawave.
func (c *Client) DeleteNode(ctx context.Context, uuid string) error {
	return c.do(ctx, http.MethodDelete, APIPathNodes+uuid, nil, nil)
}

// EnableNode activates a proxy node.
func (c *Client) EnableNode(ctx context.Context, uuid string) error {
	return c.do(ctx, http.MethodPost, APIPathNodes+uuid+subPathActions+APIPathNodeEnable, nil, nil)
}

// DisableNode deactivates a proxy node.
func (c *Client) DisableNode(ctx context.Context, uuid string) error {
	return c.do(ctx, http.MethodPost, APIPathNodes+uuid+subPathActions+APIPathNodeDisable, nil, nil)
}

// RestartNode restarts a single proxy node.
func (c *Client) RestartNode(ctx context.Context, uuid string) error {
	return c.do(ctx, http.MethodPost, APIPathNodes+uuid+subPathActions+APIPathNodeRestart, nil, nil)
}

// ResetNodeTraffic resets traffic counters for a single proxy node.
func (c *Client) ResetNodeTraffic(ctx context.Context, uuid string) error {
	return c.do(ctx, http.MethodPost, APIPathNodes+uuid+subPathActions+APIPathResetTraffic, nil, nil)
}

// RestartAllNodes restarts all proxy nodes.
func (c *Client) RestartAllNodes(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, APIPathNodesActions+APIPathNodeRestartAll, nil, nil)
}

// ReorderNodes sets the display order for proxy nodes.
func (c *Client) ReorderNodes(ctx context.Context, uuids []string) error {
	return c.do(ctx, http.MethodPost, APIPathNodesActions+APIPathNodeReorder, ReorderRequest{UUIDs: uuids}, nil)
}

// GetNodeTags returns all distinct tags assigned to proxy nodes.
func (c *Client) GetNodeTags(ctx context.Context) ([]string, error) {
	var resp APIResponse[[]string]
	if err := c.do(ctx, http.MethodGet, APIPathNodesTags, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}
