package remnawave

import (
	"context"
	"net/http"
)

// APIPathSystem is the base path for system endpoints.
const APIPathSystem = "/api/system/"

// System endpoint path segments.
const (
	systemPathHealth         = "health"
	systemPathMetadata       = "metadata"
	systemPathStats          = "stats"
	systemPathStatsBandwidth = "stats/bandwidth"
	systemPathStatsNodes     = "stats/nodes"
	systemPathNodesMetrics   = "nodes/metrics"
	systemPathStatsRecap     = "stats/recap"
)

// GetSystemHealth returns the health status of the Remnawave instance.
func (c *Client) GetSystemHealth(ctx context.Context) (*SystemHealth, error) {
	var resp APIResponse[SystemHealth]
	if err := c.do(ctx, http.MethodGet, APIPathSystem+systemPathHealth, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetSystemMetadata returns system-level metadata from Remnawave.
func (c *Client) GetSystemMetadata(ctx context.Context) (*SystemMetadata, error) {
	var resp APIResponse[SystemMetadata]
	if err := c.do(ctx, http.MethodGet, APIPathSystem+systemPathMetadata, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetSystemStats returns high-level system statistics.
func (c *Client) GetSystemStats(ctx context.Context) (*SystemStats, error) {
	var resp APIResponse[SystemStats]
	if err := c.do(ctx, http.MethodGet, APIPathSystem+systemPathStats, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetBandwidthSystemStats returns bandwidth-specific system statistics.
func (c *Client) GetBandwidthSystemStats(ctx context.Context) (*BandwidthSystemStats, error) {
	var resp APIResponse[BandwidthSystemStats]
	if err := c.do(ctx, http.MethodGet, APIPathSystem+systemPathStatsBandwidth, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetNodesSystemStats returns node-specific system statistics.
func (c *Client) GetNodesSystemStats(ctx context.Context) (*NodesSystemStats, error) {
	var resp APIResponse[NodesSystemStats]
	if err := c.do(ctx, http.MethodGet, APIPathSystem+systemPathStatsNodes, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetNodesMetrics returns detailed metrics for all nodes.
func (c *Client) GetNodesMetrics(ctx context.Context) ([]NodeMetrics, error) {
	var resp APIResponse[[]NodeMetrics]
	if err := c.do(ctx, http.MethodGet, APIPathSystem+systemPathNodesMetrics, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetStatsRecap returns a recap/summary of system statistics.
func (c *Client) GetStatsRecap(ctx context.Context) (*StatsRecap, error) {
	var resp APIResponse[StatsRecap]
	if err := c.do(ctx, http.MethodGet, APIPathSystem+systemPathStatsRecap, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}
