package service

import (
	"context"
	"fmt"
	"time"

	multisubdomain "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// SyncSaga handles the re-synchronisation of a single RemnawaveBinding with
// the Remnawave panel. It is called during periodic sync runs and when
// webhook events arrive.
type SyncSaga struct {
	bindings  multisubdomain.BindingRepository
	gateway   multisubdomain.RemnawaveGateway
	publisher domainevent.Publisher
}

// NewSyncSaga creates a SyncSaga with its dependencies.
func NewSyncSaga(
	bindings multisubdomain.BindingRepository,
	gateway multisubdomain.RemnawaveGateway,
	publisher domainevent.Publisher,
) *SyncSaga {
	return &SyncSaga{
		bindings:  bindings,
		gateway:   gateway,
		publisher: publisher,
	}
}

// SyncBinding re-syncs a single binding with its Remnawave counterpart. It
// fetches the binding from the local DB, checks the current status in
// Remnawave, and updates the binding if the remote state has drifted. The
// operation is idempotent — running it twice produces the same result.
func (s *SyncSaga) SyncBinding(ctx context.Context, bindingID string) error {
	binding, err := s.bindings.GetByID(ctx, bindingID)
	if err != nil {
		return fmt.Errorf("get binding: %w", err)
	}

	// Only sync bindings that have a Remnawave UUID (i.e. have been provisioned).
	if binding.RemnawaveUUID == "" {
		return nil
	}

	status, err := s.gateway.GetUser(ctx, binding.RemnawaveUUID)
	if err != nil {
		_ = s.publisher.Publish(ctx, multisubdomain.NewBindingSyncFailedEvent(
			binding.ID,
			binding.SubscriptionID,
			err.Error(),
		))
		return fmt.Errorf("remnawave get user: %w", err)
	}

	// Reconcile remote status with local binding.
	s.reconcileStatus(binding, status)

	// Update sync timestamp.
	now := time.Now()
	binding.SyncedAt = &now

	if err := s.bindings.Update(ctx, binding); err != nil {
		return fmt.Errorf("update binding: %w", err)
	}

	_ = s.publisher.Publish(ctx, multisubdomain.NewBindingSyncCompletedEvent(
		binding.ID,
		binding.SubscriptionID,
	))

	return nil
}

// reconcileStatus updates the binding status based on the Remnawave user state.
func (s *SyncSaga) reconcileStatus(binding *aggregate.RemnawaveBinding, status *multisubdomain.RemnawaveUserStatus) {
	switch {
	case status.Expired:
		binding.Disable()
	case !status.Enabled && binding.Status == aggregate.BindingActive:
		binding.Disable()
	case status.Enabled && binding.Status == aggregate.BindingDisabled:
		binding.Enable()
	}
}

// HandleWebhookEvent processes a translated Remnawave webhook event. It looks
// up the binding by Remnawave UUID, updates its status based on the domain
// event type, and publishes the corresponding domain event.
func (s *SyncSaga) HandleWebhookEvent(ctx context.Context, remnawaveUUID string, domainEventType domainevent.EventType) error {
	binding, err := s.bindings.GetByRemnawaveUUID(ctx, remnawaveUUID)
	if err != nil {
		return fmt.Errorf("find binding by remnawave uuid: %w", err)
	}

	switch domainEventType {
	case multisubdomain.EventBindingTrafficExceeded:
		binding.Disable()
	case multisubdomain.EventBindingSyncFailed:
		binding.MarkFailed("webhook: sync failed")
	default:
		// Unknown event type — update sync timestamp only.
	}

	now := time.Now()
	binding.SyncedAt = &now

	if err := s.bindings.Update(ctx, binding); err != nil {
		return fmt.Errorf("update binding: %w", err)
	}

	_ = s.publisher.Publish(ctx, domainevent.New(domainEventType, map[string]any{
		"binding_id":      binding.ID,
		"subscription_id": binding.SubscriptionID,
		"remnawave_uuid":  binding.RemnawaveUUID,
	}))

	return nil
}
