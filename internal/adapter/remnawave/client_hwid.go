package remnawave

import (
	"context"
	"net/http"
)

// APIPathHWID is the base path for HWID device management endpoints.
const APIPathHWID = "/api/hwid/devices/"

// HWID endpoint path segments.
const (
	hwidPathDelete    = "delete"
	hwidPathDeleteAll = "delete-all/"
	hwidPathStats     = "stats"
	hwidPathTopUsers  = "top-users"
)

// GetAllHWIDDevices returns all registered HWID devices.
func (c *Client) GetAllHWIDDevices(ctx context.Context) ([]HWIDDevice, error) {
	var resp APIResponse[[]HWIDDevice]
	if err := c.do(ctx, http.MethodGet, APIPathHWID, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// CreateHWIDDevice registers a new HWID device for a user.
func (c *Client) CreateHWIDDevice(ctx context.Context, req CreateHWIDDeviceRequest) (*HWIDDevice, error) {
	var resp APIResponse[HWIDDevice]
	if err := c.do(ctx, http.MethodPost, APIPathHWID, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetUserHWIDDevices returns all HWID devices for a specific user.
func (c *Client) GetUserHWIDDevices(ctx context.Context, userUUID string) ([]HWIDDevice, error) {
	var resp APIResponse[[]HWIDDevice]
	if err := c.do(ctx, http.MethodGet, APIPathHWID+userUUID, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// DeleteHWIDDevice removes a specific HWID device.
func (c *Client) DeleteHWIDDevice(ctx context.Context, req DeleteHWIDDeviceRequest) error {
	return c.do(ctx, http.MethodDelete, APIPathHWID+hwidPathDelete, req, nil)
}

// DeleteAllHWIDDevices removes all HWID devices for a specific user.
func (c *Client) DeleteAllHWIDDevices(ctx context.Context, userUUID string) error {
	return c.do(ctx, http.MethodDelete, APIPathHWID+hwidPathDeleteAll+userUUID, nil, nil)
}

// GetHWIDStats returns aggregate HWID statistics.
func (c *Client) GetHWIDStats(ctx context.Context) (*HWIDStats, error) {
	var resp APIResponse[HWIDStats]
	if err := c.do(ctx, http.MethodGet, APIPathHWID+hwidPathStats, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetTopHWIDUsers returns users with the most registered HWID devices.
func (c *Client) GetTopHWIDUsers(ctx context.Context) ([]TopHWIDUser, error) {
	var resp APIResponse[[]TopHWIDUser]
	if err := c.do(ctx, http.MethodGet, APIPathHWID+hwidPathTopUsers, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}
