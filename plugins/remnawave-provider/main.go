// Package main implements the Remnawave VPN provider plugin for RemnaCore.
//
// This plugin handles six sync hooks and one async hook:
//   - vpn.user.creating  -- builds a POST request to create a VPN user
//   - vpn.user.deleting  -- builds a DELETE request to remove a VPN user
//   - vpn.user.enabling  -- builds a POST request to enable a VPN user
//   - vpn.user.disabling -- builds a POST request to disable a VPN user
//   - vpn.user.fetching  -- builds a GET request to fetch a VPN user
//   - vpn.user.fetched   -- parses a Remnawave API response into canonical form
//   - vpn.user.created   -- async notification (no-op acknowledgement)
//
// When compiled to WASM (GOOS=wasip1 GOARCH=wasm), exported functions are
// called by the platform's hook dispatcher via the Extism PDK. In native mode
// the handlers are regular functions exercised by unit tests.
package main

import (
	"encoding/json"
	"net/http"
)

// --- SDK types (mirrored from pkg/sdk for standalone WASM compilation) ---

// HookContext is the JSON envelope passed as input to every plugin hook.
type HookContext struct {
	HookName  string          `json:"hook_name"`
	RequestID string          `json:"request_id"`
	Timestamp int64           `json:"timestamp"`
	PluginID  string          `json:"plugin_id"`
	Payload   json.RawMessage `json:"payload"`
}

// HookResult is the JSON envelope returned by a plugin hook.
type HookResult struct {
	Action   string          `json:"action"`
	Modified json.RawMessage `json:"modified,omitempty"`
	Error    string          `json:"error,omitempty"`
}

// VPNRequest represents a VPN provider API request built by a plugin.
// The plugin constructs the request spec; the host executes it.
type VPNRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    []byte            `json:"body,omitempty"`
	Timeout int               `json:"timeout_ms,omitempty"`
}

// VPNResponse represents a VPN provider API response returned to a plugin for parsing.
type VPNResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       []byte            `json:"body,omitempty"`
}

// --- VPN hook payload/response types (mirrored from pkg/sdk/vpn_hooks.go) ---

// VPNUserCreatingRequest is the payload for vpn.user.creating.
type VPNUserCreatingRequest struct {
	Username             string   `json:"username"`
	TrafficLimitBytes    int64    `json:"traffic_limit_bytes"`
	TrafficStrategy      string   `json:"traffic_strategy"`
	ExpireAt             string   `json:"expire_at,omitempty"`
	Tag                  string   `json:"tag"`
	BindingPurpose       string   `json:"binding_purpose"`
	ActiveInternalSquads []string `json:"active_internal_squads,omitempty"`
}

// VPNUserCreatingResponse is the response from vpn.user.creating.
type VPNUserCreatingResponse struct {
	CreateRequest VPNRequest `json:"create_request"`
}

// VPNUserDeletingRequest is the payload for vpn.user.deleting.
type VPNUserDeletingRequest struct {
	VPNUUID   string `json:"vpn_uuid"`
	BindingID string `json:"binding_id"`
}

// VPNUserDeletingResponse is the response from vpn.user.deleting.
type VPNUserDeletingResponse struct {
	DeleteRequest VPNRequest `json:"delete_request"`
}

// VPNUserEnablingRequest is the payload for vpn.user.enabling.
type VPNUserEnablingRequest struct {
	VPNUUID string `json:"vpn_uuid"`
}

// VPNUserEnablingResponse is the response from vpn.user.enabling.
type VPNUserEnablingResponse struct {
	EnableRequest VPNRequest `json:"enable_request"`
}

// VPNUserDisablingRequest is the payload for vpn.user.disabling.
type VPNUserDisablingRequest struct {
	VPNUUID string `json:"vpn_uuid"`
}

// VPNUserDisablingResponse is the response from vpn.user.disabling.
type VPNUserDisablingResponse struct {
	DisableRequest VPNRequest `json:"disable_request"`
}

// VPNUserFetchingRequest is the payload for vpn.user.fetching.
type VPNUserFetchingRequest struct {
	VPNUUID string `json:"vpn_uuid"`
}

// VPNUserFetchingResponse is the response from vpn.user.fetching.
type VPNUserFetchingResponse struct {
	GetRequest VPNRequest `json:"get_request"`
}

// VPNUserFetchedRequest is the payload for vpn.user.fetched (response parsing).
type VPNUserFetchedRequest struct {
	VPNUUID     string      `json:"vpn_uuid"`
	RawResponse VPNResponse `json:"raw_response"`
}

// VPNUserFetchedResponse is the parsed result from vpn.user.fetched.
type VPNUserFetchedResponse struct {
	UUID      string `json:"uuid"`
	Enabled   bool   `json:"enabled"`
	Expired   bool   `json:"expired"`
	UsedBytes int64  `json:"used_bytes"`
}

// VPNUserCreatedNotification is the async payload for vpn.user.created.
type VPNUserCreatedNotification struct {
	BindingID string `json:"binding_id"`
	VPNUUID   string `json:"vpn_uuid"`
	ShortUUID string `json:"short_uuid"`
	Username  string `json:"username"`
}

