package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	multisubdomain "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// SyncSaga handles the re-synchronisation of a single RemnawaveBinding with
// the Remnawave panel. It is called during periodic sync runs and when
// webhook events arrive.
type SyncSaga struct {
	bindings  multisubdomain.BindingRepository
	gateway   multisubdomain.RemnawaveGateway
	publisher domainevent.Publisher
	clock     clock.Clock
}

// NewSyncSaga creates a SyncSaga with its dependencies.
func NewSyncSaga(
	bindings multisubdomain.BindingRepository,
	gateway multisubdomain.RemnawaveGateway,
	publisher domainevent.Publisher,
	clk clock.Clock,
) *SyncSaga {
	return &SyncSaga{
		bindings:  bindings,
		gateway:   gateway,
		publisher: publisher,
		clock:     clk,
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
		syncFailedEvent := multisubdomain.NewBindingSyncFailedEvent(
			binding.ID,
			binding.SubscriptionID,
			err.Error(),
		)
		if pubErr := s.publisher.Publish(ctx, syncFailedEvent); pubErr != nil {
			slog.Warn("failed to publish event",
				slog.String("event_type", string(syncFailedEvent.Type)),
				slog.String("error", pubErr.Error()),
			)
		}
		return fmt.Errorf("remnawave get user: %w", err)
	}

	// Reconcile remote status with local binding.
	now := s.clock.Now()
	s.reconcileStatus(binding, status, now)

	// Update sync timestamp.
	binding.SyncedAt = &now

	if err := s.bindings.Update(ctx, binding); err != nil {
		return fmt.Errorf("update binding: %w", err)
	}

	syncCompletedEvent := multisubdomain.NewBindingSyncCompletedEvent(
		binding.ID,
		binding.SubscriptionID,
	)
	if err := s.publisher.Publish(ctx, syncCompletedEvent); err != nil {
		slog.Warn("failed to publish event",
			slog.String("event_type", string(syncCompletedEvent.Type)),
			slog.String("error", err.Error()),
		)
	}

	return nil
}

// reconcileStatus updates the binding status based on the Remnawave user state.
func (s *SyncSaga) reconcileStatus(binding *aggregate.RemnawaveBinding, status *multisubdomain.RemnawaveUserStatus, now time.Time) {
	switch {
	case status.Expired:
		binding.Disable(now)
	case !status.Enabled && binding.Status == aggregate.BindingActive:
		binding.Disable(now)
	case status.Enabled && binding.Status == aggregate.BindingDisabled:
		binding.Enable(now)
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

	now := s.clock.Now()
	switch domainEventType {
	case multisubdomain.EventBindingTrafficExceeded:
		binding.Disable(now)
	case multisubdomain.EventBindingSyncFailed:
		binding.MarkFailed("webhook: sync failed", now)
	default:
		// Unknown event type — update sync timestamp only.
	}

	binding.SyncedAt = &now

	if err := s.bindings.Update(ctx, binding); err != nil {
		return fmt.Errorf("update binding: %w", err)
	}

	webhookEvent := domainevent.New(domainEventType, multisubdomain.BindingWebhookPayload{
		BindingID:      binding.ID,
		SubscriptionID: binding.SubscriptionID,
		RemnawaveUUID:  binding.RemnawaveUUID,
	})
	if err := s.publisher.Publish(ctx, webhookEvent); err != nil {
		slog.Warn("failed to publish event",
			slog.String("event_type", string(webhookEvent.Type)),
			slog.String("error", err.Error()),
		)
	}

	return nil
}
