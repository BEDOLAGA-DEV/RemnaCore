// Package sdk defines the public Plugin Development Kit (PDK) types shared
// between the VPN platform runtime and plugin authors. Keep this package
// minimal and stable -- it is the contract across the WASM boundary.
package sdk

import (
	"context"
	"encoding/json"
)

// HookContext is the JSON envelope passed as input to every plugin hook
// function invoked by the runtime.
type HookContext struct {
	HookName  string          `json:"hook_name"`
	RequestID string          `json:"request_id"`
	Timestamp int64           `json:"timestamp"`
	PluginID  string          `json:"plugin_id"`
	Payload   json.RawMessage `json:"payload"`
}

// HookResult is the JSON envelope returned by a plugin hook function.
type HookResult struct {
	Action   HookAction      `json:"action"`
	Modified json.RawMessage `json:"modified,omitempty"`
	Error    string          `json:"error,omitempty"`
}

// HookAction describes what the runtime should do after a hook returns.
type HookAction string

const (
	// ActionContinue means no modification; pass the original payload through.
	ActionContinue HookAction = "continue"
	// ActionModify means the plugin has modified the payload (see Modified).
	ActionModify HookAction = "modify"
	// ActionHalt means the plugin wants to stop the hook chain and return an error.
	ActionHalt HookAction = "halt"
	// ActionRollback means the plugin wants to discard all accumulated
	// modifications from the chain and return the original payload.
	ActionRollback HookAction = "rollback"
)

// --- Host-function request/response types ---

// HTTPRequest describes an outbound HTTP call a plugin may ask the host to
// perform on its behalf.
type HTTPRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    []byte            `json:"body,omitempty"`
}

// HTTPResponse is the host's answer to an HTTPRequest.
type HTTPResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       []byte            `json:"body,omitempty"`
}

// StorageEntry represents a single key-value pair in the plugin's sandboxed
// key-value store.
type StorageEntry struct {
	Key   string `json:"key"`
	Value []byte `json:"value"`
}

// --- VPN provider request/response types ---

// VPNRequest represents a VPN provider API request built by a plugin.
// The plugin constructs the request spec; the host executes it with
// circuit breaker, retry, and connection pooling.
type VPNRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`                 // relative path, base URL from host config
	Headers map[string]string `json:"headers,omitempty"`
	Body    []byte            `json:"body,omitempty"`
	Timeout int               `json:"timeout_ms,omitempty"` // override, capped by host
}

// VPNResponse represents a VPN provider API response returned to a plugin for parsing.
type VPNResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       []byte            `json:"body,omitempty"`
}

// VPNHTTPExecutor executes VPN provider API requests on behalf of plugins.
// The host provides circuit breaker, retry, and connection pooling around the
// underlying HTTP transport. Implementations live in the adapter layer
// (internal/adapter/plugin); consumers live in both internal/plugin and
// internal/adapter/plugin, so the interface is defined here as shared contract.
type VPNHTTPExecutor interface {
	Execute(ctx context.Context, req VPNRequest) (*VPNResponse, error)
}
