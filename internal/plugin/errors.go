package plugin

import "errors"

var (
	ErrPluginNotFound        = errors.New("plugin not found")
	ErrPluginAlreadyExists   = errors.New("plugin already exists")
	ErrInvalidManifest       = errors.New("invalid plugin manifest")
	ErrInvalidPluginSlug     = errors.New("invalid plugin slug")
	ErrPluginNotEnabled      = errors.New("plugin is not enabled")
	ErrPluginAlreadyEnabled  = errors.New("plugin is already enabled")
	ErrHookTimeout           = errors.New("hook execution timed out")
	ErrHookHalted            = errors.New("hook execution halted by plugin")
	ErrPermissionDenied      = errors.New("permission denied for plugin")
	ErrStorageQuotaExceeded  = errors.New("plugin storage quota exceeded")
	ErrHTTPRateLimitExceeded = errors.New("plugin HTTP rate limit exceeded")
	ErrInternalNetworkAccess = errors.New("access to internal network addresses is forbidden")
	ErrWASMCompilationFailed = errors.New("WASM compilation failed")
	ErrNoHandlerForHook      = errors.New("no plugin handler for hook")
	ErrSlugMismatch          = errors.New("plugin slug mismatch during hot reload")
	ErrPluginNotRunning      = errors.New("plugin is not running")
	ErrIncompatibleSDK       = errors.New("incompatible plugin SDK version")
)
