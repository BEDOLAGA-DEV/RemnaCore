package remnawave

import (
	"context"
	"net/http"
)

// Remnawave API path constant for config profile endpoints.
const APIPathConfigProfiles = "/api/config-profiles/"

// Config profile sub-path constants.
const (
	subPathInbounds    = "/inbounds"
	subPathAllInbounds = "inbounds"
)

// GetConfigProfiles returns all config profiles from Remnawave.
func (c *Client) GetConfigProfiles(ctx context.Context) ([]RemnawaveConfigProfile, error) {
	var resp APIResponse[[]RemnawaveConfigProfile]
	if err := c.do(ctx, http.MethodGet, APIPathConfigProfiles, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Response, nil
}

// CreateConfigProfile provisions a new config profile in Remnawave.
func (c *Client) CreateConfigProfile(ctx context.Context, req CreateConfigProfileRequest) (*RemnawaveConfigProfile, error) {
	var resp APIResponse[RemnawaveConfigProfile]
	if err := c.do(ctx, http.MethodPost, APIPathConfigProfiles, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// UpdateConfigProfile modifies an existing config profile in Remnawave.
func (c *Client) UpdateConfigProfile(ctx context.Context, req UpdateConfigProfileRequest) (*RemnawaveConfigProfile, error) {
	var resp APIResponse[RemnawaveConfigProfile]
	if err := c.do(ctx, http.MethodPatch, APIPathConfigProfiles, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// GetConfigProfileByUUID retrieves a single config profile by UUID.
func (c *Client) GetConfigProfileByUUID(ctx context.Context, uuid string) (*RemnawaveConfigProfile, error) {
	var resp APIResponse[RemnawaveConfigProfile]
	if err := c.do(ctx, http.MethodGet, APIPathConfigProfiles+uuid, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// DeleteConfigProfile removes a config profile from Remnawave.
func (c *Client) DeleteConfigProfile(ctx context.Context, uuid string) error {
	return c.do(ctx, http.MethodDelete, APIPathConfigProfiles+uuid, nil, nil)
}

// GetConfigProfileInbounds returns the inbounds associated with a config profile.
func (c *Client) GetConfigProfileInbounds(ctx context.Context, uuid string) ([]RemnawaveInbound, error) {
	var resp inboundsListResponse
	if err := c.do(ctx, http.MethodGet, APIPathConfigProfiles+uuid+subPathInbounds, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Response.Inbounds, nil
}

// inboundsListResponse matches the nested response: {"response": {"total": N, "inbounds": [...]}}
type inboundsListResponse struct {
	Response struct {
		Total    int                `json:"total"`
		Inbounds []RemnawaveInbound `json:"inbounds"`
	} `json:"response"`
}

// GetAllInbounds returns all inbound proxy entries from Remnawave.
func (c *Client) GetAllInbounds(ctx context.Context) ([]RemnawaveInbound, error) {
	var resp inboundsListResponse
	if err := c.do(ctx, http.MethodGet, APIPathConfigProfiles+subPathAllInbounds, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Response.Inbounds, nil
}

// ReorderConfigProfiles sets the display order for config profiles.
func (c *Client) ReorderConfigProfiles(ctx context.Context, uuids []string) error {
	body := ReorderRequest{UUIDs: uuids}
	return c.do(ctx, http.MethodPost, APIPathConfigProfiles+subPathActionsReorder, body, nil)
}
