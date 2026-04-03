package service

import (
	"context"
	"fmt"

	billingaggregate "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	multisubdomain "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	multisubagg "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
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
)

// MultiSubOrchestrator is the facade that coordinates billing lifecycle events
// with the multisub domain. All billing event handlers route through this
// struct so that upstream callers do not need to know about individual sagas.
type MultiSubOrchestrator struct {
	provisioning   *ProvisioningSaga
	deprovisioning *DeprovisioningSaga
	syncService    *SyncService
	bindings       multisubdomain.BindingRepository
	gateway        multisubdomain.RemnawaveGateway
	publisher      domainevent.Publisher
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
) *MultiSubOrchestrator {
	return &MultiSubOrchestrator{
		provisioning:   provisioning,
		deprovisioning: deprovisioning,
		syncService:    syncService,
		bindings:       bindings,
		gateway:        gateway,
		publisher:      publisher,
	}
}

// OnSubscriptionActivated is called when billing publishes
// subscription.activated. It provisions all needed Remnawave bindings.
func (o *MultiSubOrchestrator) OnSubscriptionActivated(
	ctx context.Context,
	subscriptionID string,
	platformUserID string,
	plan *billingaggregate.Plan,
	addonIDs []string,
	familyMemberIDs []string,
) error {
	_, err := o.provisioning.Provision(ctx, ProvisionRequest{
		SubscriptionID:  subscriptionID,
		PlatformUserID:  platformUserID,
		Plan:            plan,
		AddonIDs:        addonIDs,
		FamilyMemberIDs: familyMemberIDs,
	})
	return err
}

// OnSubscriptionCancelled is called when billing publishes
// subscription.cancelled. It deprovisions all Remnawave bindings for the
// subscription (best-effort).
func (o *MultiSubOrchestrator) OnSubscriptionCancelled(ctx context.Context, subscriptionID string) error {
	return o.deprovisioning.Deprovision(ctx, subscriptionID)
}

// OnSubscriptionPaused is called when billing publishes subscription.paused.
// It disables all active bindings in Remnawave.
func (o *MultiSubOrchestrator) OnSubscriptionPaused(ctx context.Context, subscriptionID string) error {
	bindings, err := o.bindings.GetActiveBySubscriptionID(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("get active bindings: %w", err)
	}

	for _, binding := range bindings {
		if binding.RemnawaveUUID == "" {
			continue
		}
		if err := o.gateway.DisableUser(ctx, binding.RemnawaveUUID); err != nil {
			binding.MarkFailed(fmt.Sprintf("remnawave disable: %s", err.Error()))
			_ = o.bindings.Update(ctx, binding)
			continue
		}
		binding.Disable()
		_ = o.bindings.Update(ctx, binding)
	}

	return nil
}

// OnSubscriptionResumed is called when billing publishes subscription.resumed.
// It enables all disabled bindings in Remnawave.
func (o *MultiSubOrchestrator) OnSubscriptionResumed(ctx context.Context, subscriptionID string) error {
	bindings, err := o.bindings.GetBySubscriptionID(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("get bindings: %w", err)
	}

	for _, binding := range bindings {
		if binding.Status != multisubagg.BindingDisabled {
			continue
		}
		if binding.RemnawaveUUID == "" {
			continue
		}
		if err := o.gateway.EnableUser(ctx, binding.RemnawaveUUID); err != nil {
			binding.MarkFailed(fmt.Sprintf("remnawave enable: %s", err.Error()))
			_ = o.bindings.Update(ctx, binding)
			continue
		}
		binding.Enable()
		_ = o.bindings.Update(ctx, binding)
	}

	return nil
}
