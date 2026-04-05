package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	multisubdomain "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	multisubagg "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// ReconcilerInterval is the period between reconciliation runs that clean up
// orphaned Remnawave users from failed bindings.
const ReconcilerInterval = 1 * time.Hour

// ReconcilerBatchLimit is the maximum number of failed bindings to process in
// a single reconciliation pass.
const ReconcilerBatchLimit = 50

// BindingReconciler periodically scans for failed bindings that still reference
// a Remnawave user (ghost users) and attempts to delete them. This serves as a
// safety net for compensation failures in ProvisioningSaga.
//
// The reconciler uses SELECT ... FOR UPDATE SKIP LOCKED to prevent concurrent
// instances on multiple pods from processing the same bindings.
type BindingReconciler struct {
	bindings multisubdomain.BindingRepository
	gateway  multisubdomain.RemnawaveGateway
	txRunner txmanager.Runner
	logger   *slog.Logger
}

// NewBindingReconciler creates a BindingReconciler with its dependencies.
func NewBindingReconciler(
	bindings multisubdomain.BindingRepository,
	gateway multisubdomain.RemnawaveGateway,
	txRunner txmanager.Runner,
	logger *slog.Logger,
) *BindingReconciler {
	return &BindingReconciler{
		bindings: bindings,
		gateway:  gateway,
		txRunner: txRunner,
		logger:   logger,
	}
}

// Run starts a blocking loop that reconciles orphaned Remnawave users at
// ReconcilerInterval. It returns when the context is cancelled.
func (r *BindingReconciler) Run(ctx context.Context) {
	ticker := time.NewTicker(ReconcilerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.reconcile(ctx)
		}
	}
}

// ReconcileOnce runs a single reconciliation pass. It is the exported entry
// point used by tests and can also be invoked manually from admin tooling.
func (r *BindingReconciler) ReconcileOnce(ctx context.Context) {
	r.reconcile(ctx)
}

// reconcile finds failed bindings that still have a remnawave_uuid and attempts
// to clean up the orphaned Remnawave user for each.
//
// Two-phase approach to avoid holding row locks during external HTTP calls:
//  1. Short transaction: SELECT ... FOR UPDATE SKIP LOCKED to claim bindings.
//  2. Outside transaction: DELETE Remnawave users via HTTP (no locks held).
//  3. Per-binding short transaction: clear remnawave_uuid on success.
func (r *BindingReconciler) reconcile(ctx context.Context) {
	// Phase 1: Fetch and lock bindings in a short transaction.
	var bindings []*multisubagg.RemnawaveBinding
	err := r.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		bindings, err = r.bindings.GetFailedForReconciliation(txCtx, ReconcilerBatchLimit)
		if err != nil {
			return fmt.Errorf("get failed bindings: %w", err)
		}
		return nil
	})
	if err != nil {
		r.logger.Error("reconciler: failed to fetch bindings",
			slog.Any("error", err),
		)
		return
	}

	if len(bindings) == 0 {
		return
	}

	r.logger.Info("reconciler: found orphaned remnawave users",
		slog.Int("count", len(bindings)),
	)

	// Phase 2: Delete Remnawave users outside any transaction (no locks held).
	// Phase 3: Clear remnawave_uuid per-binding in short individual updates.
	for _, binding := range bindings {
		if err := r.gateway.DeleteUser(ctx, binding.RemnawaveUUID); err != nil {
			r.logger.Warn("reconciler: failed to delete orphaned remnawave user",
				slog.String("binding_id", binding.ID),
				slog.String("remnawave_uuid", binding.RemnawaveUUID),
				slog.Any("error", err),
			)
			continue
		}

		// Clear the remnawave_uuid so this binding is not retried.
		binding.RemnawaveUUID = ""
		binding.RemnawaveShortUUID = ""
		if err := r.bindings.Update(ctx, binding); err != nil {
			r.logger.Warn("reconciler: deleted remnawave user but failed to update binding",
				slog.String("binding_id", binding.ID),
				slog.Any("error", err),
			)
			continue
		}

		r.logger.Info("reconciler: cleaned up orphaned remnawave user",
			slog.String("binding_id", binding.ID),
		)
	}
}
