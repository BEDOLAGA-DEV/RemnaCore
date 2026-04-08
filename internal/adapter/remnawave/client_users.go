package remnawave

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
)

// Remnawave API path constants for user endpoints.
const (
	APIPathUsersByShortUUID = "/api/users/by-short-uuid/"
	APIPathUsersByUsername  = "/api/users/by-username/"
	APIPathUsersTags        = "/api/users/tags"
	APIPathResetTraffic     = "reset-traffic"
	APIPathRevoke           = "revoke"
)

// GetAllUsers returns a paginated list of all VPN users.
func (c *Client) GetAllUsers(ctx context.Context, params GetAllUsersParams) (*UsersListResponse, error) {
	query := make(map[string]string)
	query["start"] = strconv.Itoa(params.Start)
	query["size"] = strconv.Itoa(params.Size)
	if params.Status != "" {
		query["status"] = params.Status
	}

	var resp APIResponse[UsersListResponse]
	if err := c.doWithQuery(ctx, http.MethodGet, APIPathUsers, nil, query, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetUserByShortUUID retrieves a single VPN user by their short UUID.
func (c *Client) GetUserByShortUUID(ctx context.Context, shortUUID string) (*RemnawaveUserWithTraffic, error) {
	var resp APIResponse[RemnawaveUserWithTraffic]
	if err := c.do(ctx, http.MethodGet, APIPathUsersByShortUUID+shortUUID, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetUserByUsername retrieves a single VPN user by their username.
func (c *Client) GetUserByUsername(ctx context.Context, username string) (*RemnawaveUserWithTraffic, error) {
	var resp APIResponse[RemnawaveUserWithTraffic]
	if err := c.do(ctx, http.MethodGet, APIPathUsersByUsername+username, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// ResetUserTraffic resets the traffic counters for a single VPN user.
func (c *Client) ResetUserTraffic(ctx context.Context, uuid string) error {
	path := fmt.Sprintf("%s%s%s%s", APIPathUsers, uuid, subPathActions, APIPathResetTraffic)
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

// RevokeUser revokes the subscription of a single VPN user.
func (c *Client) RevokeUser(ctx context.Context, uuid string) error {
	path := fmt.Sprintf("%s%s%s%s", APIPathUsers, uuid, subPathActions, APIPathRevoke)
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

// GetUserTags returns all distinct tags assigned to VPN users.
func (c *Client) GetUserTags(ctx context.Context) ([]string, error) {
	var resp APIResponse[[]string]
	if err := c.do(ctx, http.MethodGet, APIPathUsersTags, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}
