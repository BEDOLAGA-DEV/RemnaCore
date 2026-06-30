package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeHookInput(t *testing.T, hookName string, payload any) []byte {
	t.Helper()
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	input, err := json.Marshal(HookContext{
		HookName:  hookName,
		RequestID: "req-test-123",
		PluginID:  "remnawave-provider",
		Payload:   payloadBytes,
	})
	require.NoError(t, err)
	return input
}

func TestHandleUserCreating(t *testing.T) {
	tests := []struct {
		name         string
		payload      VPNUserCreatingRequest
		wantMethod   string
		wantPath     string
		wantUsername string
		wantTraffic  float64
	}{
		{
			name: "standard user creation",
			payload: VPNUserCreatingRequest{
				Username:          "p_abc12345_base_0",
				TrafficLimitBytes: 10737418240,
				TrafficStrategy:   "no_reset",
				ExpireAt:          "2026-05-01T00:00:00Z",
				Tag:               "PLATFORM",
				BindingPurpose:    "base",
			},
			wantMethod:   http.MethodPost,
			wantPath:     apiPathUsers,
			wantUsername: "p_abc12345_base_0",
			wantTraffic:  10737418240,
		},
		{
			name: "gaming binding with zero traffic",
			payload: VPNUserCreatingRequest{
				Username:          "p_def67890_gaming_1",
				TrafficLimitBytes: 0,
				TrafficStrategy:   "no_reset",
				Tag:               "PLATFORM",
				BindingPurpose:    "gaming",
			},
			wantMethod:   http.MethodPost,
			wantPath:     apiPathUsers,
			wantUsername: "p_def67890_gaming_1",
			wantTraffic:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := makeHookInput(t, "vpn.user.creating", tc.payload)
			output, err := handleUserCreating(input)
			require.NoError(t, err)

			var result HookResult
			require.NoError(t, json.Unmarshal(output, &result))
			assert.Equal(t, actionModify, result.Action)

			var modified VPNUserCreatingResponse
			require.NoError(t, json.Unmarshal(result.Modified, &modified))
			assert.Equal(t, tc.wantMethod, modified.CreateRequest.Method)
			assert.Equal(t, tc.wantPath, modified.CreateRequest.Path)
			assert.Equal(t, defaultTimeoutMS, modified.CreateRequest.Timeout)
			assert.Equal(t, contentTypeJSON, modified.CreateRequest.Headers[headerContentType])

			var body remnawaveCreateUserRequest
			require.NoError(t, json.Unmarshal(modified.CreateRequest.Body, &body))
			assert.Equal(t, tc.wantUsername, body.Username)
			assert.Equal(t, tc.wantTraffic, body.TrafficLimitBytes)
		})
	}
}

func TestHandleUserCreating_InvalidInput(t *testing.T) {
	output, err := handleUserCreating([]byte("not-json"))
	require.NoError(t, err)

	var result HookResult
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, actionHalt, result.Action)
	assert.Contains(t, result.Error, "invalid input")
}

func TestHandleUserCreating_InvalidPayload(t *testing.T) {
	input, _ := json.Marshal(HookContext{
		HookName: "vpn.user.creating",
		Payload:  json.RawMessage(`"not-an-object"`),
	})

	output, err := handleUserCreating(input)
	require.NoError(t, err)

	var result HookResult
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, actionHalt, result.Action)
	assert.Contains(t, result.Error, "invalid payload")
}

func TestHandleUserDeleting(t *testing.T) {
	tests := []struct {
		name       string
		payload    VPNUserDeletingRequest
		wantMethod string
		wantPath   string
	}{
		{
			name: "delete by uuid",
			payload: VPNUserDeletingRequest{
				VPNUUID:   "550e8400-e29b-41d4-a716-446655440000",
				BindingID: "bind-123",
			},
			wantMethod: http.MethodDelete,
			wantPath:   apiPathUsers + "550e8400-e29b-41d4-a716-446655440000",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := makeHookInput(t, "vpn.user.deleting", tc.payload)
			output, err := handleUserDeleting(input)
			require.NoError(t, err)

			var result HookResult
			require.NoError(t, json.Unmarshal(output, &result))
			assert.Equal(t, actionModify, result.Action)

			var modified VPNUserDeletingResponse
			require.NoError(t, json.Unmarshal(result.Modified, &modified))
			assert.Equal(t, tc.wantMethod, modified.DeleteRequest.Method)
			assert.Equal(t, tc.wantPath, modified.DeleteRequest.Path)
			assert.Nil(t, modified.DeleteRequest.Body)
		})
	}
}

func TestHandleUserDeleting_EmptyUUID(t *testing.T) {
	input := makeHookInput(t, "vpn.user.deleting", VPNUserDeletingRequest{VPNUUID: ""})
	output, err := handleUserDeleting(input)
	require.NoError(t, err)

	var result HookResult
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, actionHalt, result.Action)
	assert.Contains(t, result.Error, "vpn_uuid is required")
}

