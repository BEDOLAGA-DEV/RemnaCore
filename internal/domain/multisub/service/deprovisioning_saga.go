package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	multisubdomain "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// deprovisioningState is the JSON-serializable state checkpointed after each
// successful binding deprovisioning step.
type deprovisioningState struct {
	DeprovisionedBindingIDs []string `json:"deprovisioned_binding_ids"`
}

// DeprovisioningSaga orchestrates the removal of Remnawave VPN users for a
// subscription. It is best-effort: if deleting a single Remnawave user fails,
// the saga logs the failure, marks the binding as failed, and continues with
// the remaining bindings. Progress is checkpointed after each step.
type DeprovisioningSaga struct {
	bindings multisubdomain.BindingRepository
	gateway  multisubdomain.RemnawaveGateway
	publisher domainevent.Publisher
	sagaRepo  multisubdomain.SagaRepository
	clock     clock.Clock
}

// NewDeprovisioningSaga creates a DeprovisioningSaga with its dependencies.
func NewDeprovisioningSaga(
	bindings multisubdomain.BindingRepository,
	gateway multisubdomain.RemnawaveGateway,
	publisher domainevent.Publisher,
	sagaRepo multisubdomain.SagaRepository,
	clk clock.Clock,
) *DeprovisioningSaga {
	return &DeprovisioningSaga{
		bindings:  bindings,
		gateway:   gateway,
		publisher: publisher,
		sagaRepo:  sagaRepo,
		clock:     clk,
	}
}

// Deprovision removes all Remnawave users for a subscription. It fetches every
// active binding for the given subscription and, for each one, attempts to
// delete the Remnawave user, mark the binding as deprovisioned, and publish an
// event. Individual Remnawave failures are recorded on the binding but do not
// abort the whole operation.
func (s *DeprovisioningSaga) Deprovision(ctx context.Context, subscriptionID string) error {
	bindings, err := s.bindings.GetActiveBySubscriptionID(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("get active bindings: %w", err)
	}

	// Create persistent saga instance for crash recovery.
	sagaInstance, err := s.sagaRepo.Create(ctx, &multisubdomain.SagaInstance{
		SagaType:      multisubdomain.SagaTypeDeprovisioning,
		CorrelationID: subscriptionID,
		Status:        multisubdomain.SagaStatusRunning,
		CurrentStep:   0,
		TotalSteps:    len(bindings),
		StateData:     []byte("{}"),
	})
	if err != nil {
		slog.Warn("failed to create saga instance, proceeding without persistence",
			slog.String("subscription_id", subscriptionID),
			slog.Any("error", err),
		)
	}

	var deprovisionedIDs []string

	for i, binding := range bindings {
		s.deprovisionOne(ctx, binding)
		deprovisionedIDs = append(deprovisionedIDs, binding.ID)
		s.checkpointDeprovisionProgress(ctx, sagaInstance, i+1, deprovisionedIDs)
	}

	s.completeSaga(ctx, sagaInstance)

	return nil
}

// deprovisionOne handles a single binding. It never returns an error — failures
// are recorded on the binding itself so that the caller can continue with the
// next binding.
func (s *DeprovisioningSaga) deprovisionOne(ctx context.Context, binding *aggregate.RemnawaveBinding) {
	now := s.clock.Now()
	// 1. Delete user in Remnawave
	if binding.RemnawaveUUID != "" {
		if err := s.gateway.DeleteUser(ctx, binding.RemnawaveUUID); err != nil {
			// Mark binding as failed and persist — do not stop the saga.
			if failErr := binding.MarkFailed(fmt.Sprintf("remnawave delete: %s", err.Error()), now); failErr != nil {
				slog.Warn("failed to transition binding to failed",
					slog.String("binding_id", binding.ID),
					slog.Any("error", failErr),
				)
			}
			if updateErr := s.bindings.Update(ctx, binding); updateErr != nil {
				slog.Warn("failed to update binding after remnawave delete failure",
					slog.String("binding_id", binding.ID),
					slog.Any("error", updateErr),
				)
			}
			return
		}
	}

	// 2. Mark binding as deprovisioned
	if err := binding.Deprovision(now); err != nil {
		slog.Warn("failed to transition binding to deprovisioned",
			slog.String("binding_id", binding.ID),
			slog.Any("error", err),
		)
		return
	}
	if err := s.bindings.Update(ctx, binding); err != nil {
		slog.Warn("failed to update binding after deprovision",
			slog.String("binding_id", binding.ID),
			slog.Any("error", err),
		)
	}

	// 3. Publish binding's self-recorded events
	for _, evt := range binding.DomainEvents() {
		if err := s.publisher.Publish(ctx, evt); err != nil {
			slog.Warn("failed to publish binding event",
				slog.String("binding_id", binding.ID),
				slog.String("event_type", string(evt.Type)),
				slog.Any("error", err),
			)
		}
	}
}

// checkpointDeprovisionProgress persists the current deprovisioning step.
func (s *DeprovisioningSaga) checkpointDeprovisionProgress(ctx context.Context, saga *multisubdomain.SagaInstance, step int, deprovisionedIDs []string) {
	if saga == nil {
		return
	}
	stateData, err := json.Marshal(deprovisioningState{DeprovisionedBindingIDs: deprovisionedIDs})
	if err != nil {
		slog.Warn("failed to marshal deprovisioning saga state",
			slog.String("saga_id", saga.ID),
			slog.Any("error", err),
		)
		return
	}
	if err := s.sagaRepo.UpdateProgress(ctx, saga.ID, step, stateData); err != nil {
		slog.Warn("failed to checkpoint deprovisioning saga progress",
			slog.String("saga_id", saga.ID),
			slog.Int("step", step),
			slog.Any("error", err),
		)
	}
}

// completeSaga marks the saga as completed. Failures are logged but do not
// abort the operation.
func (s *DeprovisioningSaga) completeSaga(ctx context.Context, saga *multisubdomain.SagaInstance) {
	if saga == nil {
		return
	}
	if err := s.sagaRepo.Complete(ctx, saga.ID); err != nil {
		slog.Warn("failed to mark deprovisioning saga as completed",
			slog.String("saga_id", saga.ID),
			slog.Any("error", err),
		)
	}
}
