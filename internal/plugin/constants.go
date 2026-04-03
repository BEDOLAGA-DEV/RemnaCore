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
// access.
type PermissionScope string

const (
	PermBillingRead       PermissionScope = "billing:read"
	PermBillingWrite      PermissionScope = "billing:write"
	PermPaymentWrite      PermissionScope = "payment:write"
	PermUsersRead         PermissionScope = "users:read"
	PermUsersWrite        PermissionScope = "users:write"
	PermNotificationsEmit PermissionScope = "notifications:emit"
	PermAnalyticsWrite    PermissionScope = "analytics:write"
	PermStorageRead       PermissionScope = "storage:read"
	PermStorageWrite      PermissionScope = "storage:readwrite"
	PermAPIRoutes         PermissionScope = "api:routes"
	PermVPNRead           PermissionScope = "vpn:read"
	PermVPNWrite          PermissionScope = "vpn:write"
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
)

// Slug validation constraints.
const (
	MaxPluginSlugLen = 64
	PluginSlugPattern = `^[a-z0-9][a-z0-9_-]*$`
)

// Priority bounds.
const (
	MinHookPriority = 0
	MaxHookPriority = 100
)
