package sdk

// VPN provider hook request/response types used by VPN provider plugins
// to build API requests and parse responses.

// VPNUserCreatingRequest is the payload sent to vpn.user.creating hook.
type VPNUserCreatingRequest struct {
	Username             string   `json:"username"`
	TrafficLimitBytes    int64    `json:"traffic_limit_bytes"`
	TrafficStrategy      string   `json:"traffic_strategy"`
	ExpireAt             string   `json:"expire_at,omitempty"` // RFC3339
	Tag                  string   `json:"tag"`
	BindingPurpose       string   `json:"binding_purpose"`
	ActiveInternalSquads []string `json:"active_internal_squads,omitempty"`
}

// VPNUserCreatingResponse is the response from vpn.user.creating hook.
type VPNUserCreatingResponse struct {
	CreateRequest VPNRequest `json:"create_request"`
}

// VPNUserDeletingRequest is the payload sent to vpn.user.deleting hook.
type VPNUserDeletingRequest struct {
	VPNUUID   string `json:"vpn_uuid"`
	BindingID string `json:"binding_id"`
}

// VPNUserDeletingResponse is the response from vpn.user.deleting hook.
type VPNUserDeletingResponse struct {
	DeleteRequest VPNRequest `json:"delete_request"`
}

// VPNUserEnablingRequest is the payload sent to vpn.user.enabling hook.
type VPNUserEnablingRequest struct {
	VPNUUID string `json:"vpn_uuid"`
}

// VPNUserEnablingResponse is the response from vpn.user.enabling hook.
type VPNUserEnablingResponse struct {
	EnableRequest VPNRequest `json:"enable_request"`
}

// VPNUserDisablingRequest is the payload sent to vpn.user.disabling hook.
type VPNUserDisablingRequest struct {
	VPNUUID string `json:"vpn_uuid"`
}

// VPNUserDisablingResponse is the response from vpn.user.disabling hook.
type VPNUserDisablingResponse struct {
	DisableRequest VPNRequest `json:"disable_request"`
}

// VPNUserFetchingRequest is the payload sent to vpn.user.fetching hook.
type VPNUserFetchingRequest struct {
	VPNUUID string `json:"vpn_uuid"`
}

// VPNUserFetchingResponse is the response from vpn.user.fetching hook.
type VPNUserFetchingResponse struct {
	GetRequest VPNRequest `json:"get_request"`
}

// VPNUserFetchedRequest is the payload sent to vpn.user.fetched hook (response parsing).
type VPNUserFetchedRequest struct {
	VPNUUID     string      `json:"vpn_uuid"`
	RawResponse VPNResponse `json:"raw_response"`
}

// VPNUserFetchedResponse is the parsed result from vpn.user.fetched hook.
type VPNUserFetchedResponse struct {
	UUID      string `json:"uuid"`
	Enabled   bool   `json:"enabled"`
	Expired   bool   `json:"expired"`
	UsedBytes int64  `json:"used_bytes"`
}

// VPNUserCreatedNotification is the async payload for vpn.user.created hook.
type VPNUserCreatedNotification struct {
	BindingID string `json:"binding_id"`
	VPNUUID   string `json:"vpn_uuid"`
	ShortUUID string `json:"short_uuid"`
	Username  string `json:"username"`
}
