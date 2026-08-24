// Package remnawave provides an HTTP client, webhook handler, and
// Anti-Corruption Layer for integrating with the Remnawave VPN panel API.
package remnawave

import (
	"encoding/json"
	"strconv"
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
// Remnawave 3 identifies users by a numeric id; the string uuid of 2.x is gone.
type UpdateUserRequest struct {
	ID                   int64     `json:"id"`
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
//
// Remnawave 3 replaced the string "uuid" with a numeric "id" and moved the
// former UUID identity to "vlessUuid". ID is the value every user endpoint
// addresses; VlessUUID is protocol identity and is not an API key.
type RemnawaveUser struct {
	ID                int64     `json:"id"`
	VlessUUID         string    `json:"vlessUuid"`
	Username          string    `json:"username"`
	Status            string    `json:"status"`
	TrafficLimitBytes float64   `json:"trafficLimitBytes"`
	ExpireAt          time.Time `json:"expireAt"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
	SubscriptionURL   string    `json:"subscriptionUrl"`
	ShortUUID         string    `json:"shortUuid"`
}

// UserRef renders the numeric user id the way the platform stores and passes
// it around — as a string. Multi-Sub keeps a text identifier for the bound
// Remnawave user, so this is the single conversion point on the way out.
func (u RemnawaveUser) UserRef() string {
	return strconv.FormatInt(u.ID, 10)
}

// RemnawaveUserTraffic is the nested traffic object on the extended user
// schema (GET /api/users/{userId}, by-short-uuid, by-username, list).
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
	Meta      WebhookMeta     `json:"meta"`
}

// WebhookMeta carries the optional top-level meta object Remnawave 2.8.0 attaches
// to certain user events. Pointer fields distinguish "absent" from a zero value.
//   - Expiration: signed hours for the unified user.expiration event
//     (negative = hours before expiry, positive = hours after expiry).
//   - NotConnectedAfterHours: threshold for user.not_connected (not yet consumed).
type WebhookMeta struct {
	Expiration             *int `json:"expiration"`
	NotConnectedAfterHours *int `json:"notConnectedAfterHours"`
}
