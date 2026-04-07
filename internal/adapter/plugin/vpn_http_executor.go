package plugin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/sony/gobreaker/v2"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/circuitbreaker"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
)

// VPN provider HTTP constants.
const (
	// VPNCBName is the circuit breaker instance name for VPN provider requests.
	VPNCBName = "vpn-provider"

	// DefaultVPNHTTPTimeout is the default timeout for VPN provider HTTP
	// requests when the plugin does not specify an override.
	DefaultVPNHTTPTimeout = 30 * time.Second

	// MaxVPNHTTPTimeout is the maximum allowed timeout for VPN provider HTTP
	// requests, capping any plugin-specified override.
	MaxVPNHTTPTimeout = 60 * time.Second

	// MaxVPNResponseBytes is the maximum allowed size for VPN provider API
	// responses. Prevents OOM if the upstream returns an unexpectedly large body.
	MaxVPNResponseBytes = 10 << 20 // 10 MB
)

// resilientVPNExecutor wraps HTTP execution with a circuit breaker for VPN
// provider API requests. It translates sdk.VPNRequest specs (built by plugins)
// into actual HTTP calls with circuit breaker protection. The authToken is
// injected into every outbound request as a Bearer token, keeping secrets on
// the host side rather than exposing them to WASM plugins.
type resilientVPNExecutor struct {
	client    *http.Client
	breaker   *gobreaker.CircuitBreaker[*sdk.VPNResponse]
	baseURL   string
	authToken string
	logger    *slog.Logger
}

// NewResilientVPNExecutor creates a circuit-breaker-wrapped HTTP executor for
// VPN provider requests. The baseURL is the VPN panel root URL; plugin-built
// VPNRequest.Path values are appended to it. The authToken is injected as a
// Bearer Authorization header on every outbound request. The cbCfg configures
// the circuit breaker; pass circuitbreaker.DefaultConfig() for defaults
// matching the previously hardcoded values.
func NewResilientVPNExecutor(baseURL string, authToken string, cbCfg circuitbreaker.Config, logger *slog.Logger) sdk.VPNHTTPExecutor {
	cb := circuitbreaker.NewBreaker[*sdk.VPNResponse](VPNCBName, cbCfg, func(name string, from, to gobreaker.State) {
		logger.Warn("vpn provider circuit breaker state changed",
			slog.String("name", name),
			slog.String("from", from.String()),
			slog.String("to", to.String()),
		)
	})

	return &resilientVPNExecutor{
		client: &http.Client{
			Timeout: DefaultVPNHTTPTimeout,
		},
		breaker:   cb,
		baseURL:   baseURL,
		authToken: authToken,
		logger:    logger,
	}
}

// Execute sends a VPN API request through the circuit breaker. The request
// spec is built by a plugin; this method handles the actual HTTP transport.
// Plugin-specified timeouts are honoured up to MaxVPNHTTPTimeout.
func (e *resilientVPNExecutor) Execute(ctx context.Context, req sdk.VPNRequest) (*sdk.VPNResponse, error) {
	return e.breaker.Execute(func() (*sdk.VPNResponse, error) {
		return e.doRequest(ctx, req)
	})
}

// doRequest performs the actual HTTP call for a VPN API request.
func (e *resilientVPNExecutor) doRequest(ctx context.Context, req sdk.VPNRequest) (*sdk.VPNResponse, error) {
	// Apply plugin-specified timeout, capped by MaxVPNHTTPTimeout.
	if req.Timeout > 0 {
		timeout := time.Duration(req.Timeout) * time.Millisecond
		if timeout > MaxVPNHTTPTimeout {
			timeout = MaxVPNHTTPTimeout
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	url := e.baseURL + req.Path

	var body io.Reader
	if len(req.Body) > 0 {
		body = bytes.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, url, body)
	if err != nil {
		return nil, fmt.Errorf("build vpn request: %w", err)
	}

	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// Inject host-side auth token, overriding any plugin-set Authorization
	// header to ensure the secret stays on the host.
	if e.authToken != "" {
		httpReq.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+e.authToken)
	}

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute vpn request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, MaxVPNResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read vpn response: %w", err)
	}
	if int64(len(respBody)) > MaxVPNResponseBytes {
		return nil, fmt.Errorf("vpn response body exceeds %d bytes", MaxVPNResponseBytes)
	}

	headers := make(map[string]string, len(resp.Header))
	for k := range resp.Header {
		headers[k] = resp.Header.Get(k)
	}

	return &sdk.VPNResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       respBody,
	}, nil
}

// compile-time interface check
var _ sdk.VPNHTTPExecutor = (*resilientVPNExecutor)(nil)
