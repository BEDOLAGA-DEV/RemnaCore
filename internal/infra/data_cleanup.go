// Package infra provides cross-cutting infrastructure services.
package infra

import (
	"context"
	"log/slog"
	"time"
)

// Data cleanup constants.
const (
	// DataCleanupInterval is the period between cleanup runs for cross-domain
	// data (idempotency keys, binding sync log).
	DataCleanupInterval = 6 * time.Hour

	// IdempotencyKeyRetention is the maximum age for idempotency keys before
	// they are eligible for cleanup. Keys older than this are stale and safe
	// to remove regardless of their expires_at value.
	IdempotencyKeyRetention = 7 * 24 * time.Hour // 7 days

	// BindingSyncLogRetention is the maximum age for binding sync log entries.
	// Entries older than this are historical and no longer needed for debugging.
	BindingSyncLogRetention = 90 * 24 * time.Hour // 90 days
)

// IdempotencyKeysCleaner provides time-based cleanup for idempotency keys.
type IdempotencyKeysCleaner interface {
	CleanupOldKeys(ctx context.Context, cutoff time.Time) (int64, error)
}

// BindingSyncLogCleaner provides time-based cleanup for binding sync log.
type BindingSyncLogCleaner interface {
	CleanupOldSyncLog(ctx context.Context, cutoff time.Time) (int64, error)
}

// DataCleanupScheduler periodically removes old idempotency keys and binding
// sync log entries that have exceeded their retention period. It operates
// across domain boundaries (multisub schema) and is therefore placed in the
// infra layer rather than any single domain.
type DataCleanupScheduler struct {
	idempotency IdempotencyKeysCleaner
	syncLog     BindingSyncLogCleaner
	logger      *slog.Logger
	interval    time.Duration
}

// NewDataCleanupScheduler creates a DataCleanupScheduler with its
// dependencies and the default cleanup interval.
func NewDataCleanupScheduler(
	idempotency IdempotencyKeysCleaner,
	syncLog BindingSyncLogCleaner,
	logger *slog.Logger,
) *DataCleanupScheduler {
	return &DataCleanupScheduler{
		idempotency: idempotency,
		syncLog:     syncLog,
		logger:      logger,
		interval:    DataCleanupInterval,
	}
}

// Run starts the cleanup loop. It blocks until the context is cancelled.
func (s *DataCleanupScheduler) Run(ctx context.Context) {
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

// cleanup removes old idempotency keys and binding sync log entries.
// Each operation is independent; a failure in one does not block the others.
func (s *DataCleanupScheduler) cleanup(ctx context.Context) {
	now := time.Now()

	idempotencyCutoff := now.Add(-IdempotencyKeyRetention)
	if n, err := s.idempotency.CleanupOldKeys(ctx, idempotencyCutoff); err != nil {
		s.logger.Warn("data cleanup: idempotency keys failed",
			slog.Any("error", err),
		)
	} else if n > 0 {
		s.logger.Info("data cleanup: removed old idempotency keys",
			slog.Int64("count", n),
		)
	}

	syncLogCutoff := now.Add(-BindingSyncLogRetention)
	if n, err := s.syncLog.CleanupOldSyncLog(ctx, syncLogCutoff); err != nil {
		s.logger.Warn("data cleanup: binding sync log failed",
			slog.Any("error", err),
		)
	} else if n > 0 {
		s.logger.Info("data cleanup: removed old binding sync log entries",
			slog.Int64("count", n),
		)
	}
}
