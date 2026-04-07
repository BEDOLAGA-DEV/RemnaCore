package plugin

import (
	"context"
	"time"
)

// recordHookDuration observes the hook execution duration in the Prometheus
// histogram. Safe to call when metrics is nil (e.g. in tests).
func (d *HookDispatcher) recordHookDuration(pluginSlug, hookName string, seconds float64) {
	if d.metrics == nil {
		return
	}
	d.metrics.PluginHookDuration.WithLabelValues(pluginSlug, hookName).Observe(seconds)
}

// recordHookError increments the Prometheus error counter for the given
// plugin/hook pair. Safe to call when metrics is nil.
func (d *HookDispatcher) recordHookError(pluginSlug, hookName string) {
	if d.metrics == nil {
		return
	}
	d.metrics.PluginHookErrors.WithLabelValues(pluginSlug, hookName).Inc()
}

// recordHookTotal increments the Prometheus invocation counter with the hook
// action label. Safe to call when metrics is nil.
func (d *HookDispatcher) recordHookTotal(pluginSlug, hookName, action string) {
	if d.metrics == nil {
		return
	}
	d.metrics.PluginHookTotal.WithLabelValues(pluginSlug, hookName, action).Inc()
}

// recordCircuitBreakerTrip increments the Prometheus counter for circuit
// breaker trip events. Safe to call when metrics is nil.
func (d *HookDispatcher) recordCircuitBreakerTrip(pluginSlug, hookName string) {
	if d.metrics == nil {
		return
	}
	d.metrics.HookCircuitBreakerTrips.WithLabelValues(pluginSlug, hookName).Inc()
}

// syncTimeoutForPlugin returns the sync hook timeout for the given plugin. If
// the plugin is loaded and its manifest declares a custom timeout_sync_ms, that
// value is used. Otherwise, the platform default (DefaultSyncTimeoutMs) applies.
func (d *HookDispatcher) syncTimeoutForPlugin(slug string) time.Duration {
	inst, err := d.runtime.GetInstance(slug)
	if err == nil && inst.Manifest != nil {
		limits, _ := inst.Manifest.EffectiveLimits()
		if limits.TimeoutSyncMs > 0 {
			return time.Duration(limits.TimeoutSyncMs) * time.Millisecond
		}
	}
	return time.Duration(DefaultSyncTimeoutMs) * time.Millisecond
}

// BeginFlow snapshots the current plugin pool versions and returns a context
// that pins those versions for all subsequent CallHook invocations. If the
// context already carries flow bindings, it is returned unchanged to avoid
// overwriting an existing pin.
func (d *HookDispatcher) BeginFlow(ctx context.Context) context.Context {
	if flowBindingsFromContext(ctx) != nil {
		return ctx
	}
	bindings := d.runtime.CaptureFlowBindings()
	return withFlowBindings(ctx, bindings)
}
