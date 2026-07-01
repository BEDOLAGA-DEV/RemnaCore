package plugin

// PluginStatus describes the lifecycle state of an installed plugin.
type PluginStatus string

const (
	StatusInstalled PluginStatus = "installed"
	StatusEnabled   PluginStatus = "enabled"
	StatusDisabled  PluginStatus = "disabled"
	StatusError     PluginStatus = "error"
)

// HookType distinguishes synchronous (blocking) hooks from asynchronous
// (fire-and-forget) hooks.
type HookType string

const (
	HookSync  HookType = "sync"
	HookAsync HookType = "async"
)

// PermissionScope is a typed scope string that governs what a plugin may
// access. Scopes fall into two classes:
//
//   - Runtime-enforced: checked by PermissionChecker.HasPermission before a
//     host function runs. For a sandboxed WASM guest only two host functions
//     are registered (log, vpn_request — see extism_runner.go), so the only
//     in-guest runtime-enforced scope is vpn:*; storage:* and
//     notifications:emit gate host calls used by compiled-in builtins.
//   - Declarative metadata: recorded on the plugin to express intent but NOT
//     checked at runtime today — no host function consumes them. They may gate
//     future host functions; do not assume they grant a capability.
type PermissionScope string

const (
	// Runtime-enforced scopes (gate a host function via HasPermission).
	PermNotificationsEmit PermissionScope = "notifications:emit"
	PermStorageRead       PermissionScope = "storage:read"
	PermStorageWrite      PermissionScope = "storage:readwrite"
	PermVPNRead           PermissionScope = "vpn:read"
	PermVPNWrite          PermissionScope = "vpn:write"

	// Telegram bot host op scopes (runtime-enforced by the bothost Registry).
	PermTelegramSend PermissionScope = "telegram:send"

	// Declarative metadata scopes (not runtime-enforced today), EXCEPT
	// PermUsersWrite which is also runtime-enforced by the bothost Registry
	// (it gates the user.register bot op).
	PermBillingRead    PermissionScope = "billing:read"
	PermBillingWrite   PermissionScope = "billing:write"
	PermPaymentWrite   PermissionScope = "payment:write"
	PermUsersRead      PermissionScope = "users:read"
	PermUsersWrite     PermissionScope = "users:write" // runtime-enforced by bothost (user.register)
	PermAnalyticsWrite PermissionScope = "analytics:write"
	PermAPIRoutes      PermissionScope = "api:routes"
)

// Default resource limits applied when a plugin manifest omits explicit values.
const (
	DefaultMaxMemoryMB        = 64
	DefaultMaxFuel            = 1_000_000 // WASM fuel units (CPU budget)
	DefaultMaxStorageMB       = 100
	DefaultMaxHTTPCallsPerMin = 100
	DefaultSyncTimeoutMs      = 5000  // 5 seconds
	DefaultAsyncTimeoutMs     = 30000 // 30 seconds
	DefaultPluginPriority     = 50    // middle priority (0 = first, 100 = last)

	// MaxMemoryMB is the upper bound for WASM memory to prevent wazero panics.
	// 4096 MB = 65536 pages (wazero hard maximum).
	MaxMemoryMB = 4096
)

// WASM instance pool sizing.
const (
	DefaultPoolSize = 4  // instances per plugin
	MaxPoolSize     = 16 // hard cap

	// MaxRunnerUses is the maximum number of calls a single WASM runner handles
	// before being replaced with a fresh instance. This prevents silent
	// degradation from memory leaks or accumulated state in long-running WASM
	// instances.
	MaxRunnerUses = 10_000
)

// MaxWASMBinarySize is the maximum allowed size for a WASM plugin binary (50 MB).
const MaxWASMBinarySize = 50 << 20

// WASMHashAlgorithm identifies the hash algorithm used for content-addressable
// WASM binary storage.
const WASMHashAlgorithm = "sha256"

// CurrentSDKVersion is the platform's current plugin SDK version.
// Plugins must declare a compatible sdk_version in their manifest.
const CurrentSDKVersion = "1.0.0"

// Slug validation constraints.
const (
	MaxPluginSlugLen  = 64
	PluginSlugPattern = `^[a-z0-9][a-z0-9_-]*$`
)

// Priority bounds.
const (
	MinHookPriority = 0
	MaxHookPriority = 100
)

// Permission manifest values used in ParsePermissions to map human-friendly
// TOML strings to typed PermissionScope values.
const (
	PermValueRead      = "read"
	PermValueWrite     = "write"
	PermValueReadWrite = "readwrite"
	PermValueEmit      = "emit"
	PermValueRoutes    = "routes"
	PermValueSend      = "send"
)

// DefaultBotEntry is the WASM export name used when a bot plugin manifest
// omits the [telegram].entry field.
const DefaultBotEntry = "handle_update"

// Page validation constants.
const (
	// PagePathPattern validates page path segments: lowercase alphanumeric
	// with hyphens, must start with a letter or digit.
	PagePathPattern = `^[a-z0-9][a-z0-9-]*$`

	// MaxPagePathLen is the maximum allowed length for a page path segment.
	MaxPagePathLen = 64

	// MaxPagesPerPlugin is the maximum number of UI pages a single plugin may
	// declare in its manifest.
	MaxPagesPerPlugin = 20

	// PageMenuAdmin indicates the page appears in the admin panel navigation.
	PageMenuAdmin = "admin"

	// PageMenuCabinet indicates the page appears in the customer cabinet navigation.
	PageMenuCabinet = "cabinet"
)

// Route validation constants.
const (
	// MaxRoutesPerPlugin is the maximum number of HTTP routes a single plugin
	// may declare in its manifest.
	MaxRoutesPerPlugin = 50

	// RouteFunctionPattern validates route function names: lowercase
	// alphanumeric with underscores.
	RouteFunctionPattern = `^[a-z0-9_]+$`

	// RoutePathPrefix is the required prefix for all plugin-declared routes.
	RoutePathPrefix = "/api/"
)
