package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
)

// MaxCompensationDuration is the total deadline for the entire compensation
// chain. Individual plugin timeouts are capped by the remaining budget.
const MaxCompensationDuration = 15 * time.Second

// compensateChain calls "{hookName}.compensate" on each previously executed
// plugin in reverse order, passing the original payload. Best-effort: failures
// are logged but do not propagate.
//
// Uses a detached context (context.Background) because the original request
// context may already be cancelled, but compensation must still run to undo
// side-effects.
//
// IMPORTANT: Compensation hooks MUST be idempotent. If a business operation
// is retried after partial compensation, previously compensated plugins will
// receive their .compensate hook again. There is no built-in tracking of
// which compensations have already executed — plugins must handle duplicates
// gracefully (e.g., by checking current state before reverting).
func (d *HookDispatcher) compensateChain(hookName string, executedPlugins []string, originalPayload json.RawMessage) []hookdispatch.FailedCompensation {
	compensateHook := hookName + compensateHookSuffix

	// Create a shared deadline for the entire compensation chain.
	ctx, cancel := context.WithTimeout(context.Background(), MaxCompensationDuration)
	defer cancel()

	var failures []hookdispatch.FailedCompensation

	// Reverse order — last executed first.
	for i := len(executedPlugins) - 1; i >= 0; i-- {
		slug := executedPlugins[i]

		// Check if total deadline exceeded before calling next plugin.
		if ctx.Err() != nil {
			d.logger.Error("compensation chain deadline exceeded, skipping remaining",
				"hook", hookName,
				"remaining_plugins", i+1,
			)
			for j := i; j >= 0; j-- {
				failures = append(failures, hookdispatch.FailedCompensation{
					PluginSlug: executedPlugins[j],
					HookName:   compensateHook,
					Error:      "compensation chain deadline exceeded",
				})
			}
			break
		}

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

		// Per-plugin timeout capped by remaining chain deadline.
		perPlugin := d.syncTimeoutForPlugin(slug)
		deadline, _ := ctx.Deadline()
		remaining := time.Until(deadline)
		if perPlugin > remaining {
			perPlugin = remaining
		}

		callCtx, callCancel := context.WithTimeout(ctx, perPlugin)
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
