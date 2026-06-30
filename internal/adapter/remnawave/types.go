// Package remnawave provides an HTTP client, webhook handler, and
// Anti-Corruption Layer for integrating with the Remnawave VPN panel API.
package remnawave

import (
	"encoding/json"
	"time"
)

// Remnawave user status strings returned by the VPN panel API.
const (
	RemnawaveStatusActive   = "ACTIVE"
	RemnawaveStatusDisabled = "DISABLED"
	RemnawaveStatusExpired  = "EXPIRED"
	RemnawaveStatusLimited  = "LIMITED"
)

// CreateUserRequest is the payload sent to Remnawave to provision a new VPN user.
type CreateUserRequest struct {
	Username             string    `json:"username"`
	TrafficLimitBytes    float64   `json:"trafficLimitBytes"`
	ExpireAt             time.Time `json:"expireAt"`
	ActiveInternalSquads []string  `json:"activeInternalSquads,omitempty"`
}

// UpdateUserRequest is the payload sent to Remnawave to modify an existing VPN user.
type UpdateUserRequest struct {
	UUID                 string    `json:"uuid"`
	Username             string    `json:"username,omitempty"`
	TrafficLimitBytes    float64   `json:"trafficLimitBytes,omitempty"`
	ExpireAt             time.Time `json:"expireAt,omitempty"`
	ActiveInternalSquads []string  `json:"activeInternalSquads,omitempty"`
}

// APIResponse is the generic envelope returned by Remnawave REST endpoints.
// The Remnawave API wraps responses in a "response" field.
type APIResponse[T any] struct {
	Response T `json:"response"`
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

// RemnawaveUserTraffic is the nested traffic object on the 2.8.0 extended user
// schema (GET /api/users/{uuid}, by-short-uuid, by-username, list).
type RemnawaveUserTraffic struct {
	UsedTrafficBytes         float64    `json:"usedTrafficBytes"`
	LifetimeUsedTrafficBytes float64    `json:"lifetimeUsedTrafficBytes"`
	OnlineAt                 *time.Time `json:"onlineAt"`
	FirstConnectedAt         *time.Time `json:"firstConnectedAt"`
	LastConnectedNodeUuid    *string    `json:"lastConnectedNodeUuid"`
}

// RemnawaveUserWithTraffic extends RemnawaveUser with traffic consumption data.
// Traffic counters are nested under userTraffic; lastTrafficResetAt stays
// top-level on the user object (nullable).
type RemnawaveUserWithTraffic struct {
	RemnawaveUser
	UserTraffic        RemnawaveUserTraffic `json:"userTraffic"`
	LastTrafficResetAt *time.Time           `json:"lastTrafficResetAt"`
}

// UsedTrafficBytesInt returns used traffic as int64 (bytes are whole numbers;
// the wire type is a JSON number). Single conversion point for the float→int
// domain boundary.
func (u RemnawaveUserWithTraffic) UsedTrafficBytesInt() int64 {
	return int64(u.UserTraffic.UsedTrafficBytes)
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
