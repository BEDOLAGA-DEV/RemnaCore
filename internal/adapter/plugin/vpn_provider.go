// Package plugin implements adapter-layer components that bridge the domain
// ports to the WASM plugin system. The pluginVPNProvider dispatches hooks to
// plugins for request building and response parsing, while keeping HTTP
// transport on the host side with circuit breaker protection.
package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
)

// VPN provider hook name constants.
const (
	HookVPNUserCreating  = "vpn.user.creating"
	HookVPNUserCreated   = "vpn.user.created"
	HookVPNUserFetching  = "vpn.user.fetching"
	HookVPNUserFetched   = "vpn.user.fetched"
	HookVPNUserDeleting  = "vpn.user.deleting"
	HookVPNUserEnabling  = "vpn.user.enabling"
	HookVPNUserDisabling = "vpn.user.disabling"
)

// shortUUIDLength is the number of characters used for the short UUID prefix.
const shortUUIDLength = 8

// pluginVPNProvider implements multisub.VPNProvider by dispatching VPN hooks
// to WASM plugins. The plugin builds request specs; the host executes HTTP
// with circuit breaker and retry.
type pluginVPNProvider struct {
	dispatcher            hookdispatch.Dispatcher
	httpClient            sdk.VPNHTTPExecutor
	logger                *slog.Logger
	defaultInternalSquads []string
}

// NewPluginVPNProvider creates a plugin-backed VPN provider.
func NewPluginVPNProvider(
	dispatcher hookdispatch.Dispatcher,
	httpClient sdk.VPNHTTPExecutor,
	logger *slog.Logger,
	defaultInternalSquads []string,
) multisub.VPNProvider {
	return &pluginVPNProvider{
		dispatcher:            dispatcher,
		httpClient:            httpClient,
		logger:                logger,
		defaultInternalSquads: defaultInternalSquads,
	}
}

// resolveInternalSquads returns the request's squads when non-empty, else the
// configured default.
func resolveInternalSquads(reqSquads, defaultSquads []string) []string {
	if len(reqSquads) > 0 {
		return reqSquads
	}
	return defaultSquads
}

// CreateUser provisions a VPN user through the plugin pipeline:
//  1. Dispatch vpn.user.creating -> plugin returns a VPNRequest spec
//  2. Host executes HTTP with circuit breaker
//  3. Dispatch vpn.user.fetched -> plugin parses the raw response
//  4. Dispatch vpn.user.created async notification
func (p *pluginVPNProvider) CreateUser(ctx context.Context, req multisub.CreateVPNUserRequest) (*multisub.VPNUserResult, error) {
	// 1. Dispatch vpn.user.creating -> plugin returns VPNRequest spec
	hookReq := sdk.VPNUserCreatingRequest{
		Username:             req.Username,
		TrafficLimitBytes:    req.TrafficLimitBytes,
		TrafficStrategy:      req.TrafficStrategy,
		Tag:                  req.Tag,
		BindingPurpose:       req.BindingPurpose,
		ActiveInternalSquads: resolveInternalSquads(req.ActiveInternalSquads, p.defaultInternalSquads),
	}
	if req.ExpireAt != nil {
		hookReq.ExpireAt = req.ExpireAt.Format(time.RFC3339)
	}

	payload, err := json.Marshal(hookReq)
	if err != nil {
		return nil, fmt.Errorf("marshal creating request: %w", err)
	}

	resp, err := p.dispatcher.DispatchSync(ctx, HookVPNUserCreating, payload)
	if err != nil {
		return nil, fmt.Errorf("dispatch %s: %w", HookVPNUserCreating, err)
	}

	var createResp sdk.VPNUserCreatingResponse
	if err := json.Unmarshal(resp, &createResp); err != nil {
		return nil, fmt.Errorf("unmarshal creating response: %w", err)
	}

	// 2. Host executes HTTP with circuit breaker
	httpResp, err := p.httpClient.Execute(ctx, createResp.CreateRequest)
	if err != nil {
		return nil, fmt.Errorf("execute create request: %w", err)
	}

	// 3. Dispatch vpn.user.fetched to parse response
	result, err := p.parseUserResponse(ctx, "", httpResp)
	if err != nil {
		return nil, fmt.Errorf("parse create response: %w", err)
	}

	shortUUID := result.UUID
	if len(shortUUID) > shortUUIDLength {
		shortUUID = shortUUID[:shortUUIDLength]
	}

	vpnResult := &multisub.VPNUserResult{
		UUID:      result.UUID,
		ShortUUID: shortUUID,
	}

	// 4. Async notification (fire-and-forget)
	notification := sdk.VPNUserCreatedNotification{
		VPNUUID:   vpnResult.UUID,
		ShortUUID: vpnResult.ShortUUID,
		Username:  req.Username,
	}
	if notifData, err := json.Marshal(notification); err == nil {
		p.dispatcher.DispatchAsync(ctx, HookVPNUserCreated, notifData)
	} else {
		p.logger.Warn("failed to marshal vpn.user.created notification",
			slog.Any("error", err),
		)
	}

	return vpnResult, nil
}

