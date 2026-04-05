package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/observability"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tracing"
)

// NATS subject prefix for async hook dispatch.
const asyncHookSubjectPrefix = "plugin.hook."

// HookVersionSeparator separates a base hook name from its version number.
const HookVersionSeparator = ".v"

// DefaultHookVersion is the implicit version of an unversioned hook name.
const DefaultHookVersion = 1

// MinHookVersion is the lowest version that maps to a versioned hook name.
// Version 1 is always the unversioned (base) hook name.
const MinHookVersion = 2

// HookDispatcher routes hook invocations to registered plugins. Sync hooks
// execute in priority order with payload chaining; async hooks are published to
// NATS for background processing.
type HookDispatcher struct {
	mu            sync.RWMutex
	registrations map[string][]HookRegistration // hookName -> sorted registrations
	runtime       *RuntimePool
	publisher     domainevent.Publisher
	metrics       *observability.Metrics
	logger        *slog.Logger
	clock         clock.Clock
}

// NewHookDispatcher creates a dispatcher wired to the given runtime pool,
// event publisher, and Prometheus metrics collector.
func NewHookDispatcher(runtime *RuntimePool, publisher domainevent.Publisher, metrics *observability.Metrics, logger *slog.Logger, clk clock.Clock) *HookDispatcher {
	return &HookDispatcher{
		registrations: make(map[string][]HookRegistration),
		runtime:       runtime,
		publisher:     publisher,
		metrics:       metrics,
		logger:        logger,
		clock:         clk,
	}
}

// RegisterHooks registers all hook registrations for a plugin. Existing
// registrations for the same plugin slug are NOT removed automatically; call
// UnregisterHooks first if replacing.
func (d *HookDispatcher) RegisterHooks(regs []HookRegistration) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, reg := range regs {
		d.registrations[reg.HookName] = append(d.registrations[reg.HookName], reg)
	}

	// Re-sort all affected hooks by priority (lower number = higher priority = runs first).
	affected := make(map[string]struct{})
	for _, reg := range regs {
		affected[reg.HookName] = struct{}{}
	}
	for hookName := range affected {
		sort.Slice(d.registrations[hookName], func(i, j int) bool {
			return d.registrations[hookName][i].Priority < d.registrations[hookName][j].Priority
		})
	}
}

// UnregisterHooks removes all hook registrations for the given plugin slug.
func (d *HookDispatcher) UnregisterHooks(pluginSlug string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for hookName, regs := range d.registrations {
		filtered := regs[:0]
		for _, r := range regs {
			if r.PluginSlug != pluginSlug {
				filtered = append(filtered, r)
			}
		}
		if len(filtered) == 0 {
			delete(d.registrations, hookName)
		} else {
			d.registrations[hookName] = filtered
		}
	}
}

// SwapHooks atomically replaces all hooks for a plugin. This holds the write
// lock across both the unregister and register operations, preventing a window
// where the plugin has zero hooks registered.
func (d *HookDispatcher) SwapHooks(pluginSlug string, newRegs []HookRegistration) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Remove old hooks for this plugin.
	for hookName, regs := range d.registrations {
		filtered := make([]HookRegistration, 0, len(regs))
		for _, r := range regs {
			if r.PluginSlug != pluginSlug {
				filtered = append(filtered, r)
			}
		}
		if len(filtered) > 0 {
			d.registrations[hookName] = filtered
		} else {
			delete(d.registrations, hookName)
		}
	}

	// Add new hooks.
	for _, reg := range newRegs {
		d.registrations[reg.HookName] = append(d.registrations[reg.HookName], reg)
	}

	// Re-sort all affected hooks by priority.
	affected := make(map[string]struct{})
	for _, reg := range newRegs {
		affected[reg.HookName] = struct{}{}
	}
	for hookName := range affected {
		sort.Slice(d.registrations[hookName], func(i, j int) bool {
			return d.registrations[hookName][i].Priority < d.registrations[hookName][j].Priority
		})
	}
}

