package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
)

// MaxPluginResponseBodyBytes is the maximum size of an HTTP response body that
// a plugin is allowed to read. Responses larger than this are truncated.
const MaxPluginResponseBodyBytes = 5 << 20 // 5 MB

// DefaultPluginHTTPTimeout is the default timeout for outbound HTTP requests
// made by plugins.
const DefaultPluginHTTPTimeout = 10 * time.Second

// MaxVPNRequestTimeout is the maximum timeout allowed for VPN provider API
// requests made by plugins. Plugin-specified timeouts are capped at this value.
const MaxVPNRequestTimeout = 60 * time.Second

// Log level constants for plugin log entries.
const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

// HostFunctions defines the platform capabilities injected into WASM plugins
// via host function calls. Each method corresponds to a pdk_* function that
// plugins can invoke from inside the sandbox.
type HostFunctions struct {
	Storage     StorageService
	Publisher   domainevent.Publisher
	Logger      *slog.Logger
	Permissions *PermissionChecker
	HTTPClient  *http.Client
	Clock       clock.Clock
	// vpnExecutor handles VPN provider API requests for plugins. Optional;
	// nil when no VPN provider is configured.
	vpnExecutor sdk.VPNHTTPExecutor
	// pluginRegistry is used to look up a plugin instance for permission
	// checks. Populated during Fx wiring.
	pluginRegistry func(slug string) (*Plugin, error)
	// urlChecker validates whether a URL targets a blocked internal address.
	// Defaults to isBlockedHostname. Can be overridden in tests.
	urlChecker func(rawURL string) (blocked bool, err error)
	// httpLimiters holds per-plugin HTTP rate limiters keyed by plugin slug.
	httpLimiters sync.Map
}

// NewHostFunctions creates a HostFunctions value with sensible defaults. The
// HTTP client is configured with an SSRF-safe transport that blocks connections
// to private/internal network addresses after DNS resolution. Fields that depend
// on Fx-provided services (Storage, Publisher) can be nil and are expected to be
// set before the runtime starts.
func NewHostFunctions(logger *slog.Logger, clk clock.Clock) *HostFunctions {
	dialer := &net.Dialer{Timeout: SSRFSafeDialTimeout}
	transport := &http.Transport{
		DialContext: ssrfSafeDialContext(dialer),
	}
	return &HostFunctions{
		Logger:      logger,
		Permissions: &PermissionChecker{},
		HTTPClient: &http.Client{
			Timeout:   DefaultPluginHTTPTimeout,
			Transport: transport,
		},
		Clock:      clk,
		urlChecker: isBlockedHostname,
	}
}

// SetPluginRegistry allows the lifecycle manager to inject a lookup function
// that resolves a slug to a full Plugin (needed for permission checks).
func (hf *HostFunctions) SetPluginRegistry(fn func(slug string) (*Plugin, error)) {
	hf.pluginRegistry = fn
}

// SetVPNExecutor injects the VPN HTTP executor used by the VPNRequest host
// function. When nil, VPNRequest calls return an error. This is set during Fx
// wiring only when a VPN provider adapter is configured.
func (hf *HostFunctions) SetVPNExecutor(exec sdk.VPNHTTPExecutor) {
	hf.vpnExecutor = exec
}

// normalizeURL parses a raw URL, applies path.Clean to collapse path traversal
// sequences (e.g. "/../"), and returns the cleaned URL string. If the URL
// cannot be parsed, the original string is returned unchanged.
func normalizeURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL, nil
	}

	// Check for ".." as a path segment (not substring). Only reject when ".."
	// appears as an entire segment between "/" delimiters, which is what path
	// traversal actually requires. Legitimate paths like "/files/report..v2/data"
	// must not be blocked.
	for _, segment := range strings.Split(u.Path, "/") {
		if segment == ".." {
			return "", fmt.Errorf("%w: path traversal detected", ErrPermissionDenied)
		}
	}

	u.Path = path.Clean(u.Path)
	if u.Path == "." || u.Path == "" {
		u.Path = "/"
	}
	return u.String(), nil
}

