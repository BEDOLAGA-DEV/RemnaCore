package remnawave

import (
	"context"
	"net/http"
)

// APIPathBandwidthStats is the base path for bandwidth statistics endpoints.
const APIPathBandwidthStats = "/api/bandwidth-stats/"

// Bandwidth stats endpoint path segments.
const (
	bandwidthPathNodes      = "nodes/"
	bandwidthPathRealtime   = "nodes/realtime"
	bandwidthPathNodeUsers  = "/users"
	bandwidthPathUsers      = "users/"
)

// GetNodesBandwidthStats returns bandwidth statistics for all nodes.
func (c *Client) GetNodesBandwidthStats(ctx context.Context) ([]NodeBandwidthStats, error) {
	var resp APIResponse[[]NodeBandwidthStats]
	if err := c.do(ctx, http.MethodGet, APIPathBandwidthStats+bandwidthPathNodes, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetRealtimeNodeMetrics returns real-time bandwidth metrics for all nodes.
func (c *Client) GetRealtimeNodeMetrics(ctx context.Context) ([]RealtimeNodeMetrics, error) {
	var resp APIResponse[[]RealtimeNodeMetrics]
	if err := c.do(ctx, http.MethodGet, APIPathBandwidthStats+bandwidthPathRealtime, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetNodeUsersBandwidth returns bandwidth statistics for all users on a specific node.
func (c *Client) GetNodeUsersBandwidth(ctx context.Context, nodeUUID string) ([]UserBandwidthStats, error) {
	var resp APIResponse[[]UserBandwidthStats]
	path := APIPathBandwidthStats + bandwidthPathNodes + nodeUUID + bandwidthPathNodeUsers
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetUserBandwidthStats returns bandwidth statistics for a specific user.
func (c *Client) GetUserBandwidthStats(ctx context.Context, userUUID string) (*UserBandwidthStats, error) {
	var resp APIResponse[UserBandwidthStats]
	if err := c.do(ctx, http.MethodGet, APIPathBandwidthStats+bandwidthPathUsers+userUUID, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}