// DispatchSync executes synchronous hooks in priority order, chaining payload
// modifications from one plugin to the next. If any plugin returns action
// "halt", the chain stops and ErrHookHalted is returned.
func (d *HookDispatcher) DispatchSync(ctx context.Context, hookName string, payload json.RawMessage) (json.RawMessage, error) {
	ctx, span := tracing.StartSpan(ctx, "plugin.dispatch_sync."+hookName)
	defer span.End()

	d.mu.RLock()
	regs := make([]HookRegistration, len(d.registrations[hookName]))
	copy(regs, d.registrations[hookName])
	d.mu.RUnlock()

	// No handlers registered — pass through unchanged.
	if len(regs) == 0 {
		return payload, nil
	}

	currentPayload := payload

	for _, reg := range regs {
		// Only dispatch sync hooks in the sync path.
		if reg.HookType != HookSync {
			continue
		}

		hookCtx := sdk.HookContext{
			HookName:  hookName,
			RequestID: uuid.Must(uuid.NewV7()).String(),
			Timestamp: d.clock.Now().Unix(),
			PluginID:  reg.PluginSlug,
			Payload:   currentPayload,
		}

		inputBytes, err := json.Marshal(hookCtx)
		if err != nil {
			d.logger.Error("failed to marshal hook context",
				"hook", hookName, "plugin", reg.PluginSlug, "error", err)
			continue
		}

		// Resolve per-plugin timeout from manifest, falling back to the
		// platform default.
		timeout := d.syncTimeoutForPlugin(reg.PluginSlug)
		callCtx, callCancel := context.WithTimeout(ctx, timeout)

		start := d.clock.Now()
		output, err := d.runtime.CallHook(callCtx, reg.PluginSlug, reg.FuncName, inputBytes)
		elapsed := d.clock.Now().Sub(start)
		durationMs := elapsed.Milliseconds()
		callCancel()

		// Record Prometheus metrics for every call path.
		d.recordHookDuration(reg.PluginSlug, hookName, elapsed.Seconds())

		if err != nil {
			d.recordHookError(reg.PluginSlug, hookName)
			d.recordHookTotal(reg.PluginSlug, hookName, "error")

			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrHookTimeout) {
				d.logger.Error("hook execution timed out",
					"hook", hookName, "plugin", reg.PluginSlug, "timeout", timeout, "duration_ms", durationMs)
				if d.publisher != nil {
					if pubErr := d.publisher.Publish(ctx, NewHookFailedEvent(reg.PluginID, reg.PluginSlug, hookName, "timed out", d.clock.Now())); pubErr != nil {
						d.logger.Warn("failed to publish event",
							"event_type", string(EventHookFailed),
							"error", pubErr.Error(),
						)
					}
				}
				return nil, fmt.Errorf("%w: plugin %q timed out after %v", ErrHookTimeout, reg.PluginSlug, timeout)
			}
			d.logger.Error("hook execution failed",
				"hook", hookName, "plugin", reg.PluginSlug, "error", err, "duration_ms", durationMs)
			if d.publisher != nil {
				if pubErr := d.publisher.Publish(ctx, NewHookFailedEvent(reg.PluginID, reg.PluginSlug, hookName, err.Error(), d.clock.Now())); pubErr != nil {
					d.logger.Warn("failed to publish event",
						"event_type", string(EventHookFailed),
						"error", pubErr.Error(),
					)
				}
			}
			return nil, fmt.Errorf("hook %q failed for plugin %q: %w", hookName, reg.PluginSlug, err)
		}

		var result sdk.HookResult
		if err := json.Unmarshal(output, &result); err != nil {
			d.recordHookError(reg.PluginSlug, hookName)
			d.recordHookTotal(reg.PluginSlug, hookName, "error")
			d.logger.Error("failed to unmarshal hook result",
				"hook", hookName, "plugin", reg.PluginSlug, "error", err)
			return nil, fmt.Errorf("invalid hook result from plugin %q: %w", reg.PluginSlug, err)
		}

		action := string(result.Action)
		switch result.Action {
		case sdk.ActionContinue, sdk.ActionModify, sdk.ActionHalt:
			// known action — use as-is
		default:
			action = "unknown"
		}
		d.recordHookTotal(reg.PluginSlug, hookName, action)

		if d.publisher != nil {
			if pubErr := d.publisher.Publish(ctx, NewHookExecutedEvent(reg.PluginID, reg.PluginSlug, hookName, durationMs, d.clock.Now())); pubErr != nil {
				d.logger.Warn("failed to publish event",
					"event_type", string(EventHookExecuted),
					"error", pubErr.Error(),
				)
			}
		}

		switch result.Action {
		case sdk.ActionContinue:
			// Payload unchanged, continue chain.
		case sdk.ActionModify:
			if result.Modified != nil {
				currentPayload = result.Modified
			}
		case sdk.ActionHalt:
			errMsg := result.Error
			if errMsg == "" {
				errMsg = "halted by plugin"
			}
			return nil, fmt.Errorf("%w: %s (plugin: %s)", ErrHookHalted, errMsg, reg.PluginSlug)
		default:
			d.logger.Warn("unknown hook action, treating as continue",
				"hook", hookName, "plugin", reg.PluginSlug, "action", result.Action)
		}
	}

	return currentPayload, nil
}

