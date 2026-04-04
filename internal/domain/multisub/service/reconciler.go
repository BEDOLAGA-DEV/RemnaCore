package service

import (
	"context"
	"log/slog"
	"time"

	multisubdomain "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
)

// ReconcilerInterval is the period between reconciliation runs that clean up
// orphaned Remnawave users from failed bindings.
const ReconcilerInterval = 1 * time.Hour

// BindingReconciler periodically scans for failed bindings that still reference
// a Remnawave user (ghost users) and attempts to delete them. This serves as a
// safety net for compensation failures in ProvisioningSaga.
type BindingReconciler struct {
	bindings multisubdomain.BindingRepository
	gateway  multisubdomain.RemnawaveGateway
	logger   *slog.Logger
}

// NewBindingReconciler creates a BindingReconciler with its dependencies.
func NewBindingReconciler(
	bindings multisubdomain.BindingRepository,
	gateway multisubdomain.RemnawaveGateway,
	logger *slog.Logger,
) *BindingReconciler {
	return &BindingReconciler{
		bindings: bindings,
		gateway:  gateway,
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

// reconcile finds all failed bindings that still have a remnawave_uuid and
// attempts to clean up the orphaned Remnawave user for each.
func (r *BindingReconciler) reconcile(ctx context.Context) {
	bindings, err := r.bindings.GetFailedWithRemnawaveUUID(ctx)
	if err != nil {
		r.logger.Error("reconciler: failed to query failed bindings",
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

	for _, binding := range bindings {
		if err := r.gateway.DeleteUser(ctx, binding.RemnawaveUUID); err != nil {
			r.logger.Warn("reconciler: failed to delete orphaned remnawave user",
				slog.String("binding_id", binding.ID),
				slog.String("remnawave_uuid", binding.RemnawaveUUID),
				slog.Any("error", err),
			)
			continue
		}

		// Clear the remnawave_uuid so this binding is not retried on the next
		// reconciliation pass.
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
			slog.String("remnawave_uuid", binding.RemnawaveUUID),
		)
	}
}