// GetUser retrieves the current status of a VPN user through the plugin pipeline:
//  1. Dispatch vpn.user.fetching -> plugin returns a VPNRequest spec
//  2. Host executes HTTP with circuit breaker
//  3. Dispatch vpn.user.fetched -> plugin parses the raw response
func (p *pluginVPNProvider) GetUser(ctx context.Context, vpnUUID string) (*multisub.VPNUserStatus, error) {
	// 1. Dispatch vpn.user.fetching -> get VPNRequest
	hookReq := sdk.VPNUserFetchingRequest{VPNUUID: vpnUUID}
	payload, err := json.Marshal(hookReq)
	if err != nil {
		return nil, fmt.Errorf("marshal fetching request: %w", err)
	}

	resp, err := p.dispatcher.DispatchSync(ctx, HookVPNUserFetching, payload)
	if err != nil {
		return nil, fmt.Errorf("dispatch %s: %w", HookVPNUserFetching, err)
	}

	var fetchResp sdk.VPNUserFetchingResponse
	if err := json.Unmarshal(resp, &fetchResp); err != nil {
		return nil, fmt.Errorf("unmarshal fetching response: %w", err)
	}

	// 2. Host executes HTTP
	httpResp, err := p.httpClient.Execute(ctx, fetchResp.GetRequest)
	if err != nil {
		return nil, fmt.Errorf("execute get request: %w", err)
	}

	// 3. Parse response via plugin
	parsed, err := p.parseUserResponse(ctx, vpnUUID, httpResp)
	if err != nil {
		return nil, fmt.Errorf("parse get response: %w", err)
	}

	return &multisub.VPNUserStatus{
		UUID:      parsed.UUID,
		Enabled:   parsed.Enabled,
		Expired:   parsed.Expired,
		UsedBytes: parsed.UsedBytes,
	}, nil
}

// DeleteUser removes a VPN user through the plugin pipeline:
//  1. Dispatch vpn.user.deleting -> plugin returns a VPNRequest spec
//  2. Host executes HTTP with circuit breaker
func (p *pluginVPNProvider) DeleteUser(ctx context.Context, vpnUUID string) error {
	return p.executeStateChange(ctx, HookVPNUserDeleting, vpnUUID, func(payload []byte) (sdk.VPNRequest, error) {
		hookReq := sdk.VPNUserDeletingRequest{VPNUUID: vpnUUID}
		data, err := json.Marshal(hookReq)
		if err != nil {
			return sdk.VPNRequest{}, fmt.Errorf("marshal deleting request: %w", err)
		}

		resp, err := p.dispatcher.DispatchSync(ctx, HookVPNUserDeleting, data)
		if err != nil {
			return sdk.VPNRequest{}, fmt.Errorf("dispatch %s: %w", HookVPNUserDeleting, err)
		}

		var deleteResp sdk.VPNUserDeletingResponse
		if err := json.Unmarshal(resp, &deleteResp); err != nil {
			return sdk.VPNRequest{}, fmt.Errorf("unmarshal deleting response: %w", err)
		}

		return deleteResp.DeleteRequest, nil
	})
}

