package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
)

// Hook name constants for traffic lifecycle dispatch points.
// Pre-hooks (sync) are named "{entity}.{verb}ing" and fire before the state
// transition. Post-hooks (async) are named "{entity}.{past_tense}.post" and
// fire after the transition completes.
const (
	HookSubLimiting        = "subscription.limiting"
	HookSubLimitedPost     = "subscription.limited.post"
	HookSubUnlimiting      = "subscription.unlimiting"
	HookSubUnlimitedPost   = "subscription.unlimited.post"
	HookSubTrafficWarning  = "subscription.traffic_warning.post"
)

// trafficExceededReason is the standard reason passed to LimitBinding when
// traffic is exceeded.
const trafficExceededReason = "traffic_exceeded"

// OnBindingTrafficExceeded handles the binding.traffic_exceeded event from a
// Remnawave webhook. It dispatches the subscription.limiting sync hook to
// allow plugins to block or modify the behavior, then delegates to
// BindingLifecycleService.LimitBinding.
//
// If a plugin responds with Block=true, the limit is not applied and the
// method returns nil (the plugin is responsible for handling the situation,
// e.g., offering an upgrade or sending a custom notification).
//
// After a successful limit, the subscription.limited.post async hook is
// dispatched for plugin notification (fire-and-forget).
//
// Idempotency: BindingLifecycleService.LimitBinding returns nil if the
// binding is already in limited state.
func (o *MultiSubOrchestrator) OnBindingTrafficExceeded(
	ctx context.Context,
	bindingID string,
	subscriptionID string,
	usedBytes int64,
	limitBytes int64,
) error {
	// Dispatch subscription.limiting sync hook if available.
	if o.dispatcher != nil && o.hooksEnabled {
		req := sdk.SubLimitingRequest{
			SubscriptionID: subscriptionID,
			BindingID:      bindingID,
			UsedBytes:      usedBytes,
			LimitBytes:     limitBytes,
		}
		data, err := json.Marshal(req)
		if err != nil {
			o.logger.Warn("failed to marshal limiting hook payload",
				slog.String("binding_id", bindingID),
				slog.Any("error", err),
			)
		} else {
			result := o.dispatcher.DispatchSyncSafe(ctx, HookSubLimiting, data)
			if result.Err == nil && result.Payload != nil {
				var resp sdk.SubLimitingResponse
				if unmarshalErr := json.Unmarshal(result.Payload, &resp); unmarshalErr == nil && resp.Block {
					o.logger.Info("binding limit blocked by plugin",
						slog.String("binding_id", bindingID),
						slog.String("subscription_id", subscriptionID),
					)
					return nil
				}
			}
		}
	}

	if err := o.lifecycle.LimitBinding(ctx, bindingID, trafficExceededReason); err != nil {
		return fmt.Errorf("limit binding %s: %w", bindingID, err)
	}

	// Fire async post-hook for plugin notification.
	o.dispatchAsync(ctx, HookSubLimitedPost, sdk.SubLimitedNotification{
		SubscriptionID: subscriptionID,
		BindingID:      bindingID,
		UsedBytes:      usedBytes,
		LimitBytes:     limitBytes,
	})

	return nil
}

// OnBindingTrafficReset handles the subscription.traffic_cycle_reset event.
// It dispatches the subscription.unlimiting sync hook (informational, never
// blocks), then delegates to BindingLifecycleService.UnlimitBinding.
//
// After a successful unlimit, the subscription.unlimited.post async hook is
// dispatched for plugin notification (fire-and-forget).
//
// Idempotency: BindingLifecycleService.UnlimitBinding returns nil if the
// binding is not in limited state.
func (o *MultiSubOrchestrator) OnBindingTrafficReset(
	ctx context.Context,
	bindingID string,
	subscriptionID string,
) error {
	// Dispatch subscription.unlimiting sync hook (informational, no blocking).
	if o.dispatcher != nil && o.hooksEnabled {
		req := sdk.SubUnlimitingRequest{
			SubscriptionID: subscriptionID,
			BindingID:      bindingID,
		}
		data, err := json.Marshal(req)
		if err != nil {
			o.logger.Warn("failed to marshal unlimiting hook payload",
				slog.String("binding_id", bindingID),
				slog.Any("error", err),
			)
		} else {
			o.dispatcher.DispatchSyncSafe(ctx, HookSubUnlimiting, data)
		}
	}

	if err := o.lifecycle.UnlimitBinding(ctx, bindingID); err != nil {
		return fmt.Errorf("unlimit binding %s: %w", bindingID, err)
	}

	// Fire async post-hook for plugin notification.
	o.dispatchAsync(ctx, HookSubUnlimitedPost, sdk.SubUnlimitedNotification{
		SubscriptionID: subscriptionID,
		BindingID:      bindingID,
	})

	return nil
}

// OnTrafficWarning handles the subscription.traffic_warning event. It fires
// an async hook for plugin notification only -- no binding state change occurs.
// This allows plugins to send user notifications (e.g., "you've used 80% of
// your traffic") or trigger other side effects.
func (o *MultiSubOrchestrator) OnTrafficWarning(
	ctx context.Context,
	bindingID string,
	subscriptionID string,
	usedBytes int64,
	limitBytes int64,
	thresholdPct int,
) {
	o.dispatchAsync(ctx, HookSubTrafficWarning, sdk.SubTrafficWarningNotification{
		SubscriptionID: subscriptionID,
		BindingID:      bindingID,
		UsedBytes:      usedBytes,
		LimitBytes:     limitBytes,
		ThresholdPct:   thresholdPct,
	})
}

// dispatchAsync fires an async hook if dispatcher is available and hooks are
// enabled. Errors are logged but never propagated (fire-and-forget).
func (o *MultiSubOrchestrator) dispatchAsync(ctx context.Context, hookName string, payload any) {
	if o.dispatcher == nil || !o.hooksEnabled {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		o.logger.Warn("failed to marshal async hook payload",
			slog.String("hook", hookName),
			slog.Any("error", err),
		)
		return
	}
	o.dispatcher.DispatchAsync(ctx, hookName, data)
}
