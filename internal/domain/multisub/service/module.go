package service

import (
	"context"
	"fmt"
	"log/slog"

	multisubdomain "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// MultiSubOrchestrator is the facade that coordinates billing lifecycle events
// with the multisub domain. All billing event handlers route through this
// struct so that upstream callers do not need to know about individual sagas.
//
// Every handler is idempotent: duplicate event delivery (at-least-once
// semantics from OutboxRelay) is detected via existing binding state and
// silently skipped.
type MultiSubOrchestrator struct {
	provisioning   *ProvisioningSaga
	deprovisioning *DeprovisioningSaga
	syncService    *SyncService
	lifecycle      *BindingLifecycleService
	bindings       multisubdomain.BindingRepository
	publisher      domainevent.Publisher
	txRunner       txmanager.Runner
	clock          clock.Clock
	logger         *slog.Logger
	limitingHook   LimitingHookFn // nil-safe; all dispatches guarded
	syncHook       SyncHookFn     // nil-safe; all dispatches guarded
	asyncHook      AsyncHookFn    // nil-safe; all dispatches guarded
	hooksEnabled   bool           // subscription lifecycle hooks feature flag
}

// OrchestratorOption configures optional dependencies on MultiSubOrchestrator.
type OrchestratorOption func(*MultiSubOrchestrator)

// WithLimitingHook sets the typed subscription.limiting sync hook function.
func WithLimitingHook(fn LimitingHookFn) OrchestratorOption {
	return func(o *MultiSubOrchestrator) { o.limitingHook = fn }
}

// WithSyncHook sets the generic sync hook dispatch function for informational
// pre-hooks (e.g., subscription.unlimiting) that do not need typed responses.
func WithSyncHook(fn SyncHookFn) OrchestratorOption {
	return func(o *MultiSubOrchestrator) { o.syncHook = fn }
}

// WithAsyncHook sets the async hook dispatch function for post-commit
// notifications (e.g., subscription.limited.post).
func WithAsyncHook(fn AsyncHookFn) OrchestratorOption {
	return func(o *MultiSubOrchestrator) { o.asyncHook = fn }
}

// WithHooksEnabled enables subscription lifecycle hook dispatch points.
func WithHooksEnabled(enabled bool) OrchestratorOption {
	return func(o *MultiSubOrchestrator) { o.hooksEnabled = enabled }
}

// NewMultiSubOrchestrator creates a MultiSubOrchestrator with its saga
// dependencies. Optional dependencies (dispatcher, feature flags) are
// configured via OrchestratorOption functions to keep the constructor
// backward-compatible.
func NewMultiSubOrchestrator(
	provisioning *ProvisioningSaga,
	deprovisioning *DeprovisioningSaga,
	syncService *SyncService,
	lifecycle *BindingLifecycleService,
	bindings multisubdomain.BindingRepository,
	publisher domainevent.Publisher,
	txRunner txmanager.Runner,
	clk clock.Clock,
	logger *slog.Logger,
	opts ...OrchestratorOption,
) *MultiSubOrchestrator {
	o := &MultiSubOrchestrator{
		provisioning:   provisioning,
		deprovisioning: deprovisioning,
		syncService:    syncService,
		lifecycle:      lifecycle,
		bindings:       bindings,
		publisher:      publisher,
		txRunner:       txRunner,
		clock:          clk,
		logger:         logger,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// OnSubscriptionActivated is called when billing publishes
// subscription.activated. It provisions all needed Remnawave bindings.
//
// Idempotency: if active bindings already exist for the subscription, the
// event is treated as a duplicate and provisioning is skipped.
func (o *MultiSubOrchestrator) OnSubscriptionActivated(
	ctx context.Context,
	subscriptionID string,
	platformUserID string,
	plan multisubdomain.PlanSnapshot,
	addonIDs []string,
	familyMemberIDs []string,
) error {
	existing, err := o.bindings.GetActiveBySubscriptionID(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("check existing bindings: %w", err)
	}
	if len(existing) > 0 {
		o.logger.Info("skipping duplicate subscription.activated event",
			slog.String("subscription_id", subscriptionID),
			slog.Int("existing_bindings", len(existing)),
		)
		return nil
	}

	_, err = o.provisioning.Provision(ctx, ProvisionRequest{
		SubscriptionID:  subscriptionID,
		PlatformUserID:  platformUserID,
		Plan:            plan,
		AddonIDs:        addonIDs,
		FamilyMemberIDs: familyMemberIDs,
	})
	if err != nil {
		return fmt.Errorf("provision bindings: %w", err)
	}
	return nil
}

// cancelledPendingBindingReason is the fail reason recorded on pending bindings
// that are cancelled due to out-of-order event delivery (cancelled arrives
// before provisioning completes).
const cancelledPendingBindingReason = "subscription cancelled before provisioning completed (out-of-order delivery)"

// OnSubscriptionCancelled is called when billing publishes
// subscription.cancelled. It deprovisions all Remnawave bindings for the
// subscription (best-effort).
//
// Idempotency: if no active bindings remain, the event is treated as a
// duplicate and deprovisioning is skipped — UNLESS there are pending bindings,
// which indicates out-of-order delivery (cancelled arrived before activated
// finished provisioning). In that case, pending bindings are marked as failed
// to prevent the provisioning saga from completing.
func (o *MultiSubOrchestrator) OnSubscriptionCancelled(ctx context.Context, subscriptionID string) error {
	// Narrow read tx so the RLS GUC (carried on ctx) is applied to this
	// FORCE-RLS read before delegating to the deprovisioning saga.
	var existing []*aggregate.RemnawaveBinding
	err := o.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		var readErr error
		existing, readErr = o.bindings.GetActiveBySubscriptionID(txCtx, subscriptionID)
		return readErr
	})
	if err != nil {
		return fmt.Errorf("check existing bindings: %w", err)
	}
	if len(existing) == 0 {
		// No active bindings — but this might be because provisioning hasn't
		// completed yet (out-of-order delivery). Check for pending bindings
		// and mark them as failed to prevent the provisioning saga from
		// creating Remnawave users for a cancelled subscription.
		if err := o.cancelPendingBindings(ctx, subscriptionID); err != nil {
			return fmt.Errorf("cancel pending bindings: %w", err)
		}
		return nil
	}

	// Normal path: deprovision active bindings.
	return o.deprovisioning.Deprovision(ctx, subscriptionID)
}

// cancelPendingBindings marks any pending bindings for the subscription as
// failed. This handles the out-of-order delivery edge case where
// subscription.cancelled arrives before provisioning completes. The
// ProvisioningSaga will also check subscription status before provisioning,
// but this provides a belt-and-suspenders defense.
func (o *MultiSubOrchestrator) cancelPendingBindings(ctx context.Context, subscriptionID string) error {
	// Narrow read tx so the RLS GUC (carried on ctx) is applied to this
	// FORCE-RLS read.
	var allBindings []*aggregate.RemnawaveBinding
	err := o.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		var readErr error
		allBindings, readErr = o.bindings.GetBySubscriptionID(txCtx, subscriptionID)
		return readErr
	})
	if err != nil {
		return fmt.Errorf("get bindings by subscription: %w", err)
	}

	now := o.clock.Now()
	var cancelled int
	for _, b := range allBindings {
		if b.Status != aggregate.BindingPending {
			continue
		}
		if markErr := b.MarkFailed(cancelledPendingBindingReason, now); markErr != nil {
			o.logger.Warn("failed to mark pending binding as failed",
				slog.String("binding_id", b.ID),
				slog.String("subscription_id", subscriptionID),
				slog.Any("error", markErr),
			)
			continue
		}
		// Per-step tx: each pending-binding cancellation commits independently
		// with the RLS GUC set from the scope carried on ctx.
		updateErr := o.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
			return o.bindings.Update(txCtx, b)
		})
		if updateErr != nil {
			o.logger.Warn("failed to persist cancelled pending binding",
				slog.String("binding_id", b.ID),
				slog.String("subscription_id", subscriptionID),
				slog.Any("error", updateErr),
			)
			continue
		}
		cancelled++
		o.logger.Info("cancelled pending binding due to out-of-order delivery",
			slog.String("binding_id", b.ID),
			slog.String("subscription_id", subscriptionID),
		)
	}

	if cancelled == 0 {
		o.logger.Info("skipping duplicate subscription.cancelled event",
			slog.String("subscription_id", subscriptionID),
		)
	}
	return nil
}

// OnSubscriptionPaused is called when billing publishes subscription.paused.
// It delegates to BindingLifecycleService to disable all active bindings.
//
// Idempotency: BindingLifecycleService returns nil if no active bindings exist.
func (o *MultiSubOrchestrator) OnSubscriptionPaused(ctx context.Context, subscriptionID string) error {
	return o.lifecycle.DisableAllForSubscription(ctx, subscriptionID)
}

// OnSubscriptionResumed is called when billing publishes subscription.resumed.
// It delegates to BindingLifecycleService to re-enable all disabled bindings.
//
// Idempotency: BindingLifecycleService returns nil if no disabled bindings exist.
func (o *MultiSubOrchestrator) OnSubscriptionResumed(ctx context.Context, subscriptionID string) error {
	return o.lifecycle.EnableAllForSubscription(ctx, subscriptionID)
}
