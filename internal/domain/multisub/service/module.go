package service

import (
	"context"
	"fmt"
	"log/slog"

	multisubdomain "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
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
	logger         *slog.Logger
}

// NewMultiSubOrchestrator creates a MultiSubOrchestrator with its saga
// dependencies.
func NewMultiSubOrchestrator(
	provisioning *ProvisioningSaga,
	deprovisioning *DeprovisioningSaga,
	syncService *SyncService,
	lifecycle *BindingLifecycleService,
	bindings multisubdomain.BindingRepository,
	publisher domainevent.Publisher,
	logger *slog.Logger,
) *MultiSubOrchestrator {
	return &MultiSubOrchestrator{
		provisioning:   provisioning,
		deprovisioning: deprovisioning,
		syncService:    syncService,
		lifecycle:      lifecycle,
		bindings:       bindings,
		publisher:      publisher,
		logger:         logger,
	}
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

// OnSubscriptionCancelled is called when billing publishes
// subscription.cancelled. It deprovisions all Remnawave bindings for the
// subscription (best-effort).
//
// Idempotency: if no active bindings remain, the event is treated as a
// duplicate and deprovisioning is skipped.
func (o *MultiSubOrchestrator) OnSubscriptionCancelled(ctx context.Context, subscriptionID string) error {
	existing, err := o.bindings.GetActiveBySubscriptionID(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("check existing bindings: %w", err)
	}
	if len(existing) == 0 {
		o.logger.Info("skipping duplicate subscription.cancelled event",
			slog.String("subscription_id", subscriptionID),
		)
		return nil
	}

	return o.deprovisioning.Deprovision(ctx, subscriptionID)
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
