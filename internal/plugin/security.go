// Security Model
//
// The RemnaCore plugin system enforces defense-in-depth with multiple security
// layers. Each layer operates independently so that a bypass of one layer does
// not compromise the entire sandbox.
//
// # Layer 1: WASM Sandbox (Extism/wazero)
//
// Each plugin instance runs inside an isolated WebAssembly linear memory space
// managed by the wazero runtime:
//
//   - Memory isolation: every plugin instance has its own linear memory; one
//     plugin cannot read or write another plugin's memory.
//   - Memory cap (ENFORCED): ManifestLimits.MaxMemoryMB (default:
//     [DefaultMaxMemoryMB], hard cap: [MaxMemoryMB]) limits WASM linear memory
//     growth. Enforced via [extism.ManifestMemory.MaxPages] in
//     [applyManifestLimits] (1 MB = [WASMPagesPerMB] pages of 64 KB each).
//     Exceeding the limit causes a wazero trap.
//   - CPU budget (INDIRECT): the Extism Go SDK does not expose fuel-based CPU
//     limits directly. ManifestLimits.MaxFuel is mapped to the Extism manifest
//     Timeout field (milliseconds), which cancels execution via context after
//     the elapsed time. A plugin that enters an infinite loop will be
//     terminated after the timeout elapses. For sync hooks, an additional
//     per-call timeout from ManifestLimits.TimeoutSyncMs (default:
//     [DefaultSyncTimeoutMs]) is applied by the dispatcher. Together these
//     provide bounded CPU usage without true fuel metering.
//   - No direct filesystem, network, or system call access: all I/O is
//     mediated through host functions.
//
// See [applyManifestLimits] in extism_runner.go for the enforcement
// implementation.
//
// # Layer 2: Permission System (runtime enforcement)
//
// Every host function call checks [PermissionChecker.HasPermission] at runtime
// before executing. Permissions are declared in the plugin manifest and granted
// at install time:
//
//   - Scopes follow the "{resource}:{access}" pattern (e.g. "billing:read",
//     "storage:readwrite").
//   - Write scopes implicitly grant the corresponding read scope
//     (e.g. "billing:write" implies "billing:read").
//   - HTTP requests require explicit URL allowlist matching via
//     [PermissionChecker.ValidateHTTPRequest].
//   - See [PermissionScope] constants in model.go for all defined scopes.
//
// # Layer 3: Network Security (SSRF protection)
//
// Outbound HTTP requests from plugins pass through four independent checks
// (see ssrf.go for implementation details):
//
//  1. URL path normalization: rejects ".." traversal sequences via
//     [normalizeURL].
//  2. Manifest allowlist validation: the target URL must match at least one
//     pattern in the plugin's HTTP allowlist.
//  3. Pre-flight hostname check: [isBlockedHostname] blocks known
//     loopback/local/cloud-metadata hostnames without DNS resolution.
//  4. Transport-level dial guard: [ssrfSafeDialContext] resolves the hostname,
//     checks ALL resolved IPs against [blockedCIDRs], and dials by resolved
//     IP (not hostname) to prevent DNS rebinding attacks.
//
// Additionally, per-plugin HTTP rate limiting (ManifestLimits.MaxHTTPCallsPerMin)
// prevents abuse through volume.
//
// # Layer 4: Storage Isolation
//
// Plugin key-value storage is namespaced by plugin slug. Each plugin can only
// access its own keys:
//
//   - Key validation via [ValidateStorageKey] rejects path traversal (".."),
//     path separators ("/" and "\"), null bytes, control characters, and
//     oversized keys.
//   - Quota enforcement is atomic: an advisory lock per plugin slug prevents
//     concurrent Set requests from bypassing the quota check (TOCTOU-safe).
//     See StorageAdvisoryLock in plugin_storage_repo.go.
//   - TTL support enables automatic expiration of temporary data.
//   - Keys are limited to [MaxStorageKeyLen] bytes.
//
// # Layer 5: Hook Dispatch Safety
//
// The [HookDispatcher] protects the host from misbehaving plugins:
//
//   - [HookDispatcher.DispatchSyncSafe]: plugin errors do not block business
//     logic. Failed hooks trigger compensation chains that undo side-effects
//     in reverse priority order.
//   - Per-plugin-hook circuit breaker prevents cascading failures from
//     sustained plugin errors.
//   - Flow pinning via [HookDispatcher.BeginFlow] ensures multi-hook business
//     flows use consistent plugin versions even during hot reloads.
//   - Invalid or malformed JSON responses from plugins are handled gracefully
//     with descriptive errors rather than panics.
//
// # Layer 6: Hot Reload Safety
//
// Plugin updates are applied with zero-downtime guarantees:
//
//   - New pool starts before old pool drains: new requests immediately use
//     the updated plugin.
//   - In-flight requests complete on the old pool within [DrainTimeout].
//   - CAS rollback: the old WASM binary is restored if the new one fails to
//     load or compile.
//   - Atomic hook swap via [HookDispatcher.SwapHooks]: there is no window
//     where the plugin has zero hooks registered.
//   - Retired pools remain available for flow-pinned callers during
//     [RetiredPoolGracePeriod].
//
// # Layer 7: Runner Lifecycle
//
// WASM runner instances are proactively recycled to prevent silent degradation:
//
//   - Each runner is replaced after [MaxRunnerUses] invocations.
//   - Corruption detection via [isRunnerCorrupted] inspects errors for WASM
//     fault indicators (memory, trap, unreachable, etc.) and discards
//     corrupted runners immediately.
//   - Replacement runners are created asynchronously to avoid blocking the
//     request path.
package plugin
