// Package remnawave provides an HTTP client, webhook handler, and
// Anti-Corruption Layer for integrating with the Remnawave VPN panel API.
package remnawave

import (
	"encoding/json"
	"time"
)

// Remnawave user status strings returned by the VPN panel API.
const (
	RemnawaveStatusActive   = "active"
	RemnawaveStatusDisabled = "disabled"
	RemnawaveStatusExpired  = "expired"
	RemnawaveStatusLimited  = "limited"
)

// CreateUserRequest is the payload sent to Remnawave to provision a new VPN user.
type CreateUserRequest struct {
	Username           string    `json:"username"`
	TrafficLimitBytes  float64   `json:"trafficLimitBytes"`
	ExpireAt           time.Time `json:"expireAt"`
	ActiveUserInbounds []string  `json:"activeUserInbounds,omitempty"`
}

// UpdateUserRequest is the payload sent to Remnawave to modify an existing VPN user.
type UpdateUserRequest struct {
	UUID              string    `json:"uuid"`
	Username          string    `json:"username,omitempty"`
	TrafficLimitBytes float64   `json:"trafficLimitBytes,omitempty"`
	ExpireAt          time.Time `json:"expireAt"`
}

// APIResponse is the generic envelope returned by Remnawave REST endpoints.
type APIResponse[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    T      `json:"data"`
}

// RemnawaveUser represents a VPN user as returned by Remnawave.
type RemnawaveUser struct {
	UUID              string    `json:"uuid"`
	Username          string    `json:"username"`
	Status            string    `json:"status"`
	TrafficLimitBytes float64   `json:"trafficLimitBytes"`
	ExpireAt          time.Time `json:"expireAt"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
	SubscriptionURL   string    `json:"subscriptionUrl"`
	ShortUUID         string    `json:"shortUuid"`
}

// RemnawaveUserWithTraffic extends RemnawaveUser with traffic consumption data.
type RemnawaveUserWithTraffic struct {
	RemnawaveUser
	UsedTrafficBytes   int64     `json:"usedTrafficBytes"`
	DownloadBytes      int64     `json:"downloadBytes"`
	UploadBytes        int64     `json:"uploadBytes"`
	LifetimeUsedBytes  int64     `json:"lifetimeUsedTrafficBytes"`
	LastTrafficResetAt time.Time `json:"lastTrafficResetAt"`
	OnlineAt           time.Time `json:"onlineAt"`
}

// RemnawaveNode represents a proxy node in the Remnawave infrastructure.
type RemnawaveNode struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	Address     string `json:"address"`
	Port        int    `json:"port"`
	IsConnected bool   `json:"isConnected"`
	TrafficUsed int64  `json:"trafficUsedBytes"`
}

// WebhookPayload is the top-level structure Remnawave sends to webhook endpoints.
type WebhookPayload struct {
	Scope     string          `json:"scope"`
	Event     string          `json:"event"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}
