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
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
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
	logger        *slog.Logger
}

// NewHookDispatcher creates a dispatcher wired to the given runtime pool and
// event publisher.
func NewHookDispatcher(runtime *RuntimePool, publisher domainevent.Publisher, logger *slog.Logger) *HookDispatcher {
	return &HookDispatcher{
		registrations: make(map[string][]HookRegistration),
		runtime:       runtime,
		publisher:     publisher,
		logger:        logger,
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

// DispatchSync executes synchronous hooks in priority order, chaining payload
// modifications from one plugin to the next. If any plugin returns action
// "halt", the chain stops and ErrHookHalted is returned.
func (d *HookDispatcher) DispatchSync(ctx context.Context, hookName string, payload json.RawMessage) (json.RawMessage, error) {
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
			RequestID: uuid.New().String(),
			Timestamp: time.Now().Unix(),
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

		start := time.Now()
		output, err := d.runtime.CallHook(callCtx, reg.PluginSlug, reg.FuncName, inputBytes)
		durationMs := time.Since(start).Milliseconds()
		callCancel()

		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrHookTimeout) {
				d.logger.Error("hook execution timed out",
					"hook", hookName, "plugin", reg.PluginSlug, "timeout", timeout, "duration_ms", durationMs)
				if d.publisher != nil {
					if pubErr := d.publisher.Publish(ctx, NewHookFailedEvent(reg.PluginID, reg.PluginSlug, hookName, "timed out")); pubErr != nil {
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
				if pubErr := d.publisher.Publish(ctx, NewHookFailedEvent(reg.PluginID, reg.PluginSlug, hookName, err.Error())); pubErr != nil {
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
			d.logger.Error("failed to unmarshal hook result",
				"hook", hookName, "plugin", reg.PluginSlug, "error", err)
			return nil, fmt.Errorf("invalid hook result from plugin %q: %w", reg.PluginSlug, err)
		}

		if d.publisher != nil {
			if pubErr := d.publisher.Publish(ctx, NewHookExecutedEvent(reg.PluginID, reg.PluginSlug, hookName, durationMs)); pubErr != nil {
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

	event := domainevent.New(domainevent.EventType(asyncHookSubjectPrefix+hookName), map[string]any{
		"hook_name": hookName,
		"payload":   string(payload),
	})

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
