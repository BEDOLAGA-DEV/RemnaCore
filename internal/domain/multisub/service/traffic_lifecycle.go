package service

import (
	"context"
	"fmt"
	"log/slog"
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
	if o.limitingHook != nil && o.hooksEnabled {
		resp, err := o.limitingHook(ctx, SubLimitingPayload{
			SubscriptionID: subscriptionID,
			BindingID:      bindingID,
			UsedBytes:      usedBytes,
			LimitBytes:     limitBytes,
		})
		if err != nil {
			o.logger.Warn("limiting hook dispatch failed, proceeding with limit",
				slog.String("binding_id", bindingID),
				slog.Any("error", err),
			)
		} else if resp != nil && resp.Block {
			o.logger.Info("binding limit blocked by plugin",
				slog.String("binding_id", bindingID),
				slog.String("subscription_id", subscriptionID),
			)
			return nil
		}
	}

	if err := o.lifecycle.LimitBinding(ctx, bindingID, trafficExceededReason); err != nil {
		return fmt.Errorf("limit binding %s: %w", bindingID, err)
	}

	// Fire async post-hook for plugin notification.
	o.dispatchAsync(ctx, HookSubLimitedPost, SubLimitedNotification{
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
	if o.syncHook != nil && o.hooksEnabled {
		_, err := o.syncHook(ctx, HookSubUnlimiting, SubUnlimitingPayload{
			SubscriptionID: subscriptionID,
			BindingID:      bindingID,
		})
		if err != nil {
			o.logger.Warn("unlimiting hook dispatch failed, proceeding",
				slog.String("binding_id", bindingID),
				slog.Any("error", err),
			)
		}
	}

	if err := o.lifecycle.UnlimitBinding(ctx, bindingID); err != nil {
		return fmt.Errorf("unlimit binding %s: %w", bindingID, err)
	}

	// Fire async post-hook for plugin notification.
	o.dispatchAsync(ctx, HookSubUnlimitedPost, SubUnlimitedNotification{
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
	o.dispatchAsync(ctx, HookSubTrafficWarning, SubTrafficWarningNotification{
		SubscriptionID: subscriptionID,
		BindingID:      bindingID,
		UsedBytes:      usedBytes,
		LimitBytes:     limitBytes,
		ThresholdPct:   thresholdPct,
	})
}

// dispatchAsync fires an async hook if the async hook function is available
// and hooks are enabled. Errors are logged but never propagated (fire-and-forget).
func (o *MultiSubOrchestrator) dispatchAsync(ctx context.Context, hookName string, payload any) {
	if o.asyncHook == nil || !o.hooksEnabled {
		return
	}
	o.asyncHook(ctx, hookName, payload)
}
