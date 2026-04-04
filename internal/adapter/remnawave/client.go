package remnawave

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
)

// isHTTPSuccess reports whether the given HTTP status code is in the 2xx range.
func isHTTPSuccess(statusCode int) bool {
	const (
		httpSuccessMin = 200
		httpSuccessMax = 300
	)
	return statusCode >= httpSuccessMin && statusCode < httpSuccessMax
}

// Client communicates with the Remnawave REST API.
type Client struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

// NewClient returns a Client configured for the given Remnawave instance.
func NewClient(baseURL, apiToken string) *Client {
	return &Client{
		baseURL:  baseURL,
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: DefaultHTTPTimeout,
		},
	}
}

// BaseURL returns the configured Remnawave base URL. This is used by the
// SubscriptionProxy to build direct subscription fetch URLs.
func (c *Client) BaseURL() string {
	return c.baseURL
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

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+c.apiToken)
	req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	// Remnawave v2.7+ requires reverse proxy headers to bypass ProxyCheckMiddleware
	req.Header.Set(httpconst.HeaderForwardedProto, ForwardedProtoHTTPS)
	req.Header.Set(httpconst.HeaderForwardedFor, ForwardedLoopbackIP)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if !isHTTPSuccess(resp.StatusCode) {
		return fmt.Errorf("remnawave API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if dest != nil {
		if err := json.Unmarshal(respBody, dest); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// CreateUser provisions a new VPN user in Remnawave.
func (c *Client) CreateUser(ctx context.Context, req CreateUserRequest) (*RemnawaveUser, error) {
	var resp APIResponse[RemnawaveUser]
	if err := c.do(ctx, http.MethodPost, "/api/users/", req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetUserByUUID retrieves a single VPN user with traffic stats.
func (c *Client) GetUserByUUID(ctx context.Context, uuid string) (*RemnawaveUserWithTraffic, error) {
	var resp APIResponse[RemnawaveUserWithTraffic]
	if err := c.do(ctx, http.MethodGet, "/api/users/"+uuid, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// UpdateUser modifies an existing VPN user in Remnawave.
func (c *Client) UpdateUser(ctx context.Context, req UpdateUserRequest) (*RemnawaveUser, error) {
	var resp APIResponse[RemnawaveUser]
	if err := c.do(ctx, http.MethodPut, "/api/users/", req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// DeleteUser removes a VPN user from Remnawave.
func (c *Client) DeleteUser(ctx context.Context, uuid string) error {
	return c.do(ctx, http.MethodDelete, "/api/users/"+uuid, nil, nil)
}

// EnableUser activates a VPN user in Remnawave.
func (c *Client) EnableUser(ctx context.Context, uuid string) error {
	return c.do(ctx, http.MethodPost, "/api/users/"+uuid+"/enable", nil, nil)
}

// DisableUser deactivates a VPN user in Remnawave.
func (c *Client) DisableUser(ctx context.Context, uuid string) error {
	return c.do(ctx, http.MethodPost, "/api/users/"+uuid+"/disable", nil, nil)
}

// GetNodes returns all proxy nodes registered in Remnawave.
func (c *Client) GetNodes(ctx context.Context) ([]RemnawaveNode, error) {
	var resp APIResponse[[]RemnawaveNode]
	if err := c.do(ctx, http.MethodGet, "/api/nodes/", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}
