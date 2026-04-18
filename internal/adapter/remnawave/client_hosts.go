package remnawave

import (
	"context"
	"net/http"
)

// Remnawave API path constants for host endpoints.
const (
	APIPathHosts        = "/api/hosts/"
	APIPathHostsActions = "/api/hosts/actions/"
	APIPathHostsBulk    = "/api/hosts/bulk/"
	APIPathHostsTags    = "/api/hosts/tags"
	APIPathHostReorder  = "reorder"
	APIPathHostEnable   = "enable"
	APIPathHostDisable  = "disable"
	APIPathHostDelete   = "delete"
)

// CreateHost creates a new host in Remnawave.
func (c *Client) CreateHost(ctx context.Context, req CreateHostRequest) (*RemnawaveHost, error) {
	var resp APIResponse[RemnawaveHost]
	if err := c.do(ctx, http.MethodPost, APIPathHosts, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// UpdateHost modifies an existing host.
func (c *Client) UpdateHost(ctx context.Context, req UpdateHostRequest) (*RemnawaveHost, error) {
	var resp APIResponse[RemnawaveHost]
	if err := c.do(ctx, http.MethodPatch, APIPathHosts, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// GetAllHosts returns all hosts registered in Remnawave.
func (c *Client) GetAllHosts(ctx context.Context) ([]RemnawaveHost, error) {
	var resp APIResponse[[]RemnawaveHost]
	if err := c.do(ctx, http.MethodGet, APIPathHosts, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Response, nil
}

// GetHostByUUID retrieves a single host by its UUID.
func (c *Client) GetHostByUUID(ctx context.Context, uuid string) (*RemnawaveHost, error) {
	var resp APIResponse[RemnawaveHost]
	if err := c.do(ctx, http.MethodGet, APIPathHosts+uuid, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// DeleteHost removes a host from Remnawave.
func (c *Client) DeleteHost(ctx context.Context, uuid string) error {
	return c.do(ctx, http.MethodDelete, APIPathHosts+uuid, nil, nil)
}

// ReorderHosts sets the display order for hosts.
func (c *Client) ReorderHosts(ctx context.Context, uuids []string) error {
	return c.do(ctx, http.MethodPost, APIPathHostsActions+APIPathHostReorder, ReorderRequest{UUIDs: uuids}, nil)
}

// GetHostTags returns all distinct tags assigned to hosts.
func (c *Client) GetHostTags(ctx context.Context) ([]string, error) {
	var resp APIResponse[[]string]
	if err := c.do(ctx, http.MethodGet, APIPathHostsTags, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Response, nil
}

// BulkEnableHosts enables the specified hosts.
func (c *Client) BulkEnableHosts(ctx context.Context, uuids []string) error {
	return c.do(ctx, http.MethodPost, APIPathHostsBulk+APIPathHostEnable, UUIDsRequest{UUIDs: uuids}, nil)
}

// BulkDisableHosts disables the specified hosts.
func (c *Client) BulkDisableHosts(ctx context.Context, uuids []string) error {
	return c.do(ctx, http.MethodPost, APIPathHostsBulk+APIPathHostDisable, UUIDsRequest{UUIDs: uuids}, nil)
}

// BulkDeleteHosts deletes the specified hosts.
func (c *Client) BulkDeleteHosts(ctx context.Context, uuids []string) error {
	return c.do(ctx, http.MethodPost, APIPathHostsBulk+APIPathHostDelete, UUIDsRequest{UUIDs: uuids}, nil)
}
