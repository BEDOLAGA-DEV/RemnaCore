package remnawave

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tracing"
)

const (
	// DefaultHTTPTimeout is the default timeout for HTTP requests to Remnawave.
	DefaultHTTPTimeout = 30 * time.Second

	// ForwardedProtoHTTPS is the value indicating HTTPS protocol.
	ForwardedProtoHTTPS = "https"

	// ForwardedLoopbackIP is the loopback IP used when RemnaCore calls
	// Remnawave directly (same host / Docker network).
	ForwardedLoopbackIP = "127.0.0.1"

	// MaxResponseBytes is the maximum allowed size for Remnawave API responses.
	// Prevents OOM if the upstream returns an unexpectedly large body.
	MaxResponseBytes = 10 << 20 // 10 MB

	// maxErrorBodyPreview is the maximum number of bytes from an error response
	// body included in error messages to prevent log bloat.
	maxErrorBodyPreview = 512
)

// Remnawave API path constants.
const (
	APIPathUsers   = "/api/users/"
	APIPathNodes   = "/api/nodes/"
	APIPathEnable  = "enable"
	APIPathDisable = "disable"
)

// Shared sub-path constants used across multiple resource clients.
const (
	subPathActions        = "/actions/"
	subPathActionsReorder = "actions/reorder"
)

// ErrNotConfigured is returned when no Remnawave panel has been registered
// yet. It is deliberately distinct from a transport error: the operator has to
// add a panel connection, not debug the network.
var ErrNotConfigured = errors.New("remnawave panel is not configured")

// isHTTPSuccess reports whether the given HTTP status code is in the 2xx range.
func isHTTPSuccess(statusCode int) bool {
	const (
		httpSuccessMin = 200
		httpSuccessMax = 300
	)
	return statusCode >= httpSuccessMin && statusCode < httpSuccessMax
}

// CredentialsFunc reports the Remnawave endpoint to talk to right now.
type CredentialsFunc func(context.Context) (baseURL, apiToken string, err error)

// Client communicates with the Remnawave REST API.
//
// Credentials are resolved per request rather than fixed at construction.
// Remnawave is configured through the admin panel, not the environment, so a
// client built during dependency wiring would hold an empty base URL for the
// life of the process — which surfaced as `unsupported protocol scheme ""`
// from the subscription proxy.
type Client struct {
	credentials CredentialsFunc
	httpClient  *http.Client
}

// NewClient returns a Client pinned to one Remnawave instance.
func NewClient(baseURL, apiToken string) *Client {
	return NewResolvingClient(func(context.Context) (string, string, error) {
		return baseURL, apiToken, nil
	})
}

// NewResolvingClient returns a Client that asks resolve for its endpoint on
// every call, so configuration changes take effect without a restart.
func NewResolvingClient(resolve CredentialsFunc) *Client {
	return &Client{
		credentials: resolve,
		httpClient: &http.Client{
			Timeout: DefaultHTTPTimeout,
		},
	}
}

// BaseURL returns the Remnawave base URL currently configured. The
// SubscriptionProxy uses it to build direct subscription fetch URLs.
func (c *Client) BaseURL(ctx context.Context) (string, error) {
	baseURL, _, err := c.credentials(ctx)
	if err != nil {
		return "", err
	}
	if baseURL == "" {
		return "", ErrNotConfigured
	}
	return baseURL, nil
}

// IsConfigured reports whether a panel endpoint is available. Until an
// administrator registers one there is nothing to call.
func (c *Client) IsConfigured(ctx context.Context) bool {
	baseURL, token, err := c.credentials(ctx)
	return err == nil && baseURL != "" && token != ""
}

// do executes an HTTP request against the Remnawave API and decodes the JSON
// response into dest (when dest is non-nil). It returns an error for non-2xx
// status codes that includes the response body for debugging.
func (c *Client) do(ctx context.Context, method, path string, body any, dest any) error {
	ctx, span := tracing.StartSpan(ctx, "remnawave."+method+"."+path)
	defer span.End()

	var reqBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(encoded)
	}

	baseURL, apiToken, err := c.credentials(ctx)
	if err != nil {
		return fmt.Errorf("resolve remnawave endpoint: %w", err)
	}
	if baseURL == "" {
		return ErrNotConfigured
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+apiToken)
	req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	// Remnawave v2.7+ requires reverse proxy headers to bypass ProxyCheckMiddleware
	req.Header.Set(httpconst.HeaderForwardedProto, ForwardedProtoHTTPS)
	req.Header.Set(httpconst.HeaderForwardedFor, ForwardedLoopbackIP)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	if int64(len(respBody)) > MaxResponseBytes {
		return fmt.Errorf("response body exceeds %d bytes", MaxResponseBytes)
	}

	if !isHTTPSuccess(resp.StatusCode) {
		detail := string(respBody)
		if len(detail) > maxErrorBodyPreview {
			detail = detail[:maxErrorBodyPreview] + "...(truncated)"
		}
		return fmt.Errorf("remnawave API error (status %d): %s", resp.StatusCode, detail)
	}

	if dest != nil {
		if err := json.Unmarshal(respBody, dest); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// doWithQuery behaves like do but appends query parameters to the request URL.
func (c *Client) doWithQuery(ctx context.Context, method, path string, body any, query map[string]string, dest any) error {
	if len(query) > 0 {
		params := url.Values{}
		for k, v := range query {
			params.Set(k, v)
		}
		path = path + "?" + params.Encode()
	}
	return c.do(ctx, method, path, body, dest)
}

// CreateUser provisions a new VPN user in Remnawave.
func (c *Client) CreateUser(ctx context.Context, req CreateUserRequest) (*RemnawaveUser, error) {
	var resp APIResponse[RemnawaveUser]
	if err := c.do(ctx, http.MethodPost, APIPathUsers, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// GetUserByUUID retrieves a single VPN user with traffic stats.
func (c *Client) GetUserByUUID(ctx context.Context, uuid string) (*RemnawaveUserWithTraffic, error) {
	var resp APIResponse[RemnawaveUserWithTraffic]
	if err := c.do(ctx, http.MethodGet, APIPathUsers+uuid, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// UpdateUser modifies an existing VPN user in Remnawave.
func (c *Client) UpdateUser(ctx context.Context, req UpdateUserRequest) (*RemnawaveUser, error) {
	var resp APIResponse[RemnawaveUser]
	if err := c.do(ctx, http.MethodPatch, APIPathUsers, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// DeleteUser removes a VPN user from Remnawave.
func (c *Client) DeleteUser(ctx context.Context, uuid string) error {
	return c.do(ctx, http.MethodDelete, APIPathUsers+uuid, nil, nil)
}

// EnableUser activates a VPN user in Remnawave.
func (c *Client) EnableUser(ctx context.Context, uuid string) error {
	path := fmt.Sprintf("%s%s%s%s", APIPathUsers, uuid, subPathActions, APIPathEnable)
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

// DisableUser deactivates a VPN user in Remnawave.
func (c *Client) DisableUser(ctx context.Context, uuid string) error {
	path := fmt.Sprintf("%s%s%s%s", APIPathUsers, uuid, subPathActions, APIPathDisable)
	return c.do(ctx, http.MethodPost, path, nil, nil)
}

// GetNodes returns all proxy nodes registered in Remnawave.
func (c *Client) GetNodes(ctx context.Context) ([]RemnawaveNode, error) {
	var resp APIResponse[[]RemnawaveNode]
	if err := c.do(ctx, http.MethodGet, APIPathNodes, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Response, nil
}
