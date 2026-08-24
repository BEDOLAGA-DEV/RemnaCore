package remnawave

import (
	"context"
	"net/http"
)

// APIPathNodePlugins is the base path for node plugin management endpoints.
const APIPathNodePlugins = "/api/node-plugins/"

// Node plugin endpoint path segments.
const (
	nodePluginsPathClone = "actions/clone"
)

// GetNodePlugins returns all node plugins.
func (c *Client) GetNodePlugins(ctx context.Context) ([]NodePlugin, error) {
	var resp APIResponse[[]NodePlugin]
	if err := c.do(ctx, http.MethodGet, APIPathNodePlugins, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Response, nil
}

// GetNodePluginByUUID returns a single node plugin by its UUID.
func (c *Client) GetNodePluginByUUID(ctx context.Context, uuid string) (*NodePlugin, error) {
	var resp APIResponse[NodePlugin]
	if err := c.do(ctx, http.MethodGet, APIPathNodePlugins+uuid, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// CreateNodePlugin creates a new node plugin.
func (c *Client) CreateNodePlugin(ctx context.Context, req CreateNodePluginRequest) (*NodePlugin, error) {
	var resp APIResponse[NodePlugin]
	if err := c.do(ctx, http.MethodPost, APIPathNodePlugins, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// UpdateNodePlugin modifies an existing node plugin.
func (c *Client) UpdateNodePlugin(ctx context.Context, req UpdateNodePluginRequest) (*NodePlugin, error) {
	var resp APIResponse[NodePlugin]
	if err := c.do(ctx, http.MethodPatch, APIPathNodePlugins, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// DeleteNodePlugin removes a node plugin by its UUID.
func (c *Client) DeleteNodePlugin(ctx context.Context, uuid string) error {
	return c.do(ctx, http.MethodDelete, APIPathNodePlugins+uuid, nil, nil)
}

// ReorderNodePlugins sets the ordering of node plugins by their UUIDs.
func (c *Client) ReorderNodePlugins(ctx context.Context, uuids []string) error {
	req := ReorderRequest{UUIDs: uuids}
	return c.do(ctx, http.MethodPost, APIPathNodePlugins+subPathActionsReorder, req, nil)
}

// CloneNodePlugin duplicates an existing node plugin. Remnawave 3 moved the
// source uuid out of the path and into the request body.
func (c *Client) CloneNodePlugin(ctx context.Context, uuid string) (*NodePlugin, error) {
	var resp APIResponse[NodePlugin]
	body := cloneNodePluginRequest{CloneFromUUID: uuid}
	if err := c.do(ctx, http.MethodPost, APIPathNodePlugins+nodePluginsPathClone, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}
