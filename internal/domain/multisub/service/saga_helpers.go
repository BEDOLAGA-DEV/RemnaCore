package service

import (
	"context"
	"log/slog"

	multisubdomain "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
)

// completeSaga marks a saga instance as completed. Best-effort: failures are
// logged but do not affect the calling operation.
func completeSaga(ctx context.Context, repo multisubdomain.SagaRepository, saga *multisubdomain.SagaInstance, logger *slog.Logger) {
	if saga == nil {
		return
	}
	if err := repo.Complete(ctx, saga.ID); err != nil {
		logger.Warn("failed to mark saga as completed",
			slog.String("saga_id", saga.ID),
			slog.Any("error", err),
		)
	}
}

// failSaga marks a saga instance as failed with an error message. Best-effort.
func failSaga(ctx context.Context, repo multisubdomain.SagaRepository, saga *multisubdomain.SagaInstance, errMsg string, logger *slog.Logger) {
	if saga == nil {
		return
	}
	if err := repo.Fail(ctx, saga.ID, errMsg); err != nil {
		logger.Warn("failed to mark saga as failed",
			slog.String("saga_id", saga.ID),
			slog.Any("error", err),
		)
	}
}

// checkpointSaga persists the saga's current step and serialized state data.
// Best-effort: failures are logged but do not abort the saga.
func checkpointSaga(ctx context.Context, repo multisubdomain.SagaRepository, saga *multisubdomain.SagaInstance, step int, stateData []byte, logger *slog.Logger) {
	if saga == nil {
		return
	}
	if err := repo.UpdateProgress(ctx, saga.ID, step, stateData); err != nil {
		logger.Warn("failed to checkpoint saga progress",
			slog.String("saga_id", saga.ID),
			slog.Int("step", step),
			slog.Any("error", err),
		)
	}
}
