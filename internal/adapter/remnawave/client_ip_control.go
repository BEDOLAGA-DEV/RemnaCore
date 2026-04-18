package remnawave

import (
	"context"
	"net/http"
)

// APIPathIPControl is the base path for IP control endpoints.
const APIPathIPControl = "/api/ip-control/"

// IP control endpoint path segments.
const (
	ipControlPathFetchIPs      = "fetch-ips/"
	ipControlPathFetchResult   = "fetch-ips/result/"
	ipControlPathDrop          = "drop-connections"
	ipControlPathFetchUsersIPs = "fetch-users-ips/"
	ipControlPathUsersResult   = "fetch-users-ips/result/"
)

// FetchUserIPs initiates an asynchronous job to fetch IPs for a specific user.
func (c *Client) FetchUserIPs(ctx context.Context, userUUID string) (*IPFetchJob, error) {
	var resp APIResponse[IPFetchJob]
	if err := c.do(ctx, http.MethodPost, APIPathIPControl+ipControlPathFetchIPs+userUUID, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// GetFetchIPsResult returns the result of an asynchronous IP fetch job.
func (c *Client) GetFetchIPsResult(ctx context.Context, jobID string) (*IPFetchResult, error) {
	var resp APIResponse[IPFetchResult]
	if err := c.do(ctx, http.MethodGet, APIPathIPControl+ipControlPathFetchResult+jobID, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// DropConnections terminates active connections for a user, optionally on a specific node.
func (c *Client) DropConnections(ctx context.Context, req DropConnectionsRequest) error {
	return c.do(ctx, http.MethodPost, APIPathIPControl+ipControlPathDrop, req, nil)
}

// FetchNodeUsersIPs initiates an asynchronous job to fetch IPs of all users on a node.
func (c *Client) FetchNodeUsersIPs(ctx context.Context, nodeUUID string) (*IPFetchJob, error) {
	var resp APIResponse[IPFetchJob]
	if err := c.do(ctx, http.MethodPost, APIPathIPControl+ipControlPathFetchUsersIPs+nodeUUID, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// GetFetchNodeUsersIPsResult returns the result of a node users IP fetch job.
func (c *Client) GetFetchNodeUsersIPsResult(ctx context.Context, jobID string) (*IPFetchResult, error) {
	var resp APIResponse[IPFetchResult]
	if err := c.do(ctx, http.MethodGet, APIPathIPControl+ipControlPathUsersResult+jobID, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}
