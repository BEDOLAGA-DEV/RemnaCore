package plugin

import (
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// Plugin-specific event types.
const (
	EventPluginInstalled   domainevent.EventType = "plugin.installed"
	EventPluginEnabled     domainevent.EventType = "plugin.enabled"
	EventPluginDisabled    domainevent.EventType = "plugin.disabled"
	EventPluginUninstalled  domainevent.EventType = "plugin.uninstalled"
	EventPluginHotReloaded domainevent.EventType = "plugin.hot_reloaded"
	EventPluginError       domainevent.EventType = "plugin.error"
	EventHookExecuted      domainevent.EventType = "plugin.hook.executed"
	EventHookFailed        domainevent.EventType = "plugin.hook.failed"
)

// Event is an alias for the shared domainevent.Event so that callers within the
// plugin context can reference plugin.Event without importing pkg/domainevent.
type Event = domainevent.Event

// EventType is an alias for the shared domainevent.EventType.
type EventType = domainevent.EventType

// --- Plugin lifecycle event factories ---

// NewPluginInstalledEvent creates an event for a newly installed plugin.
func NewPluginInstalledEvent(pluginID, slug, version string, now time.Time) Event {
	return domainevent.NewAt(EventPluginInstalled, map[string]any{
		"plugin_id": pluginID,
		"slug":      slug,
		"version":   version,
	}, now)
}

// NewPluginEnabledEvent creates an event for a plugin being enabled.
func NewPluginEnabledEvent(pluginID, slug string, now time.Time) Event {
	return domainevent.NewAt(EventPluginEnabled, map[string]any{
		"plugin_id": pluginID,
		"slug":      slug,
	}, now)
}

// NewPluginDisabledEvent creates an event for a plugin being disabled.
func NewPluginDisabledEvent(pluginID, slug string, now time.Time) Event {
	return domainevent.NewAt(EventPluginDisabled, map[string]any{
		"plugin_id": pluginID,
		"slug":      slug,
	}, now)
}

// NewPluginUninstalledEvent creates an event for a plugin being removed.
func NewPluginUninstalledEvent(pluginID, slug string, now time.Time) Event {
	return domainevent.NewAt(EventPluginUninstalled, map[string]any{
		"plugin_id": pluginID,
		"slug":      slug,
	}, now)
}

// NewPluginHotReloadedEvent creates an event for a plugin that was atomically
// replaced with a new version while running.
func NewPluginHotReloadedEvent(pluginID, slug, oldVersion, newVersion string, now time.Time) Event {
	return domainevent.NewAt(EventPluginHotReloaded, map[string]any{
		"plugin_id":   pluginID,
		"slug":        slug,
		"old_version": oldVersion,
		"new_version": newVersion,
	}, now)
}

// NewPluginErrorEvent creates an event for a plugin entering the error state.
func NewPluginErrorEvent(pluginID, slug, reason string, now time.Time) Event {
	return domainevent.NewAt(EventPluginError, map[string]any{
		"plugin_id": pluginID,
		"slug":      slug,
		"reason":    reason,
	}, now)
}

// --- Hook execution event factories ---

// NewHookExecutedEvent creates an event for a successful hook invocation.
func NewHookExecutedEvent(pluginID, slug, hookName string, durationMs int64, now time.Time) Event {
	return domainevent.NewAt(EventHookExecuted, map[string]any{
		"plugin_id":   pluginID,
		"slug":        slug,
		"hook_name":   hookName,
		"duration_ms": durationMs,
	}, now)
}

// NewHookFailedEvent creates an event for a failed hook invocation.
func NewHookFailedEvent(pluginID, slug, hookName, reason string, now time.Time) Event {
	return domainevent.NewAt(EventHookFailed, map[string]any{
		"plugin_id": pluginID,
		"slug":      slug,
		"hook_name": hookName,
		"reason":    reason,
	}, now)
}
