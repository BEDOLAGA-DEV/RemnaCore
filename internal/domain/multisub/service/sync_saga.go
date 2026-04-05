package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	multisubdomain "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// syncState is the JSON-serializable state checkpointed for a sync saga.
type syncState struct {
	BindingID string `json:"binding_id"`
	Phase     string `json:"phase"`
}

// SyncSaga handles the re-synchronisation of a single RemnawaveBinding with
// the Remnawave panel. It is called during periodic sync runs and when
// webhook events arrive. Progress is checkpointed to the SagaRepository.
type SyncSaga struct {
	bindings multisubdomain.BindingRepository
	gateway  multisubdomain.RemnawaveGateway
	publisher domainevent.Publisher
	sagaRepo  multisubdomain.SagaRepository
	clock     clock.Clock
}

// NewSyncSaga creates a SyncSaga with its dependencies.
func NewSyncSaga(
	bindings multisubdomain.BindingRepository,
	gateway multisubdomain.RemnawaveGateway,
	publisher domainevent.Publisher,
	sagaRepo multisubdomain.SagaRepository,
	clk clock.Clock,
) *SyncSaga {
	return &SyncSaga{
		bindings:  bindings,
		gateway:   gateway,
		publisher: publisher,
		sagaRepo:  sagaRepo,
		clock:     clk,
	}
}

// syncTotalSteps is the number of logical steps in a sync saga:
// 1. fetch remote status, 2. reconcile + persist, 3. publish event.
const syncTotalSteps = 3

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

	// Create persistent saga instance for crash recovery.
	sagaInstance, err := s.sagaRepo.Create(ctx, &multisubdomain.SagaInstance{
		SagaType:      multisubdomain.SagaTypeSync,
		CorrelationID: bindingID,
		Status:        multisubdomain.SagaStatusRunning,
		CurrentStep:   0,
		TotalSteps:    syncTotalSteps,
		StateData:     []byte("{}"),
	})
	if err != nil {
		slog.Warn("failed to create sync saga instance, proceeding without persistence",
			slog.String("binding_id", bindingID),
			slog.Any("error", err),
		)
	}

	// Step 1: Fetch remote status
	status, err := s.gateway.GetUser(ctx, binding.RemnawaveUUID)
	if err != nil {
		syncFailedEvent := multisubdomain.NewBindingSyncFailedEvent(
			binding.ID,
			binding.SubscriptionID,
			err.Error(),
			s.clock.Now(),
		)
		if pubErr := s.publisher.Publish(ctx, syncFailedEvent); pubErr != nil {
			slog.Warn("failed to publish event",
				slog.String("event_type", string(syncFailedEvent.Type)),
				slog.Any("error", pubErr),
			)
		}
		s.failSyncSaga(ctx, sagaInstance, fmt.Sprintf("remnawave get user: %s", err.Error()))
		return fmt.Errorf("remnawave get user: %w", err)
	}
	s.checkpointSyncProgress(ctx, sagaInstance, 1, bindingID, "fetched_remote")

	// Step 2: Reconcile remote status with local binding.
	now := s.clock.Now()
	s.reconcileStatus(binding, status, now)

	// Update sync timestamp.
	binding.SyncedAt = &now

	if err := s.bindings.Update(ctx, binding); err != nil {
		s.failSyncSaga(ctx, sagaInstance, fmt.Sprintf("update binding: %s", err.Error()))
		return fmt.Errorf("update binding: %w", err)
	}
	s.checkpointSyncProgress(ctx, sagaInstance, 2, bindingID, "reconciled")

	// Step 3: Publish sync completed event.
	syncCompletedEvent := multisubdomain.NewBindingSyncCompletedEvent(
		binding.ID,
		binding.SubscriptionID,
		s.clock.Now(),
	)
	if err := s.publisher.Publish(ctx, syncCompletedEvent); err != nil {
		slog.Warn("failed to publish event",
			slog.String("event_type", string(syncCompletedEvent.Type)),
			slog.Any("error", err),
		)
	}

	s.completeSyncSaga(ctx, sagaInstance)

	return nil
}

