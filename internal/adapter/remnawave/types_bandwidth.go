package remnawave

import "time"

// NodeBandwidthStats represents bandwidth statistics for a single node.
type NodeBandwidthStats struct {
	UUID          string `json:"uuid"`
	Name          string `json:"name"`
	UploadBytes   int64  `json:"uploadBytes"`
	DownloadBytes int64  `json:"downloadBytes"`
	TotalBytes    int64  `json:"totalBytes"`
	Date          string `json:"date"`
}

// RealtimeNodeMetrics represents real-time bandwidth metrics for a node.
type RealtimeNodeMetrics struct {
	UUID               string  `json:"uuid"`
	Name               string  `json:"name"`
	UploadSpeedBytes   float64 `json:"uploadSpeedBytes"`
	DownloadSpeedBytes float64 `json:"downloadSpeedBytes"`
	ActiveConnections  int     `json:"activeConnections"`
	CollectedAt        string  `json:"collectedAt"`
}

// UserBandwidthStats represents bandwidth statistics for a single user.
type UserBandwidthStats struct {
	UUID          string    `json:"uuid"`
	Username      string    `json:"username"`
	UploadBytes   int64     `json:"uploadBytes"`
	DownloadBytes int64     `json:"downloadBytes"`
	TotalBytes    int64     `json:"totalBytes"`
	Date          string    `json:"date"`
	LastOnlineAt  time.Time `json:"lastOnlineAt"`
}
