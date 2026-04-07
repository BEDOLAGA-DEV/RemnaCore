package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
)

// compensateChain calls "{hookName}.compensate" on each previously executed
// plugin in reverse order, passing the original payload. Best-effort: failures
// are logged but do not propagate.
//
// Uses a detached context (context.Background) because the original request
// context may already be cancelled, but compensation must still run to undo
// side-effects.
func (d *HookDispatcher) compensateChain(hookName string, executedPlugins []string, originalPayload json.RawMessage) []hookdispatch.FailedCompensation {
	compensateHook := hookName + compensateHookSuffix

	var failures []hookdispatch.FailedCompensation

	// Reverse order — last executed first.
	for i := len(executedPlugins) - 1; i >= 0; i-- {
		slug := executedPlugins[i]

		// Copy the registrations slice under lock to avoid races with
		// concurrent Register/Unregister calls.
		d.mu.RLock()
		src := d.registrations[compensateHook]
		regs := make([]HookRegistration, len(src))
		copy(regs, src)
		d.mu.RUnlock()

		var targetReg *HookRegistration
		for _, r := range regs {
			if r.PluginSlug == slug {
				targetReg = &r
				break
			}
		}
		if targetReg == nil {
			continue // No compensate handler for this plugin — skip silently.
		}

		hookCtx := sdk.HookContext{
			HookName:  compensateHook,
			RequestID: uuid.Must(uuid.NewV7()).String(),
			Timestamp: d.clock.Now().Unix(),
			PluginID:  slug,
			Payload:   originalPayload,
		}

		inputBytes, err := json.Marshal(hookCtx)
		if err != nil {
			d.logger.Error("failed to marshal compensation context",
				"hook", compensateHook, "plugin", slug, "error", err)
			failures = append(failures, hookdispatch.FailedCompensation{
				PluginSlug: slug,
				HookName:   compensateHook,
				Error:      fmt.Errorf("marshal compensation context: %w", err).Error(),
			})
			continue
		}

		timeout := d.syncTimeoutForPlugin(slug)
		callCtx, callCancel := context.WithTimeout(context.Background(), timeout)
		_, err = d.runtime.CallHook(callCtx, slug, targetReg.FuncName, inputBytes)
		callCancel()

		if err != nil {
			d.logger.Error("compensation hook failed",
				"hook", compensateHook, "plugin", slug, "error", err)
			failures = append(failures, hookdispatch.FailedCompensation{
				PluginSlug: slug,
				HookName:   compensateHook,
				Error:      err.Error(),
			})
		} else {
			d.logger.Info("compensation hook executed",
				"hook", compensateHook, "plugin", slug)
		}
	}

	return failures
}
