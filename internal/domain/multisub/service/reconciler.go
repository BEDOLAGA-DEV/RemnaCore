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

// AbandonedClaimThreshold is the duration after which a binding stuck in
// 'reconciling' status is considered abandoned and reset to 'failed'.
const AbandonedClaimThreshold = 10 * time.Minute

// BindingReconciler periodically scans for failed bindings that still reference
// a Remnawave user (ghost users) and attempts to delete them. This serves as a
// safety net for compensation failures in ProvisioningSaga.
//
// The reconciler claims bindings by transitioning their status from 'failed' to
// 'reconciling' inside the same transaction that fetches them. This prevents
// TOCTOU races where concurrent reconciler instances could grab the same
// bindings between the SELECT and subsequent processing.
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
// Three-phase approach with claim-via-status to prevent TOCTOU races:
//  1. Short transaction: SELECT FOR UPDATE SKIP LOCKED + claim by setting
//     status to 'reconciling'. Locks are released when the tx commits but the
//     status change prevents other instances from selecting the same rows.
//  2. Outside transaction: DELETE Remnawave users via HTTP (no locks held).
//  3. Per-binding update: clear remnawave_uuid on success, or release claim
//     back to 'failed' on failure so the binding can be retried.
func (r *BindingReconciler) reconcile(ctx context.Context) {
	// Reset abandoned claims from crashed/timed-out reconciler instances.
	r.resetAbandonedClaims(ctx)

	// Phase 1: Fetch failed bindings and claim them by setting status to 'reconciling'.
	now := time.Now()
	var bindings []*multisubagg.RemnawaveBinding
	err := r.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		bindings, err = r.bindings.GetFailedForReconciliation(txCtx, ReconcilerBatchLimit)
		if err != nil {
			return fmt.Errorf("get failed bindings: %w", err)
		}
		for _, b := range bindings {
			if err := b.ClaimForReconciliation(now); err != nil {
				return fmt.Errorf("claim binding %s: %w", b.ID, err)
			}
			if err := r.bindings.Update(txCtx, b); err != nil {
				return fmt.Errorf("persist claim for binding %s: %w", b.ID, err)
			}
		}
		return nil
	})
	if err != nil {
		r.logger.Error("reconciler: failed to fetch and claim bindings",
			slog.Any("error", err),
		)
		return
	}

	if len(bindings) == 0 {
		return
	}

	r.logger.Info("reconciler: claimed orphaned remnawave users for reconciliation",
		slog.Int("count", len(bindings)),
	)

	// Phase 2: Delete Remnawave users outside any transaction (no locks held).
	// Phase 3: Clear remnawave_uuid on success, or release claim on failure.
	for _, binding := range bindings {
		if err := r.gateway.DeleteUser(ctx, binding.RemnawaveUUID); err != nil {
			r.logger.Warn("reconciler: failed to delete orphaned remnawave user, releasing claim",
				slog.String("binding_id", binding.ID),
				slog.String("remnawave_uuid", binding.RemnawaveUUID),
				slog.Any("error", err),
			)
			r.releaseBinding(ctx, binding)
			continue
		}

		// Clear the remnawave_uuid and transition to failed (cleaned up, retryable).
		binding.RemnawaveUUID = ""
		binding.RemnawaveShortUUID = ""
		if err := binding.ReleaseFromReconciliation(time.Now()); err != nil {
			r.logger.Warn("reconciler: failed to transition binding after cleanup",
				slog.String("binding_id", binding.ID),
				slog.Any("error", err),
			)
			continue
		}
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

// resetAbandonedClaims recovers bindings that were claimed by a reconciler
// instance that crashed or timed out before completing processing.
func (r *BindingReconciler) resetAbandonedClaims(ctx context.Context) {
	count, err := r.bindings.ResetAbandonedReconciling(ctx, AbandonedClaimThreshold)
	if err != nil {
		r.logger.Warn("reconciler: failed to reset abandoned claims",
			slog.Any("error", err),
		)
		return
	}
	if count > 0 {
		r.logger.Info("reconciler: reset abandoned reconciling claims",
			slog.Int64("count", count),
		)
	}
}

// releaseBinding transitions a claimed binding back to 'failed' so it can be
// retried by a future reconciliation pass.
func (r *BindingReconciler) releaseBinding(ctx context.Context, binding *multisubagg.RemnawaveBinding) {
	if err := binding.ReleaseFromReconciliation(time.Now()); err != nil {
		r.logger.Error("reconciler: failed to release binding from reconciling state",
			slog.String("binding_id", binding.ID),
			slog.Any("error", err),
		)
		return
	}
	if err := r.bindings.Update(ctx, binding); err != nil {
		r.logger.Error("reconciler: failed to persist released binding",
			slog.String("binding_id", binding.ID),
			slog.Any("error", err),
		)
	}
}