// reconcileStatus updates the binding status based on the Remnawave user state.
func (s *SyncSaga) reconcileStatus(binding *aggregate.RemnawaveBinding, status *multisubdomain.RemnawaveUserStatus, now time.Time) {
	switch {
	case status.Expired:
		if err := binding.Disable(now); err != nil {
			slog.Warn("sync reconcile: disable transition failed",
				slog.String("binding_id", binding.ID),
				slog.Any("error", err),
			)
		}
	case !status.Enabled && binding.Status == aggregate.BindingActive:
		if err := binding.Disable(now); err != nil {
			slog.Warn("sync reconcile: disable transition failed",
				slog.String("binding_id", binding.ID),
				slog.Any("error", err),
			)
		}
	case status.Enabled && binding.Status == aggregate.BindingDisabled:
		if err := binding.Enable(now); err != nil {
			slog.Warn("sync reconcile: enable transition failed",
				slog.String("binding_id", binding.ID),
				slog.Any("error", err),
			)
		}
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
		if err := binding.Disable(now); err != nil {
			slog.Warn("webhook: disable transition failed",
				slog.String("binding_id", binding.ID),
				slog.Any("error", err),
			)
		}
	case multisubdomain.EventBindingSyncFailed:
		if err := binding.MarkFailed("webhook: sync failed", now); err != nil {
			slog.Warn("webhook: mark failed transition failed",
				slog.String("binding_id", binding.ID),
				slog.Any("error", err),
			)
		}
	default:
		// Unknown event type — update sync timestamp only.
	}

	binding.SyncedAt = &now

	if err := s.bindings.Update(ctx, binding); err != nil {
		return fmt.Errorf("update binding: %w", err)
	}

	webhookEvent := domainevent.NewAt(domainEventType, multisubdomain.BindingWebhookPayload{
		BindingID:      binding.ID,
		SubscriptionID: binding.SubscriptionID,
		RemnawaveUUID:  binding.RemnawaveUUID,
	}, s.clock.Now())
	if err := s.publisher.Publish(ctx, webhookEvent); err != nil {
		slog.Warn("failed to publish event",
			slog.String("event_type", string(webhookEvent.Type)),
			slog.Any("error", err),
		)
	}

	return nil
}

// checkpointSyncProgress persists the current sync saga step.
func (s *SyncSaga) checkpointSyncProgress(ctx context.Context, saga *multisubdomain.SagaInstance, step int, bindingID, phase string) {
	if saga == nil {
		return
	}
	stateData, err := json.Marshal(syncState{BindingID: bindingID, Phase: phase})
	if err != nil {
		slog.Warn("failed to marshal sync saga state",
			slog.String("saga_id", saga.ID),
			slog.Any("error", err),
		)
		return
	}
	if err := s.sagaRepo.UpdateProgress(ctx, saga.ID, step, stateData); err != nil {
		slog.Warn("failed to checkpoint sync saga progress",
			slog.String("saga_id", saga.ID),
			slog.Int("step", step),
			slog.Any("error", err),
		)
	}
}

// completeSyncSaga marks the sync saga as completed.
func (s *SyncSaga) completeSyncSaga(ctx context.Context, saga *multisubdomain.SagaInstance) {
	if saga == nil {
		return
	}
	if err := s.sagaRepo.Complete(ctx, saga.ID); err != nil {
		slog.Warn("failed to mark sync saga as completed",
			slog.String("saga_id", saga.ID),
			slog.Any("error", err),
		)
	}
}

// failSyncSaga marks the sync saga as failed.
func (s *SyncSaga) failSyncSaga(ctx context.Context, saga *multisubdomain.SagaInstance, errMsg string) {
	if saga == nil {
		return
	}
	if err := s.sagaRepo.Fail(ctx, saga.ID, errMsg); err != nil {
		slog.Warn("failed to mark sync saga as failed",
			slog.String("saga_id", saga.ID),
			slog.Any("error", err),
		)
	}
}