func TestHandleUserEnabling(t *testing.T) {
	tests := []struct {
		name       string
		vpnUUID    string
		wantMethod string
		wantPath   string
	}{
		{
			name:       "enable user",
			vpnUUID:    "uuid-enable-123",
			wantMethod: http.MethodPost,
			wantPath:   apiPathUsers + "uuid-enable-123" + apiPathActions + apiPathEnable,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := makeHookInput(t, "vpn.user.enabling", VPNUserEnablingRequest{VPNUUID: tc.vpnUUID})
			output, err := handleUserEnabling(input)
			require.NoError(t, err)

			var result HookResult
			require.NoError(t, json.Unmarshal(output, &result))
			assert.Equal(t, actionModify, result.Action)

			var modified VPNUserEnablingResponse
			require.NoError(t, json.Unmarshal(result.Modified, &modified))
			assert.Equal(t, tc.wantMethod, modified.EnableRequest.Method)
			assert.Equal(t, tc.wantPath, modified.EnableRequest.Path)
		})
	}
}

func TestHandleUserEnabling_EmptyUUID(t *testing.T) {
	input := makeHookInput(t, "vpn.user.enabling", VPNUserEnablingRequest{VPNUUID: ""})
	output, err := handleUserEnabling(input)
	require.NoError(t, err)

	var result HookResult
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, actionHalt, result.Action)
	assert.Contains(t, result.Error, "vpn_uuid is required")
}

func TestHandleUserDisabling(t *testing.T) {
	tests := []struct {
		name       string
		vpnUUID    string
		wantMethod string
		wantPath   string
	}{
		{
			name:       "disable user",
			vpnUUID:    "uuid-disable-456",
			wantMethod: http.MethodPost,
			wantPath:   apiPathUsers + "uuid-disable-456" + apiPathActions + apiPathDisable,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := makeHookInput(t, "vpn.user.disabling", VPNUserDisablingRequest{VPNUUID: tc.vpnUUID})
			output, err := handleUserDisabling(input)
			require.NoError(t, err)

			var result HookResult
			require.NoError(t, json.Unmarshal(output, &result))
			assert.Equal(t, actionModify, result.Action)

			var modified VPNUserDisablingResponse
			require.NoError(t, json.Unmarshal(result.Modified, &modified))
			assert.Equal(t, tc.wantMethod, modified.DisableRequest.Method)
			assert.Equal(t, tc.wantPath, modified.DisableRequest.Path)
		})
	}
}

func TestHandleUserDisabling_EmptyUUID(t *testing.T) {
	input := makeHookInput(t, "vpn.user.disabling", VPNUserDisablingRequest{VPNUUID: ""})
	output, err := handleUserDisabling(input)
	require.NoError(t, err)

	var result HookResult
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, actionHalt, result.Action)
	assert.Contains(t, result.Error, "vpn_uuid is required")
}

func TestHandleUserFetching(t *testing.T) {
	tests := []struct {
		name       string
		vpnUUID    string
		wantMethod string
		wantPath   string
	}{
		{
			name:       "fetch user",
			vpnUUID:    "uuid-fetch-789",
			wantMethod: http.MethodGet,
			wantPath:   apiPathUsers + "uuid-fetch-789",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := makeHookInput(t, "vpn.user.fetching", VPNUserFetchingRequest{VPNUUID: tc.vpnUUID})
			output, err := handleUserFetching(input)
			require.NoError(t, err)

			var result HookResult
			require.NoError(t, json.Unmarshal(output, &result))
			assert.Equal(t, actionModify, result.Action)

			var modified VPNUserFetchingResponse
			require.NoError(t, json.Unmarshal(result.Modified, &modified))
			assert.Equal(t, tc.wantMethod, modified.GetRequest.Method)
			assert.Equal(t, tc.wantPath, modified.GetRequest.Path)
		})
	}
}

func TestHandleUserFetching_EmptyUUID(t *testing.T) {
	input := makeHookInput(t, "vpn.user.fetching", VPNUserFetchingRequest{VPNUUID: ""})
	output, err := handleUserFetching(input)
	require.NoError(t, err)

	var result HookResult
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, actionHalt, result.Action)
	assert.Contains(t, result.Error, "vpn_uuid is required")
}

