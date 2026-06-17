package service

import (
	"context"
	"log/slog"
	"time"
)

// Cleanup interval constants.
const (
	// DefaultCleanupInterval is the period between identity cleanup runs.
	DefaultCleanupInterval = 1 * time.Hour
)

// CleanupRepository defines the subset of identity persistence operations
// needed by the CleanupScheduler. It is satisfied by both identity.Repository
// and postgres.IdentityRepository.
type CleanupRepository interface {
	DeleteExpiredSessions(ctx context.Context) (int64, error)
	DeleteExpiredVerifications(ctx context.Context) (int64, error)
	DeleteExpiredPasswordResets(ctx context.Context) (int64, error)
	DeleteExpiredInvitations(ctx context.Context) (int64, error)
}

// CleanupScheduler periodically removes expired sessions, email verifications,
// and password resets from the identity schema. These are ephemeral records
// whose useful lifetime is bound by their expires_at column; once expired,
// they serve no purpose and consume storage.
type CleanupScheduler struct {
	repo     CleanupRepository
	logger   *slog.Logger
	interval time.Duration
}

// NewCleanupScheduler creates a CleanupScheduler that runs at
// DefaultCleanupInterval.
func NewCleanupScheduler(repo CleanupRepository, logger *slog.Logger) *CleanupScheduler {
	return &CleanupScheduler{
		repo:     repo,
		logger:   logger,
		interval: DefaultCleanupInterval,
	}
}

// Run starts the cleanup loop. It blocks until the context is cancelled.
func (s *CleanupScheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
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

// cleanup deletes expired sessions, verifications, and password resets.
// Each operation is independent; a failure in one does not block the others.
func (s *CleanupScheduler) cleanup(ctx context.Context) {
	if n, err := s.repo.DeleteExpiredSessions(ctx); err != nil {
		s.logger.Warn("identity cleanup: expired sessions failed",
			slog.Any("error", err),
		)
	} else if n > 0 {
		s.logger.Info("identity cleanup: removed expired sessions",
			slog.Int64("count", n),
		)
	}

	if n, err := s.repo.DeleteExpiredVerifications(ctx); err != nil {
		s.logger.Warn("identity cleanup: expired verifications failed",
			slog.Any("error", err),
		)
	} else if n > 0 {
		s.logger.Info("identity cleanup: removed expired verifications",
			slog.Int64("count", n),
		)
	}

	if n, err := s.repo.DeleteExpiredPasswordResets(ctx); err != nil {
		s.logger.Warn("identity cleanup: expired password resets failed",
			slog.Any("error", err),
		)
	} else if n > 0 {
		s.logger.Info("identity cleanup: removed expired password resets",
			slog.Int64("count", n),
		)
	}

	if n, err := s.repo.DeleteExpiredInvitations(ctx); err != nil {
		s.logger.Warn("identity cleanup: expired invitations failed",
			slog.Any("error", err),
		)
	} else if n > 0 {
		s.logger.Info("identity cleanup: removed expired invitations",
			slog.Int64("count", n),
		)
	}
}
