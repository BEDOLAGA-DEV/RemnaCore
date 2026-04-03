package plugin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
)

// Default timeout for outbound HTTP requests made by plugins.
const DefaultPluginHTTPTimeoutMs = 10000 // 10 seconds

// HostFunctions defines the platform capabilities injected into WASM plugins
// via host function calls. Each method corresponds to a pdk_* function that
// plugins can invoke from inside the sandbox.
type HostFunctions struct {
	Storage     StorageService
	Publisher   domainevent.Publisher
	Logger      *slog.Logger
	Permissions *PermissionChecker
	HTTPClient  *http.Client
	// pluginRegistry is used to look up a plugin instance for permission
	// checks. Populated during Fx wiring.
	pluginRegistry func(slug string) (*Plugin, error)
}

// NewHostFunctions creates a HostFunctions value with sensible defaults. Fields
// that depend on Fx-provided services (Storage, Publisher) can be nil and are
// expected to be set before the runtime starts.
func NewHostFunctions(logger *slog.Logger) *HostFunctions {
	return &HostFunctions{
		Logger:      logger,
		Permissions: &PermissionChecker{},
		HTTPClient: &http.Client{
			Timeout: time.Duration(DefaultPluginHTTPTimeoutMs) * time.Millisecond,
		},
	}
}

// SetPluginRegistry allows the lifecycle manager to inject a lookup function
// that resolves a slug to a full Plugin (needed for permission checks).
func (hf *HostFunctions) SetPluginRegistry(fn func(slug string) (*Plugin, error)) {
	hf.pluginRegistry = fn
}

// HTTPRequest performs an outbound HTTP request on behalf of a plugin after
// validating the target URL against the plugin's HTTP allowlist.
func (hf *HostFunctions) HTTPRequest(pluginSlug string, req sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	p, err := hf.resolvePlugin(pluginSlug)
	if err != nil {
		return nil, err
	}

	if !hf.Permissions.ValidateHTTPRequest(p, req.URL) {
		return nil, fmt.Errorf("%w: HTTP request to %s not in allowlist", ErrPermissionDenied, req.URL)
	}

	httpReq, err := http.NewRequest(req.Method, req.URL, nil)
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

	body, err := io.ReadAll(resp.Body)
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

// StorageGet retrieves a value from the plugin's isolated key-value store.
func (hf *HostFunctions) StorageGet(ctx context.Context, pluginSlug, key string) ([]byte, error) {
	if hf.Storage == nil {
		return nil, fmt.Errorf("storage service not configured")
	}
	return hf.Storage.Get(ctx, pluginSlug, key)
}

// StorageSet stores a value in the plugin's isolated key-value store.
func (hf *HostFunctions) StorageSet(ctx context.Context, pluginSlug, key string, value []byte, ttlSeconds int64) error {
	if hf.Storage == nil {
		return fmt.Errorf("storage service not configured")
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
func (hf *HostFunctions) EmitEvent(ctx context.Context, pluginSlug string, eventType string, payload []byte) error {
	if hf.Publisher == nil {
		return fmt.Errorf("event publisher not configured")
	}

	event := domainevent.New(domainevent.EventType(eventType), map[string]any{
		"plugin_slug": pluginSlug,
		"payload":     string(payload),
	})
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
	case "debug":
		hf.Logger.Debug(message, args...)
	case "warn":
		hf.Logger.Warn(message, args...)
	case "error":
		hf.Logger.Error(message, args...)
	default:
		hf.Logger.Info(message, args...)
	}
}

// resolvePlugin uses the registry lookup function to find a Plugin by slug.
func (hf *HostFunctions) resolvePlugin(slug string) (*Plugin, error) {
	if hf.pluginRegistry == nil {
		return nil, fmt.Errorf("plugin registry not configured")
	}
	return hf.pluginRegistry(slug)
}
