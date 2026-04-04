package service

import (
	"context"
	"fmt"
	"log/slog"

	multisubdomain "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	multisubagg "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"go.uber.org/fx"
)

// Module provides all multisub domain services to the Fx container.
var Module = fx.Module("multisub",
	fx.Provide(NewBindingCalculator),
	fx.Provide(NewProvisioningSaga),
	fx.Provide(NewDeprovisioningSaga),
	fx.Provide(NewSyncSaga),
	fx.Provide(NewSyncService),
	fx.Provide(NewMultiSubOrchestrator),
	fx.Provide(NewBindingReconciler),
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
	bindings       multisubdomain.BindingRepository
	gateway        multisubdomain.RemnawaveGateway
	publisher      domainevent.Publisher
	logger         *slog.Logger
	clock          clock.Clock
}

// NewMultiSubOrchestrator creates a MultiSubOrchestrator with its saga
// dependencies.
func NewMultiSubOrchestrator(
	provisioning *ProvisioningSaga,
	deprovisioning *DeprovisioningSaga,
	syncService *SyncService,
	bindings multisubdomain.BindingRepository,
	gateway multisubdomain.RemnawaveGateway,
	publisher domainevent.Publisher,
	logger *slog.Logger,
	clk clock.Clock,
) *MultiSubOrchestrator {
	return &MultiSubOrchestrator{
		provisioning:   provisioning,
		deprovisioning: deprovisioning,
		syncService:    syncService,
		bindings:       bindings,
		gateway:        gateway,
		publisher:      publisher,
		logger:         logger,
		clock:          clk,
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
// It disables all active bindings in Remnawave.
//
// Idempotency: if no active bindings exist (already paused or deprovisioned),
// the event is treated as a duplicate and the operation is skipped.
func (o *MultiSubOrchestrator) OnSubscriptionPaused(ctx context.Context, subscriptionID string) error {
	bindings, err := o.bindings.GetActiveBySubscriptionID(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("get active bindings: %w", err)
	}
	if len(bindings) == 0 {
		o.logger.Info("skipping duplicate subscription.paused event",
			slog.String("subscription_id", subscriptionID),
		)
		return nil
	}

	now := o.clock.Now()
	for _, binding := range bindings {
		if binding.RemnawaveUUID == "" {
			continue
		}
		if err := o.gateway.DisableUser(ctx, binding.RemnawaveUUID); err != nil {
			if failErr := binding.MarkFailed(fmt.Sprintf("remnawave disable: %s", err.Error()), now); failErr != nil {
				o.logger.Warn("failed to transition binding to failed",
					slog.String("binding_id", binding.ID),
					slog.Any("error", failErr),
				)
			}
			if updateErr := o.bindings.Update(ctx, binding); updateErr != nil {
				o.logger.Warn("failed to update binding status",
					slog.String("binding_id", binding.ID),
					slog.Any("error", updateErr),
				)
			}
			continue
		}
		if disableErr := binding.Disable(now); disableErr != nil {
			o.logger.Warn("failed to transition binding to disabled",
				slog.String("binding_id", binding.ID),
				slog.Any("error", disableErr),
			)
		}
		if err := o.bindings.Update(ctx, binding); err != nil {
			o.logger.Warn("failed to update binding status",
				slog.String("binding_id", binding.ID),
				slog.Any("error", err),
			)
		}
	}

	return nil
}

// OnSubscriptionResumed is called when billing publishes subscription.resumed.
// It enables all disabled bindings in Remnawave.
//
// Idempotency: if no disabled bindings exist (already resumed or never
// paused), the event is treated as a duplicate and the operation is skipped.
func (o *MultiSubOrchestrator) OnSubscriptionResumed(ctx context.Context, subscriptionID string) error {
	bindings, err := o.bindings.GetBySubscriptionID(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("get bindings: %w", err)
	}

	hasDisabled := false
	for _, binding := range bindings {
		if binding.Status == multisubagg.BindingDisabled {
			hasDisabled = true
			break
		}
	}
	if !hasDisabled {
		o.logger.Info("skipping duplicate subscription.resumed event",
			slog.String("subscription_id", subscriptionID),
		)
		return nil
	}

	now := o.clock.Now()
	for _, binding := range bindings {
		if binding.Status != multisubagg.BindingDisabled {
			continue
		}
		if binding.RemnawaveUUID == "" {
			continue
		}
		if err := o.gateway.EnableUser(ctx, binding.RemnawaveUUID); err != nil {
			if failErr := binding.MarkFailed(fmt.Sprintf("remnawave enable: %s", err.Error()), now); failErr != nil {
				o.logger.Warn("failed to transition binding to failed",
					slog.String("binding_id", binding.ID),
					slog.Any("error", failErr),
				)
			}
			if updateErr := o.bindings.Update(ctx, binding); updateErr != nil {
				o.logger.Warn("failed to update binding status",
					slog.String("binding_id", binding.ID),
					slog.Any("error", updateErr),
				)
			}
			continue
		}
		if enableErr := binding.Enable(now); enableErr != nil {
			o.logger.Warn("failed to transition binding to active",
				slog.String("binding_id", binding.ID),
				slog.Any("error", enableErr),
			)
		}
		if err := o.bindings.Update(ctx, binding); err != nil {
			o.logger.Warn("failed to update binding status",
				slog.String("binding_id", binding.ID),
				slog.Any("error", err),
			)
		}
	}

	return nil
}
