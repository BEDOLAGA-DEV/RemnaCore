package remnawave

import (
	"time"
)

// --- Bulk operation request types ---

// StatusRequest is a request body carrying a single status string.
type StatusRequest struct {
	Status string `json:"status"`
}

// HostUUIDsRequest is a request body carrying a list of host UUIDs. Hosts kept
// string UUIDs in Remnawave 3 — only users moved to numeric ids — so this is
// deliberately separate from UserIDsRequest.
type HostUUIDsRequest struct {
	UUIDs []string `json:"uuids"`
}

// UserIDsRequest is a request body carrying a list of numeric user ids.
type UserIDsRequest struct {
	UserIDs []int64 `json:"userIds"`
}

// BulkUpdateFields carries the mutable user attributes of a bulk update.
// Remnawave 3 nests them under "fields" instead of spreading them next to the
// id list.
type BulkUpdateFields struct {
	Status            *string    `json:"status,omitempty"`
	TrafficLimitBytes *float64   `json:"trafficLimitBytes,omitempty"`
	ExpireAt          *time.Time `json:"expireAt,omitempty"`
}

// BulkUpdateUsersRequest is the payload for bulk-updating selected users.
type BulkUpdateUsersRequest struct {
	UserIDs []int64          `json:"userIds"`
	Fields  BulkUpdateFields `json:"fields"`
}

// BulkUpdateSquadsRequest is the payload for bulk-updating squad assignments.
type BulkUpdateSquadsRequest struct {
	UserIDs              []int64  `json:"userIds"`
	ActiveInternalSquads []string `json:"activeInternalSquads"`
}

// BulkExtendExpirationRequest is the payload for extending expiration dates.
// UserIDs is optional: when set, only those users are affected; the /all/
// variant of the endpoint ignores it and extends everyone.
type BulkExtendExpirationRequest struct {
	UserIDs    []int64 `json:"userIds,omitempty"`
	ExtendDays int     `json:"extendDays"`
}

// BulkUpdateAllRequest is the payload for updating all users at once.
type BulkUpdateAllRequest struct {
	Status            *string    `json:"status,omitempty"`
	TrafficLimitBytes *float64   `json:"trafficLimitBytes,omitempty"`
	ExpireAt          *time.Time `json:"expireAt,omitempty"`
}

// --- Users list types ---

// GetAllUsersParams holds query parameters for paginated user listing.
type GetAllUsersParams struct {
	Start  int    `json:"start"`
	Size   int    `json:"size"`
	Status string `json:"status,omitempty"`
}

// UsersListResponse is the response from the paginated users list endpoint.
type UsersListResponse struct {
	Users []RemnawaveUserWithTraffic `json:"users"`
	Total int                        `json:"total"`
}

// --- Node types ---

// NodeConfigProfile is the 2.8.0 config-profile reference on a node. It replaced
// the removed excludedInbounds/externalRawConfig keys; ActiveInbounds lists the
// config-profile inbound UUIDs the node serves.
type NodeConfigProfile struct {
	ActiveConfigProfileUuid string   `json:"activeConfigProfileUuid"`
	ActiveInbounds          []string `json:"activeInbounds"`
}

// CreateNodeRequest is the payload for creating a new proxy node.
type CreateNodeRequest struct {
	Name                  string             `json:"name"`
	Address               string             `json:"address"`
	Port                  int                `json:"port"`
	IsTrafficTrackable    bool               `json:"isTrafficTrackingActive"`
	TrafficLimitBytes     int64              `json:"trafficLimitBytes,omitempty"`
	NotifyPercent         int                `json:"notifyPercent,omitempty"`
	TrafficResetDay       int                `json:"trafficResetDay,omitempty"`
	CountryCode           string             `json:"countryCode,omitempty"`
	ConsumptionMultiplier float64            `json:"consumptionMultiplier,omitempty"`
	ConfigProfile         *NodeConfigProfile `json:"configProfile,omitempty"`
}