// EnableUser activates a VPN user through the plugin pipeline:
//  1. Dispatch vpn.user.enabling -> plugin returns a VPNRequest spec
//  2. Host executes HTTP with circuit breaker
func (p *pluginVPNProvider) EnableUser(ctx context.Context, vpnUUID string) error {
	return p.executeStateChange(ctx, HookVPNUserEnabling, vpnUUID, func(_ []byte) (sdk.VPNRequest, error) {
		hookReq := sdk.VPNUserEnablingRequest{VPNUUID: vpnUUID}
		data, err := json.Marshal(hookReq)
		if err != nil {
			return sdk.VPNRequest{}, fmt.Errorf("marshal enabling request: %w", err)
		}

		resp, err := p.dispatcher.DispatchSync(ctx, HookVPNUserEnabling, data)
		if err != nil {
			return sdk.VPNRequest{}, fmt.Errorf("dispatch %s: %w", HookVPNUserEnabling, err)
		}

		var enableResp sdk.VPNUserEnablingResponse
		if err := json.Unmarshal(resp, &enableResp); err != nil {
			return sdk.VPNRequest{}, fmt.Errorf("unmarshal enabling response: %w", err)
		}

		return enableResp.EnableRequest, nil
	})
}

// DisableUser deactivates a VPN user through the plugin pipeline:
//  1. Dispatch vpn.user.disabling -> plugin returns a VPNRequest spec
//  2. Host executes HTTP with circuit breaker
func (p *pluginVPNProvider) DisableUser(ctx context.Context, vpnUUID string) error {
	return p.executeStateChange(ctx, HookVPNUserDisabling, vpnUUID, func(_ []byte) (sdk.VPNRequest, error) {
		hookReq := sdk.VPNUserDisablingRequest{VPNUUID: vpnUUID}
		data, err := json.Marshal(hookReq)
		if err != nil {
			return sdk.VPNRequest{}, fmt.Errorf("marshal disabling request: %w", err)
		}

		resp, err := p.dispatcher.DispatchSync(ctx, HookVPNUserDisabling, data)
		if err != nil {
			return sdk.VPNRequest{}, fmt.Errorf("dispatch %s: %w", HookVPNUserDisabling, err)
		}

		var disableResp sdk.VPNUserDisablingResponse
		if err := json.Unmarshal(resp, &disableResp); err != nil {
			return sdk.VPNRequest{}, fmt.Errorf("unmarshal disabling response: %w", err)
		}

		return disableResp.DisableRequest, nil
	})
}

// parseUserResponse dispatches vpn.user.fetched to have the plugin parse a raw
// HTTP response into structured VPN user data.
func (p *pluginVPNProvider) parseUserResponse(ctx context.Context, vpnUUID string, httpResp *sdk.VPNResponse) (*sdk.VPNUserFetchedResponse, error) {
	fetchedReq := sdk.VPNUserFetchedRequest{
		VPNUUID:     vpnUUID,
		RawResponse: *httpResp,
	}
	fetchPayload, err := json.Marshal(fetchedReq)
	if err != nil {
		return nil, fmt.Errorf("marshal fetched request: %w", err)
	}

	parsedResp, err := p.dispatcher.DispatchSync(ctx, HookVPNUserFetched, fetchPayload)
	if err != nil {
		return nil, fmt.Errorf("dispatch %s: %w", HookVPNUserFetched, err)
	}

	var parsed sdk.VPNUserFetchedResponse
	if err := json.Unmarshal(parsedResp, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal fetched response: %w", err)
	}

	return &parsed, nil
}

// executeStateChange is a shared helper for delete/enable/disable operations
// that follow the same pattern: dispatch hook -> get VPNRequest -> execute HTTP.
// The buildRequest callback is responsible for dispatching the specific hook
// and returning the VPNRequest.
func (p *pluginVPNProvider) executeStateChange(
	ctx context.Context,
	hookName string,
	vpnUUID string,
	buildRequest func(payload []byte) (sdk.VPNRequest, error),
) error {
	vpnReq, err := buildRequest(nil)
	if err != nil {
		return err
	}

	if _, err := p.httpClient.Execute(ctx, vpnReq); err != nil {
		return fmt.Errorf("execute %s request for %s: %w", hookName, vpnUUID, err)
	}

	return nil
}

// compile-time interface check
var _ multisub.VPNProvider = (*pluginVPNProvider)(nil)