// HTTPRequest performs an outbound HTTP request on behalf of a plugin after
// validating the target URL against the plugin's HTTP allowlist and blocking
// access to internal/private network addresses (SSRF protection).
//
// Security layers (applied in order):
//  0. URL path normalization and traversal rejection.
//  1. URL allowlist check from the plugin manifest.
//  2. Pre-flight hostname check for known loopback/local names.
//  3. Transport-level dial guard that rejects resolved private IPs (DNS rebinding).
func (hf *HostFunctions) HTTPRequest(ctx context.Context, pluginSlug string, req sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	p, err := hf.resolvePlugin(pluginSlug)
	if err != nil {
		return nil, err
	}

	// Layer 0: Normalize URL path and reject traversal sequences.
	normalizedURL, err := normalizeURL(req.URL)
	if err != nil {
		return nil, err
	}

	// Layer 1: URL allowlist (using normalized URL).
	if !hf.Permissions.ValidateHTTPRequest(p, normalizedURL) {
		return nil, fmt.Errorf("%w: HTTP request to %s not in allowlist for plugin %s",
			ErrPermissionDenied, normalizedURL, pluginSlug)
	}

	// Layer 2: pre-flight hostname check (fast, no DNS).
	blocked, err := hf.urlChecker(normalizedURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInternalNetworkAccess, err)
	}
	if blocked {
		return nil, fmt.Errorf("%w: plugin %s cannot access internal address in URL %s",
			ErrInternalNetworkAccess, pluginSlug, normalizedURL)
	}

	// Layer 3: DNS-rebinding protection is handled by the SSRF-safe transport
	// on hf.HTTPClient, which validates resolved IPs in DialContext.

	// Layer 4: per-plugin HTTP rate limiting from manifest limits.
	limits := p.Manifest.EffectiveLimits()
	if limits.MaxHTTPCallsPerMin > 0 {
		limiter := hf.getOrCreateHTTPLimiter(pluginSlug, limits.MaxHTTPCallsPerMin)
		if !limiter.Allow() {
			return nil, fmt.Errorf("%w: plugin %q exceeded %d HTTP calls/min",
				ErrHTTPRateLimitExceeded, pluginSlug, limits.MaxHTTPCallsPerMin)
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, normalizedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid HTTP request: %w", err)
	}

	if req.Body != nil {
		httpReq.Body = io.NopCloser(bytes.NewReader(req.Body))
		httpReq.ContentLength = int64(len(req.Body))
	}

	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := hf.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxPluginResponseBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to read HTTP response body: %w", err)
	}

	headers := make(map[string]string, len(resp.Header))
	for k := range resp.Header {
		headers[k] = resp.Header.Get(k)
	}

	return &sdk.HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       body,
	}, nil
}