// --- Remnawave API types (mirrored from adapter/remnawave/types.go) ---

// remnawaveCreateUserRequest is the JSON body sent to POST /api/users/.
type remnawaveCreateUserRequest struct {
	Username             string   `json:"username"`
	TrafficLimitBytes    float64  `json:"trafficLimitBytes"`
	ExpireAt             string   `json:"expireAt,omitempty"`
	ActiveInternalSquads []string `json:"activeInternalSquads,omitempty"`
}

// KNOWN ISSUE (2.8.0 users sub-project): this envelope ({success,message,data})
// does NOT match Remnawave's real REST envelope, which is {response: <user>}
// (see internal/adapter/remnawave APIResponse[T]). The plugin VPN-provider path
// is feature-flag-off (HooksVPNProviderEnabled); before enabling it, align this
// to the {response} envelope and verify against a live 2.8.0 panel.
type remnawaveAPIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message,omitempty"`
	Data    json.RawMessage `json:"data"`
}

// remnawaveUserTraffic is the nested traffic object in the Remnawave user response.
type remnawaveUserTraffic struct {
	UsedTrafficBytes float64 `json:"usedTrafficBytes"`
}

// remnawaveUserWithTraffic is the user object returned by GET /api/users/{uuid}.
type remnawaveUserWithTraffic struct {
	UUID        string               `json:"uuid"`
	Username    string               `json:"username"`
	Status      string               `json:"status"`
	UserTraffic remnawaveUserTraffic `json:"userTraffic"`
}

// --- Constants ---

const (
	providerName   = "remnawave"
	actionContinue = "continue"
	actionModify   = "modify"
	actionHalt     = "halt"
)

// Remnawave API path constants (matching adapter/remnawave/client.go).
const (
	apiPathUsers   = "/api/users/"
	apiPathActions = "/actions/"
	apiPathEnable  = "enable"
	apiPathDisable = "disable"
)

// HTTP header constants for Remnawave API requests.
const (
	headerContentType    = "Content-Type"
	headerAuthorization  = "Authorization"
	headerForwardedProto = "X-Forwarded-Proto"
	headerForwardedFor   = "X-Forwarded-For"
	contentTypeJSON      = "application/json"
	bearerPrefix         = "Bearer "
	forwardedProtoHTTPS  = "https"
	forwardedLoopbackIP  = "127.0.0.1"
)

// Remnawave user status constants.
const (
	remnawaveStatusActive   = "ACTIVE"
	remnawaveStatusDisabled = "DISABLED"
	remnawaveStatusExpired  = "EXPIRED"
	remnawaveStatusLimited  = "LIMITED"
)

// defaultTimeoutMS is the default timeout for Remnawave API requests.
const defaultTimeoutMS = 10000

// --- Hook handlers ---

// handleUserCreating builds a POST request to create a VPN user in Remnawave.
func handleUserCreating(input []byte) ([]byte, error) {
	var ctx HookContext
	if err := json.Unmarshal(input, &ctx); err != nil {
		return haltResult("invalid input: " + err.Error())
	}

	var payload VPNUserCreatingRequest
	if err := json.Unmarshal(ctx.Payload, &payload); err != nil {
		return haltResult("invalid payload: " + err.Error())
	}

	reqBody := remnawaveCreateUserRequest{
		Username:             payload.Username,
		TrafficLimitBytes:    float64(payload.TrafficLimitBytes),
		ExpireAt:             payload.ExpireAt,
		ActiveInternalSquads: payload.ActiveInternalSquads,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return haltResult("marshal request body: " + err.Error())
	}

	result := VPNUserCreatingResponse{
		CreateRequest: VPNRequest{
			Method:  http.MethodPost,
			Path:    apiPathUsers,
			Headers: remnawaveHeaders(),
			Body:    body,
			Timeout: defaultTimeoutMS,
		},
	}

	return modifyResult(result)
}

// handleUserDeleting builds a DELETE request to remove a VPN user from Remnawave.
func handleUserDeleting(input []byte) ([]byte, error) {
	var ctx HookContext
	if err := json.Unmarshal(input, &ctx); err != nil {
		return haltResult("invalid input: " + err.Error())
	}

	var payload VPNUserDeletingRequest
	if err := json.Unmarshal(ctx.Payload, &payload); err != nil {
		return haltResult("invalid payload: " + err.Error())
	}

	if payload.VPNUUID == "" {
		return haltResult("vpn_uuid is required")
	}

	result := VPNUserDeletingResponse{
		DeleteRequest: VPNRequest{
			Method:  http.MethodDelete,
			Path:    apiPathUsers + payload.VPNUUID,
			Headers: remnawaveHeaders(),
			Timeout: defaultTimeoutMS,
		},
	}

	return modifyResult(result)
}

