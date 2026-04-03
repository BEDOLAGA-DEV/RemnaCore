// Package sdk provides types and utilities for building RemnaCore plugins.
//
// Plugins are compiled to WebAssembly (WASM) and executed in a sandboxed runtime.
// They communicate with the platform through exported functions and host functions.
//
// # Quick Start
//
//	vpnctl plugin init --lang go --name my-plugin --hooks pricing.calculate
//	cd my-plugin
//	# edit main.go
//	make build
//	vpnctl plugin install ./plugin.wasm
//	vpnctl plugin enable my-plugin
//
// # Hook Functions
//
// Plugins export functions that are called when specific events occur:
//   - Sync hooks: called in the request path, can modify the payload
//   - Async hooks: called asynchronously via NATS, fire-and-forget
//
// Each exported function must follow the naming convention on_{hook_name} and
// return an int32 status code (0 = success, non-zero = error).
//
// # Host Functions
//
// The platform provides these capabilities to plugins:
//   - pdk_http_request: make outbound HTTP requests (allowlist-gated)
//   - pdk_config_get: read plugin configuration
//   - pdk_db_query/pdk_db_write: access isolated key-value storage
//   - pdk_emit_event: publish events to the platform event bus
//   - pdk_log: structured logging
//
// # Types
//
// [HookContext] is the JSON envelope passed as input to every plugin hook.
// [HookResult] is the JSON envelope returned by a plugin hook.
// [HookAction] controls what the runtime does after a hook returns:
//   - [ActionContinue]: pass the original payload through
//   - [ActionModify]: use the modified payload
//   - [ActionHalt]: stop the hook chain and return an error
//
// [HTTPRequest] and [HTTPResponse] model outbound HTTP calls made through the
// pdk_http_request host function.
//
// [StorageEntry] represents a key-value pair in the plugin's sandboxed store.
//
// # PDK Helper Functions
//
// The following functions are available in the plugin WASM environment:
//   - Input() []byte: read the hook context from the host
//   - Output(result HookResult): write the hook result back to the host
//
// These are thin wrappers around the host function ABI and handle JSON
// serialization automatically.
package sdk
