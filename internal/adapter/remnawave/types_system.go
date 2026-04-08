package remnawave

import "encoding/json"

// SystemHealth represents the health check response from Remnawave.
type SystemHealth struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Uptime  int64  `json:"uptime"`
}

// SystemMetadata contains system-level metadata from Remnawave.
type SystemMetadata struct {
	Version     string          `json:"version"`
	Environment string          `json:"environment"`
	Extra       json.RawMessage `json:"extra,omitempty"`
}

// SystemStats contains high-level system statistics.
type SystemStats struct {
	TotalUsers      int   `json:"totalUsers"`
	ActiveUsers     int   `json:"activeUsers"`
	TotalNodes      int   `json:"totalNodes"`
	ConnectedNodes  int   `json:"connectedNodes"`
	TotalTraffic    int64 `json:"totalTraffic"`
}

// BandwidthSystemStats contains bandwidth-specific system statistics.
type BandwidthSystemStats struct {
	TotalUploadBytes   int64 `json:"totalUploadBytes"`
	TotalDownloadBytes int64 `json:"totalDownloadBytes"`
	TotalBytes         int64 `json:"totalBytes"`
}

// NodesSystemStats contains node-specific system statistics.
type NodesSystemStats struct {
	TotalNodes     int `json:"totalNodes"`
	ConnectedNodes int `json:"connectedNodes"`
	DisabledNodes  int `json:"disabledNodes"`
}

// NodeMetrics represents metrics for a single node.
type NodeMetrics struct {
	UUID              string  `json:"uuid"`
	Name              string  `json:"name"`
	IsConnected       bool    `json:"isConnected"`
	UploadSpeedBytes  float64 `json:"uploadSpeedBytes"`
	DownloadSpeedBytes float64 `json:"downloadSpeedBytes"`
	ActiveConnections int     `json:"activeConnections"`
}

// StatsRecap contains a recap/summary of system statistics.
type StatsRecap struct {
	TotalUsers      int   `json:"totalUsers"`
	ActiveUsers     int   `json:"activeUsers"`
	TotalNodes      int   `json:"totalNodes"`
	TotalTraffic    int64 `json:"totalTraffic"`
	NewUsersToday   int   `json:"newUsersToday"`
}

// IPFetchJob represents an asynchronous IP fetch job.
type IPFetchJob struct {
	JobID  string `json:"jobId"`
	Status string `json:"status"`
}

// IPFetchResult contains the result of a completed IP fetch job.
type IPFetchResult struct {
	JobID  string   `json:"jobId"`
	Status string   `json:"status"`
	IPs    []string `json:"ips,omitempty"`
}

// DropConnectionsRequest is the payload to drop active connections.
type DropConnectionsRequest struct {
	UserUUID string `json:"userUuid"`
	NodeUUID string `json:"nodeUuid,omitempty"`
}