// handleUserEnabling builds a POST request to enable a VPN user in Remnawave.
func handleUserEnabling(input []byte) ([]byte, error) {
	var ctx HookContext
	if err := json.Unmarshal(input, &ctx); err != nil {
		return haltResult("invalid input: " + err.Error())
	}

	var payload VPNUserEnablingRequest
	if err := json.Unmarshal(ctx.Payload, &payload); err != nil {
		return haltResult("invalid payload: " + err.Error())
	}

	if payload.VPNUUID == "" {
		return haltResult("vpn_uuid is required")
	}

	result := VPNUserEnablingResponse{
		EnableRequest: VPNRequest{
			Method:  http.MethodPost,
			Path:    apiPathUsers + payload.VPNUUID + apiPathActions + apiPathEnable,
			Headers: remnawaveHeaders(),
			Timeout: defaultTimeoutMS,
		},
	}

	return modifyResult(result)
}

// handleUserDisabling builds a POST request to disable a VPN user in Remnawave.
func handleUserDisabling(input []byte) ([]byte, error) {
	var ctx HookContext
	if err := json.Unmarshal(input, &ctx); err != nil {
		return haltResult("invalid input: " + err.Error())
	}

	var payload VPNUserDisablingRequest
	if err := json.Unmarshal(ctx.Payload, &payload); err != nil {
		return haltResult("invalid payload: " + err.Error())
	}

	if payload.VPNUUID == "" {
		return haltResult("vpn_uuid is required")
	}

	result := VPNUserDisablingResponse{
		DisableRequest: VPNRequest{
			Method:  http.MethodPost,
			Path:    apiPathUsers + payload.VPNUUID + apiPathActions + apiPathDisable,
			Headers: remnawaveHeaders(),
			Timeout: defaultTimeoutMS,
		},
	}

	return modifyResult(result)
}

// handleUserFetching builds a GET request to fetch a VPN user from Remnawave.
func handleUserFetching(input []byte) ([]byte, error) {
	var ctx HookContext
	if err := json.Unmarshal(input, &ctx); err != nil {
		return haltResult("invalid input: " + err.Error())
	}

	var payload VPNUserFetchingRequest
	if err := json.Unmarshal(ctx.Payload, &payload); err != nil {
		return haltResult("invalid payload: " + err.Error())
	}

	if payload.VPNUUID == "" {
		return haltResult("vpn_uuid is required")
	}

	result := VPNUserFetchingResponse{
		GetRequest: VPNRequest{
			Method:  http.MethodGet,
			Path:    apiPathUsers + payload.VPNUUID,
			Headers: remnawaveHeaders(),
			Timeout: defaultTimeoutMS,
		},
	}

	return modifyResult(result)
}

// handleUserFetched parses a Remnawave API response into canonical VPN user data.
func handleUserFetched(input []byte) ([]byte, error) {
	var ctx HookContext
	if err := json.Unmarshal(input, &ctx); err != nil {
		return haltResult("invalid input: " + err.Error())
	}

	var payload VPNUserFetchedRequest
	if err := json.Unmarshal(ctx.Payload, &payload); err != nil {
		return haltResult("invalid payload: " + err.Error())
	}

	var apiResp remnawaveAPIResponse
	if err := json.Unmarshal(payload.RawResponse.Body, &apiResp); err != nil {
		return haltResult("invalid remnawave response body: " + err.Error())
	}

	if !apiResp.Success {
		return haltResult("remnawave API returned failure: " + apiResp.Message)
	}

	var user remnawaveUserWithTraffic
	if err := json.Unmarshal(apiResp.Data, &user); err != nil {
		return haltResult("invalid remnawave user data: " + err.Error())
	}

	result := VPNUserFetchedResponse{
		UUID:      user.UUID,
		Enabled:   user.Status == remnawaveStatusActive,
		Expired:   user.Status == remnawaveStatusExpired || user.Status == remnawaveStatusLimited,
		UsedBytes: int64(user.UserTraffic.UsedTrafficBytes),
	}

	return modifyResult(result)
}

// handleUserCreated is the async notification handler for vpn.user.created.
// It acknowledges receipt; the host has already persisted the binding.
func handleUserCreated(input []byte) ([]byte, error) {
	return json.Marshal(HookResult{Action: actionContinue})
}

// --- Helpers ---

// remnawaveHeaders returns the standard HTTP headers required by the Remnawave API.
// The api_token is injected by the host when executing the VPNRequest,
// so we include placeholder headers for content type and proxy bypass only.
func remnawaveHeaders() map[string]string {
	return map[string]string{
		headerContentType:    contentTypeJSON,
		headerForwardedProto: forwardedProtoHTTPS,
		headerForwardedFor:   forwardedLoopbackIP,
	}
}

func modifyResult(data any) ([]byte, error) {
	modified, err := json.Marshal(data)
	if err != nil {
		return haltResult("marshal error: " + err.Error())
	}
	return json.Marshal(HookResult{
		Action:   actionModify,
		Modified: modified,
	})
}

func haltResult(errMsg string) ([]byte, error) {
	return json.Marshal(HookResult{
		Action: actionHalt,
		Error:  errMsg,
	})
}

func main() {}