// DispatchSyncVersioned dispatches to the highest available version of a hook.
// It tries hookName.v{N}, hookName.v{N-1}, ..., down to the unversioned base
// name (which is implicitly v1). This allows plugins to register for a specific
// hook version and the platform to gracefully fall back.
func (d *HookDispatcher) DispatchSyncVersioned(ctx context.Context, hookName string, currentVersion int, payload json.RawMessage) (json.RawMessage, error) {
	for v := currentVersion; v >= MinHookVersion; v-- {
		versionedName := fmt.Sprintf("%s%s%d", hookName, HookVersionSeparator, v)
		if d.hasHandlers(versionedName) {
			return d.DispatchSync(ctx, versionedName, payload)
		}
	}
	// Fall back to unversioned (v1).
	return d.DispatchSync(ctx, hookName, payload)
}

// hasHandlers returns true if at least one handler is registered for the given
// hook name.
func (d *HookDispatcher) hasHandlers(hookName string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	regs, ok := d.registrations[hookName]
	return ok && len(regs) > 0
}

// DispatchAsync publishes a hook event to NATS for asynchronous processing.
// The payload is published to subject "plugin.hook.{hookName}".
func (d *HookDispatcher) DispatchAsync(ctx context.Context, hookName string, payload json.RawMessage) error {
	if d.publisher == nil {
		return fmt.Errorf("event publisher not configured")
	}

	event := domainevent.NewAt(domainevent.EventType(asyncHookSubjectPrefix+hookName), map[string]any{
		"hook_name": hookName,
		"payload":   string(payload),
	}, d.clock.Now())

	return d.publisher.Publish(ctx, event)
}

// Registrations returns a snapshot of current registrations for a hook name.
// Primarily useful for testing.
func (d *HookDispatcher) Registrations(hookName string) []HookRegistration {
	d.mu.RLock()
	defer d.mu.RUnlock()

	regs := d.registrations[hookName]
	out := make([]HookRegistration, len(regs))
	copy(out, regs)
	return out
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

// syncTimeoutForPlugin returns the sync hook timeout for the given plugin. If
// the plugin is loaded and its manifest declares a custom timeout_sync_ms, that
// value is used. Otherwise, the platform default (DefaultSyncTimeoutMs) applies.
func (d *HookDispatcher) syncTimeoutForPlugin(slug string) time.Duration {
	inst, err := d.runtime.GetInstance(slug)
	if err == nil && inst.Manifest != nil {
		limits := inst.Manifest.EffectiveLimits()
		if limits.TimeoutSyncMs > 0 {
			return time.Duration(limits.TimeoutSyncMs) * time.Millisecond
		}
	}
	return time.Duration(DefaultSyncTimeoutMs) * time.Millisecond
}

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
