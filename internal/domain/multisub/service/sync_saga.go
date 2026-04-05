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
	bindings  multisubdomain.BindingRepository
	gateway   multisubdomain.RemnawaveGateway
	publisher domainevent.Publisher
	sagaRepo  multisubdomain.SagaRepository
	clock     clock.Clock
	logger    *slog.Logger
}

// NewSyncSaga creates a SyncSaga with its dependencies.
func NewSyncSaga(
	bindings multisubdomain.BindingRepository,
	gateway multisubdomain.RemnawaveGateway,
	publisher domainevent.Publisher,
	sagaRepo multisubdomain.SagaRepository,
	clk clock.Clock,
	logger *slog.Logger,
) *SyncSaga {
	return &SyncSaga{
		bindings:  bindings,
		gateway:   gateway,
		publisher: publisher,
		sagaRepo:  sagaRepo,
		clock:     clk,
		logger:    logger,
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
		s.logger.Warn("failed to create sync saga instance, proceeding without persistence",
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
			s.logger.Warn("failed to publish event",
				slog.String("event_type", string(syncFailedEvent.Type)),
				slog.Any("error", pubErr),
			)
		}
		failSaga(ctx, s.sagaRepo, sagaInstance, fmt.Sprintf("remnawave get user: %s", err.Error()), s.logger)
		return fmt.Errorf("remnawave get user: %w", err)
	}
	if sagaInstance != nil {
		stateData, err := json.Marshal(syncState{BindingID: bindingID, Phase: "fetched_remote"})
		if err != nil {
			s.logger.Warn("failed to marshal sync saga state",
				slog.String("saga_id", sagaInstance.ID),
				slog.Any("error", err),
			)
		} else {
			checkpointSaga(ctx, s.sagaRepo, sagaInstance, 1, stateData, s.logger)
		}
	}

	// Step 2: Reconcile remote status with local binding.
	now := s.clock.Now()
	s.reconcileStatus(binding, status, now)

	// Update sync timestamp.
	binding.SyncedAt = &now

	if err := s.bindings.Update(ctx, binding); err != nil {
		failSaga(ctx, s.sagaRepo, sagaInstance, fmt.Sprintf("update binding: %s", err.Error()), s.logger)
		return fmt.Errorf("update binding: %w", err)
	}
	if sagaInstance != nil {
		stateData, err := json.Marshal(syncState{BindingID: bindingID, Phase: "reconciled"})
		if err != nil {
			s.logger.Warn("failed to marshal sync saga state",
				slog.String("saga_id", sagaInstance.ID),
				slog.Any("error", err),
			)
		} else {
			checkpointSaga(ctx, s.sagaRepo, sagaInstance, 2, stateData, s.logger)
		}
	}

	// Step 3: Publish sync completed event.
	syncCompletedEvent := multisubdomain.NewBindingSyncCompletedEvent(
		binding.ID,
		binding.SubscriptionID,
		s.clock.Now(),
	)
	if err := s.publisher.Publish(ctx, syncCompletedEvent); err != nil {
		s.logger.Warn("failed to publish event",
			slog.String("event_type", string(syncCompletedEvent.Type)),
			slog.Any("error", err),
		)
	}

	completeSaga(ctx, s.sagaRepo, sagaInstance, s.logger)

	return nil
}

// reconcileStatus updates the binding status based on the Remnawave user state.
func (s *SyncSaga) reconcileStatus(binding *aggregate.RemnawaveBinding, status *multisubdomain.RemnawaveUserStatus, now time.Time) {
	switch {
	case status.Expired:
		if err := binding.Disable(now); err != nil {
			s.logger.Warn("sync reconcile: disable transition failed",
				slog.String("binding_id", binding.ID),
				slog.Any("error", err),
			)
		}
	case !status.Enabled && binding.Status == aggregate.BindingActive:
		if err := binding.Disable(now); err != nil {
			s.logger.Warn("sync reconcile: disable transition failed",
				slog.String("binding_id", binding.ID),
				slog.Any("error", err),
			)
		}
	case status.Enabled && binding.Status == aggregate.BindingDisabled:
		if err := binding.Enable(now); err != nil {
			s.logger.Warn("sync reconcile: enable transition failed",
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
			s.logger.Warn("webhook: disable transition failed",
				slog.String("binding_id", binding.ID),
				slog.Any("error", err),
			)
		}
	case multisubdomain.EventBindingSyncFailed:
		if err := binding.MarkFailed("webhook: sync failed", now); err != nil {
			s.logger.Warn("webhook: mark failed transition failed",
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
		s.logger.Warn("failed to publish event",
			slog.String("event_type", string(webhookEvent.Type)),
			slog.Any("error", err),
		)
	}

	return nil
}

