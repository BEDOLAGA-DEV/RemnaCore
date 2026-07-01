// NOTE: This file uses encoding/json for saga state serialization (checkpoint
// persistence). This is an intentional infrastructure concern within the domain
// service -- saga state is inherently persistence-aware. The hook dispatch
// protocol (hookdispatch/sdk) has been fully extracted to the wiring layer.
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
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
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
	bindings  multisubdomain.BindingRepository
	gateway   multisubdomain.RemnawaveGateway
	publisher domainevent.Publisher
	sagaRepo  multisubdomain.SagaRepository
	txRunner  txmanager.Runner
	clock     clock.Clock
	logger    *slog.Logger
}

// NewDeprovisioningSaga creates a DeprovisioningSaga with its dependencies.
func NewDeprovisioningSaga(
	bindings multisubdomain.BindingRepository,
	gateway multisubdomain.RemnawaveGateway,
	publisher domainevent.Publisher,
	sagaRepo multisubdomain.SagaRepository,
	txRunner txmanager.Runner,
	clk clock.Clock,
	logger *slog.Logger,
) *DeprovisioningSaga {
	return &DeprovisioningSaga{
		bindings:  bindings,
		gateway:   gateway,
		publisher: publisher,
		sagaRepo:  sagaRepo,
		txRunner:  txRunner,
		clock:     clk,
		logger:    logger,
	}
}

// Deprovision removes all Remnawave users for a subscription. It fetches every
// active binding for the given subscription and, for each one, attempts to
// delete the Remnawave user, mark the binding as deprovisioned, and publish an
// event. Individual Remnawave failures are recorded on the binding but do not
// abort the whole operation.
func (s *DeprovisioningSaga) Deprovision(ctx context.Context, subscriptionID string) error {
	// Narrow read tx so the RLS GUC (carried on ctx) is applied to this
	// FORCE-RLS read; the tx commits before the per-binding loop begins so the
	// Remnawave HTTP calls below run outside any open transaction.
	var bindings []*aggregate.RemnawaveBinding
	err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		var readErr error
		bindings, readErr = s.bindings.GetActiveBySubscriptionID(txCtx, subscriptionID)
		return readErr
	})
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
		StateData:     []byte(emptySagaStateJSON),
	})
	if err != nil {
		s.logger.Warn("failed to create saga instance, proceeding without persistence",
			slog.String("subscription_id", subscriptionID),
			slog.Any("error", err),
		)
	}

	var deprovisionedIDs []string

	for i, binding := range bindings {
		s.deprovisionOne(ctx, binding)
		deprovisionedIDs = append(deprovisionedIDs, binding.ID)
		if sagaInstance != nil {
			stateData, err := json.Marshal(deprovisioningState{DeprovisionedBindingIDs: deprovisionedIDs})
			if err != nil {
				s.logger.Warn("failed to marshal deprovisioning saga state",
					slog.String("saga_id", sagaInstance.ID),
					slog.Any("error", err),
				)
			} else {
				checkpointSaga(ctx, s.sagaRepo, sagaInstance, i+1, stateData, s.logger)
			}
		}
	}

	completeSaga(ctx, s.sagaRepo, sagaInstance, s.logger)

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
				s.logger.Warn("failed to transition binding to failed",
					slog.String("binding_id", binding.ID),
					slog.Any("error", failErr),
				)
			}
			// Per-step tx: persist the failure status independently and set the
			// RLS GUC from the scope carried on ctx (gateway.DeleteUser above ran
			// outside any tx).
			updateErr := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
				return s.bindings.Update(txCtx, binding)
			})
			if updateErr != nil {
				s.logger.Warn("failed to update binding after remnawave delete failure",
					slog.String("binding_id", binding.ID),
					slog.Any("error", updateErr),
				)
			}
			return
		}
	}

	// 2. Mark binding as deprovisioned, persist update and publish events
	// atomically inside a transaction to prevent event loss.
	if err := binding.Deprovision(now); err != nil {
		s.logger.Warn("failed to transition binding to deprovisioned",
			slog.String("binding_id", binding.ID),
			slog.Any("error", err),
		)
		return
	}
	err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.bindings.Update(txCtx, binding); err != nil {
			return fmt.Errorf("update binding: %w", err)
		}
		return domainevent.PublishAll(txCtx, s.publisher, binding)
	})
	if err != nil {
		s.logger.Warn("failed to persist deprovisioned binding",
			slog.String("binding_id", binding.ID),
			slog.Any("error", err),
		)
	}
}
