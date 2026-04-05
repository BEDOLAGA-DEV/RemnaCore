package service

import (
	"context"
	"log/slog"
	"time"

	multisubdomain "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
)

const (
	// SagaCleanupInterval is the period between cleanup runs that remove old
	// completed and failed saga instances.
	SagaCleanupInterval = 6 * time.Hour

	// SagaRetentionPeriod is how long completed/failed saga instances are
	// kept before being eligible for cleanup.
	SagaRetentionPeriod = 7 * 24 * time.Hour // 7 days
)

// SagaCleanupService periodically removes old saga instances and, on startup,
// logs any sagas that were left in 'running' status (indicating a crash). The
// BindingReconciler handles the actual recovery of orphaned resources; this
// service ensures the saga_instances table does not grow unbounded and
// provides observability into incomplete sagas.
type SagaCleanupService struct {
	sagaRepo multisubdomain.SagaRepository
	logger   *slog.Logger
}

// NewSagaCleanupService creates a SagaCleanupService with its dependencies.
func NewSagaCleanupService(
	sagaRepo multisubdomain.SagaRepository,
	logger *slog.Logger,
) *SagaCleanupService {
	return &SagaCleanupService{
		sagaRepo: sagaRepo,
		logger:   logger,
	}
}

// ReportStaleOnStartup queries for sagas left in 'running' status and logs
// them as warnings. These indicate sagas that were interrupted by a crash.
// Called once during application startup.
func (s *SagaCleanupService) ReportStaleOnStartup(ctx context.Context) {
	running, err := s.sagaRepo.GetRunning(ctx)
	if err != nil {
		s.logger.Error("saga cleanup: failed to query running sagas on startup",
			slog.Any("error", err),
		)
		return
	}

	if len(running) == 0 {
		s.logger.Info("saga cleanup: no stale running sagas found on startup")
		return
	}

	s.logger.Warn("saga cleanup: found stale running sagas from previous process lifetime",
		slog.Int("count", len(running)),
	)
	for _, saga := range running {
		s.logger.Warn("saga cleanup: stale saga",
			slog.String("saga_id", saga.ID),
			slog.String("saga_type", string(saga.SagaType)),
			slog.String("correlation_id", saga.CorrelationID),
			slog.Int("current_step", saga.CurrentStep),
			slog.Int("total_steps", saga.TotalSteps),
			slog.Time("created_at", saga.CreatedAt),
		)

		// Mark stale running sagas as failed so they are eligible for cleanup
		// and don't block new saga creation for the same correlation ID.
		if err := s.sagaRepo.Fail(ctx, saga.ID, "stale: process restarted before completion"); err != nil {
			s.logger.Error("saga cleanup: failed to mark stale saga as failed",
				slog.String("saga_id", saga.ID),
				slog.Any("error", err),
			)
		}
	}
}

// Run starts a blocking loop that cleans up old saga instances at
// SagaCleanupInterval. It returns when the context is cancelled.
func (s *SagaCleanupService) Run(ctx context.Context) {
	ticker := time.NewTicker(SagaCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cleanup(ctx)
		}
	}
}

// cleanup removes completed and failed sagas older than SagaRetentionPeriod.
func (s *SagaCleanupService) cleanup(ctx context.Context) {
	if err := s.sagaRepo.Cleanup(ctx, SagaRetentionPeriod); err != nil {
		s.logger.Error("saga cleanup: failed to cleanup old sagas",
			slog.Any("error", err),
		)
	}
}