func TestHandleUserFetched(t *testing.T) {
	tests := []struct {
		name        string
		status      string
		wantEnabled bool
		wantExpired bool
	}{
		{
			name:        "active user",
			status:      remnawaveStatusActive,
			wantEnabled: true,
			wantExpired: false,
		},
		{
			name:        "disabled user",
			status:      remnawaveStatusDisabled,
			wantEnabled: false,
			wantExpired: false,
		},
		{
			name:        "expired user",
			status:      remnawaveStatusExpired,
			wantEnabled: false,
			wantExpired: true,
		},
		{
			name:        "limited user",
			status:      remnawaveStatusLimited,
			wantEnabled: false,
			wantExpired: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			userData, err := json.Marshal(remnawaveUserWithTraffic{
				UUID:        "uuid-fetched-001",
				Username:    "p_abc12345_base_0",
				Status:      tc.status,
				UserTraffic: remnawaveUserTraffic{UsedTrafficBytes: 1073741824},
			})
			require.NoError(t, err)

			apiResp, err := json.Marshal(remnawaveAPIResponse{
				Success: true,
				Data:    userData,
			})
			require.NoError(t, err)

			payload := VPNUserFetchedRequest{
				VPNUUID: "uuid-fetched-001",
				RawResponse: VPNResponse{
					StatusCode: http.StatusOK,
					Body:       apiResp,
				},
			}

			input := makeHookInput(t, "vpn.user.fetched", payload)
			output, err := handleUserFetched(input)
			require.NoError(t, err)

			var result HookResult
			require.NoError(t, json.Unmarshal(output, &result))
			assert.Equal(t, actionModify, result.Action)

			var modified VPNUserFetchedResponse
			require.NoError(t, json.Unmarshal(result.Modified, &modified))
			assert.Equal(t, "uuid-fetched-001", modified.UUID)
			assert.Equal(t, tc.wantEnabled, modified.Enabled)
			assert.Equal(t, tc.wantExpired, modified.Expired)
			assert.Equal(t, int64(1073741824), modified.UsedBytes)
		})
	}
}

func TestHandleUserFetched_APIFailure(t *testing.T) {
	apiResp, err := json.Marshal(remnawaveAPIResponse{
		Success: false,
		Message: "user not found",
	})
	require.NoError(t, err)

	payload := VPNUserFetchedRequest{
		VPNUUID: "uuid-missing",
		RawResponse: VPNResponse{
			StatusCode: http.StatusNotFound,
			Body:       apiResp,
		},
	}

	input := makeHookInput(t, "vpn.user.fetched", payload)
	output, err := handleUserFetched(input)
	require.NoError(t, err)

	var result HookResult
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, actionHalt, result.Action)
	assert.Contains(t, result.Error, "remnawave API returned failure")
	assert.Contains(t, result.Error, "user not found")
}

func TestHandleUserFetched_InvalidResponseBody(t *testing.T) {
	payload := VPNUserFetchedRequest{
		VPNUUID: "uuid-bad-body",
		RawResponse: VPNResponse{
			StatusCode: http.StatusOK,
			Body:       []byte("not-json"),
		},
	}

	input := makeHookInput(t, "vpn.user.fetched", payload)
	output, err := handleUserFetched(input)
	require.NoError(t, err)

	var result HookResult
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, actionHalt, result.Action)
	assert.Contains(t, result.Error, "invalid remnawave response body")
}

func TestHandleUserCreated(t *testing.T) {
	payload := VPNUserCreatedNotification{
		BindingID: "bind-001",
		VPNUUID:   "uuid-created-001",
		ShortUUID: "short-001",
		Username:  "p_abc12345_base_0",
	}

	input := makeHookInput(t, "vpn.user.created", payload)
	output, err := handleUserCreated(input)
	require.NoError(t, err)

	var result HookResult
	require.NoError(t, json.Unmarshal(output, &result))
	assert.Equal(t, actionContinue, result.Action)
}

func TestRemnawaveHeaders(t *testing.T) {
	headers := remnawaveHeaders()
	assert.Equal(t, contentTypeJSON, headers[headerContentType])
	assert.Equal(t, forwardedProtoHTTPS, headers[headerForwardedProto])
	assert.Equal(t, forwardedLoopbackIP, headers[headerForwardedFor])
}

func TestHandleUserEnabling_ActionsPath(t *testing.T) {
	payloadBytes, _ := json.Marshal(VPNUserEnablingRequest{VPNUUID: "u1"})
	input, _ := json.Marshal(HookContext{HookName: "vpn.user.enabling", Payload: payloadBytes})
	out, err := handleUserEnabling(input)
	require.NoError(t, err)
	var res HookResult
	require.NoError(t, json.Unmarshal(out, &res))
	var er VPNUserEnablingResponse
	require.NoError(t, json.Unmarshal(res.Modified, &er))
	require.Equal(t, "/api/users/u1/actions/enable", er.EnableRequest.Path)
}

func TestHandleUserFetched_NestedTrafficUppercaseStatus(t *testing.T) {
	userData, _ := json.Marshal(remnawaveUserWithTraffic{
		UUID:        "u1",
		Status:      "LIMITED",
		UserTraffic: remnawaveUserTraffic{UsedTrafficBytes: 2048},
	})
	apiResp, _ := json.Marshal(remnawaveAPIResponse{Success: true, Data: userData})
	payloadBytes, _ := json.Marshal(VPNUserFetchedRequest{VPNUUID: "u1", RawResponse: VPNResponse{StatusCode: 200, Body: apiResp}})
	input, _ := json.Marshal(HookContext{HookName: "vpn.user.fetched", Payload: payloadBytes})
	out, err := handleUserFetched(input)
	require.NoError(t, err)
	var res HookResult
	require.NoError(t, json.Unmarshal(out, &res))
	var fr VPNUserFetchedResponse
	require.NoError(t, json.Unmarshal(res.Modified, &fr))
	require.Equal(t, int64(2048), fr.UsedBytes)
	require.False(t, fr.Enabled)
	require.True(t, fr.Expired)
}