// VPNRequest executes a VPN provider API request on behalf of a plugin. The
// plugin builds the request spec (method, relative path, headers, body); the
// host handles transport through its circuit breaker and connection pool.
// Requires the vpn:write permission scope.
//
// If the plugin specifies a timeout it is capped at MaxVPNRequestTimeout. When
// no VPN executor is configured (nil), the call returns an error.
func (hf *HostFunctions) VPNRequest(ctx context.Context, pluginSlug string, input []byte) ([]byte, error) {
	p, err := hf.resolvePlugin(pluginSlug)
	if err != nil {
		return nil, err
	}

	if !hf.Permissions.HasPermission(p, PermVPNWrite) {
		return nil, fmt.Errorf("%w: plugin %q lacks vpn:write permission", ErrPermissionDenied, pluginSlug)
	}

	var req sdk.VPNRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, fmt.Errorf("unmarshal vpn request: %w", err)
	}

	if req.Timeout > 0 {
		timeout := time.Duration(req.Timeout) * time.Millisecond
		if timeout > MaxVPNRequestTimeout {
			timeout = MaxVPNRequestTimeout
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	if hf.vpnExecutor == nil {
		return nil, fmt.Errorf("vpn executor not configured")
	}

	resp, err := hf.vpnExecutor.Execute(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("execute vpn request: %w", err)
	}

	return json.Marshal(resp)
}

// StorageGet retrieves a value from the plugin's isolated key-value store.
// Requires the storage:read permission scope.
func (hf *HostFunctions) StorageGet(ctx context.Context, pluginSlug, key string) ([]byte, error) {
	if hf.Storage == nil {
		return nil, fmt.Errorf("storage service not configured")
	}
	p, err := hf.resolvePlugin(pluginSlug)
	if err != nil {
		return nil, err
	}
	if !hf.Permissions.HasPermission(p, PermStorageRead) {
		return nil, fmt.Errorf("%w: plugin %q lacks storage:read permission", ErrPermissionDenied, pluginSlug)
	}
	if err := ValidateStorageKey(key); err != nil {
		return nil, fmt.Errorf("validate storage key: %w", err)
	}
	return hf.Storage.Get(ctx, pluginSlug, key)
}

// StorageSet stores a value in the plugin's isolated key-value store.
// Requires the storage:readwrite permission scope.
func (hf *HostFunctions) StorageSet(ctx context.Context, pluginSlug, key string, value []byte, ttlSeconds int64) error {
	if hf.Storage == nil {
		return fmt.Errorf("storage service not configured")
	}
	p, err := hf.resolvePlugin(pluginSlug)
	if err != nil {
		return err
	}
	if !hf.Permissions.HasPermission(p, PermStorageWrite) {
		return fmt.Errorf("%w: plugin %q lacks storage:readwrite permission", ErrPermissionDenied, pluginSlug)
	}
	if err := ValidateStorageKey(key); err != nil {
		return fmt.Errorf("validate storage key: %w", err)
	}
	return hf.Storage.Set(ctx, pluginSlug, key, value, ttlSeconds)
}

// ConfigGet retrieves a configuration value for the plugin. Config values are
// set by platform admins and stored in the plugin registry.
func (hf *HostFunctions) ConfigGet(pluginSlug, key string) (string, error) {
	p, err := hf.resolvePlugin(pluginSlug)
	if err != nil {
		return "", err
	}

	val, ok := p.Config[key]
	if !ok {
		return "", fmt.Errorf("config key %q not found for plugin %q", key, pluginSlug)
	}
	return val, nil
}

// EmitEvent publishes a domain event on behalf of a plugin.
// Requires the notifications:emit permission scope.
func (hf *HostFunctions) EmitEvent(ctx context.Context, pluginSlug string, eventType string, payload []byte) error {
	if hf.Publisher == nil {
		return fmt.Errorf("event publisher not configured")
	}
	p, err := hf.resolvePlugin(pluginSlug)
	if err != nil {
		return err
	}
	if !hf.Permissions.HasPermission(p, PermNotificationsEmit) {
		return fmt.Errorf("%w: plugin %q lacks notifications:emit permission", ErrPermissionDenied, pluginSlug)
	}

	event := domainevent.NewAt(domainevent.EventType(eventType), map[string]any{
		"plugin_slug": pluginSlug,
		"payload":     string(payload),
	}, hf.Clock.Now())
	return hf.Publisher.Publish(ctx, event)
}

// Log writes a structured log entry on behalf of a plugin. The log level must
// be one of: debug, info, warn, error. Invalid levels default to info.
func (hf *HostFunctions) Log(pluginSlug, level, message string, fields map[string]string) {
	attrs := make([]slog.Attr, 0, len(fields)+1)
	attrs = append(attrs, slog.String("plugin_slug", pluginSlug))
	for k, v := range fields {
		attrs = append(attrs, slog.String(k, v))
	}

	args := make([]any, len(attrs))
	for i, a := range attrs {
		args[i] = a
	}

	switch level {
	case LogLevelDebug:
		hf.Logger.Debug(message, args...)
	case LogLevelWarn:
		hf.Logger.Warn(message, args...)
	case LogLevelError:
		hf.Logger.Error(message, args...)
	default:
		hf.Logger.Info(message, args...)
	}
}

// getOrCreateHTTPLimiter returns the existing rate limiter for a plugin slug,
// or atomically creates one with the given per-minute limit.
func (hf *HostFunctions) getOrCreateHTTPLimiter(slug string, maxPerMin int) *httpRateLimiter {
	if v, ok := hf.httpLimiters.Load(slug); ok {
		return v.(*httpRateLimiter)
	}
	limiter := newHTTPRateLimiter(maxPerMin, time.Minute)
	actual, _ := hf.httpLimiters.LoadOrStore(slug, limiter)
	return actual.(*httpRateLimiter)
}

// resolvePlugin uses the registry lookup function to find a Plugin by slug.
func (hf *HostFunctions) resolvePlugin(slug string) (*Plugin, error) {
	if hf.pluginRegistry == nil {
		return nil, fmt.Errorf("plugin registry not configured")
	}
	return hf.pluginRegistry(slug)
}