// UpdateNodeRequest is the payload for modifying an existing node.
type UpdateNodeRequest struct {
	UUID                  string             `json:"uuid"`
	Name                  string             `json:"name,omitempty"`
	Address               string             `json:"address,omitempty"`
	Port                  *int               `json:"port,omitempty"`
	IsTrafficTrackable    *bool              `json:"isTrafficTrackingActive,omitempty"`
	TrafficLimitBytes     *int64             `json:"trafficLimitBytes,omitempty"`
	NotifyPercent         *int               `json:"notifyPercent,omitempty"`
	TrafficResetDay       *int               `json:"trafficResetDay,omitempty"`
	CountryCode           string             `json:"countryCode,omitempty"`
	ConsumptionMultiplier *float64           `json:"consumptionMultiplier,omitempty"`
	ConfigProfile         *NodeConfigProfile `json:"configProfile,omitempty"`
}

// ReorderRequest is a request body for reordering items by their UUIDs.
type ReorderRequest struct {
	UUIDs []string `json:"uuids"`
}

// --- Host types ---

// HostInbound is the nested inbound reference on a host. Remnawave 2.8.0
// replaced the flat top-level inboundUuid with this object; it points at a
// config-profile inbound. Pointer fields distinguish absent from empty.
type HostInbound struct {
	ConfigProfileUuid        *string `json:"configProfileUuid"`
	ConfigProfileInboundUuid *string `json:"configProfileInboundUuid"`
}

// RemnawaveHost represents a host (inbound connection endpoint) in Remnawave.
type RemnawaveHost struct {
	UUID          string      `json:"uuid"`
	Remark        string      `json:"remark"`
	Address       string      `json:"address"`
	Port          int         `json:"port"`
	Inbound       HostInbound `json:"inbound"`
	IsDisabled    bool        `json:"isDisabled"`
	SecurityLayer string      `json:"securityLayer,omitempty"`
	Fingerprint   string      `json:"fingerprint,omitempty"`
	ALPN          string      `json:"alpn,omitempty"`
	SNI           string      `json:"sni,omitempty"`
	Path          string      `json:"path,omitempty"`
	Host          string      `json:"host,omitempty"`
	Tags          []string    `json:"tags,omitempty"`
}

// CreateHostRequest is the payload for creating a new host. The nested inbound
// object is REQUIRED by the 2.8.0 contract (no omitempty).
type CreateHostRequest struct {
	Remark        string      `json:"remark"`
	Address       string      `json:"address"`
	Port          int         `json:"port"`
	Inbound       HostInbound `json:"inbound"`
	IsDisabled    bool        `json:"isDisabled,omitempty"`
	SecurityLayer string      `json:"securityLayer,omitempty"`
	Fingerprint   string      `json:"fingerprint,omitempty"`
	ALPN          string      `json:"alpn,omitempty"`
	SNI           string      `json:"sni,omitempty"`
	Path          string      `json:"path,omitempty"`
	Host          string      `json:"host,omitempty"`
	Tags          []string    `json:"tags,omitempty"`
}

// UpdateHostRequest is the payload for modifying an existing host. Inbound is a
// pointer so a partial update can omit it.
type UpdateHostRequest struct {
	UUID          string       `json:"uuid"`
	Remark        string       `json:"remark,omitempty"`
	Address       string       `json:"address,omitempty"`
	Port          *int         `json:"port,omitempty"`
	Inbound       *HostInbound `json:"inbound,omitempty"`
	IsDisabled    *bool        `json:"isDisabled,omitempty"`
	SecurityLayer string       `json:"securityLayer,omitempty"`
	Fingerprint   string       `json:"fingerprint,omitempty"`
	ALPN          string       `json:"alpn,omitempty"`
	SNI           string       `json:"sni,omitempty"`
	Path          string       `json:"path,omitempty"`
	Host          string       `json:"host,omitempty"`
	Tags          []string     `json:"tags,omitempty"`
}
