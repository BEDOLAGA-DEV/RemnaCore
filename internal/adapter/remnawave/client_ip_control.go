package remnawave

import (
	"context"
	"net/http"
	"strconv"
)

// APIPathConnections is the base path for connection inspection and control.
// Remnawave 3 replaced the /api/ip-control/ family with this one; the fetch
// endpoints stayed asynchronous — a POST starts a job, a GET on the job id
// returns the result.
const APIPathConnections = "/api/connections/"

// Connections endpoint path segments.
const (
	connectionsPathByUser = "by-user/"
	connectionsPathByNode = "by-node/"
	connectionsPathDrop   = "drop"
)

// FetchUserIPs starts an asynchronous job collecting the connections of one user.
func (c *Client) FetchUserIPs(ctx context.Context, userID int64) (*IPFetchJob, error) {
	var resp APIResponse[IPFetchJob]
	path := APIPathConnections + connectionsPathByUser + strconv.FormatInt(userID, 10)
	if err := c.do(ctx, http.MethodPost, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// GetFetchIPsResult returns the result of a per-user connections job.
func (c *Client) GetFetchIPsResult(ctx context.Context, jobID string) (*IPFetchResult, error) {
	var resp APIResponse[IPFetchResult]
	if err := c.do(ctx, http.MethodGet, APIPathConnections+connectionsPathByUser+jobID, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// DropConnections terminates active connections selected by the request.
func (c *Client) DropConnections(ctx context.Context, req DropConnectionsRequest) error {
	return c.do(ctx, http.MethodPost, APIPathConnections+connectionsPathDrop, req, nil)
}

// FetchNodeUsersIPs starts an asynchronous job collecting every connection on a node.
func (c *Client) FetchNodeUsersIPs(ctx context.Context, nodeUUID string) (*IPFetchJob, error) {
	var resp APIResponse[IPFetchJob]
	if err := c.do(ctx, http.MethodPost, APIPathConnections+connectionsPathByNode+nodeUUID, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// GetFetchNodeUsersIPsResult returns the result of a per-node connections job.
func (c *Client) GetFetchNodeUsersIPsResult(ctx context.Context, jobID string) (*IPFetchResult, error) {
	var resp APIResponse[IPFetchResult]
	if err := c.do(ctx, http.MethodGet, APIPathConnections+connectionsPathByNode+jobID, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}
