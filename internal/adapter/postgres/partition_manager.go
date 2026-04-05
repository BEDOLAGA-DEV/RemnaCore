package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

// partitionCheckInterval is the period between automatic partition
// ensure/cleanup runs. Partitions change quarterly, so daily is sufficient.
const partitionCheckInterval = 24 * time.Hour

// partitionBoundPattern extracts the TO ('YYYY-MM-DD') upper bound from
// pg_get_expr output like: FOR VALUES FROM ('2026-01-01') TO ('2026-04-01')
var partitionBoundPattern = regexp.MustCompile(`TO \('(\d{4}-\d{2}-\d{2})`)

// listOutboxPartitionsSQL queries pg_catalog for all child tables of
// public.outbox along with their partition bound expressions.
const listOutboxPartitionsSQL = `
SELECT c.relname AS partition_name,
       pg_get_expr(c.relpartbound, c.oid) AS bound_expr
FROM pg_class p
JOIN pg_inherits i ON i.inhparent = p.oid
JOIN pg_class c ON c.oid = i.inhrelid
WHERE p.relname = 'outbox' AND p.relnamespace = 'public'::regnamespace
ORDER BY c.relname`

// hasUnpublishedSQL checks whether a partition contains any unpublished events.
// The partition name is validated against outboxPartitionPattern before
// interpolation to prevent SQL injection.
const hasUnpublishedSQL = `SELECT EXISTS (SELECT 1 FROM %s WHERE published = false)`

// PartitionManager ensures outbox partitions exist for the near future and
// cleans up old partitions past the retention period. It runs as a background
// service with daily checks.
type PartitionManager struct {
	outbox    *OutboxRepository
	pool      *pgxpool.Pool
	clock     clock.Clock
	logger    *slog.Logger
	lookahead int           // quarters ahead to ensure
	retention time.Duration // 0 = no cleanup
}

// NewPartitionManager creates a PartitionManager that pre-creates future
// outbox partitions and drops old ones whose data is fully published and
// past the retention window.
func NewPartitionManager(
	outbox *OutboxRepository,
	pool *pgxpool.Pool,
	clk clock.Clock,
	logger *slog.Logger,
	lookahead int,
	retention time.Duration,
) *PartitionManager {
	return &PartitionManager{
		outbox:    outbox,
		pool:      pool,
		clock:     clk,
		logger:    logger,
		lookahead: lookahead,
		retention: retention,
	}
}

// Run performs an initial ensure+cleanup cycle, then repeats every
// partitionCheckInterval until the context is cancelled.
func (pm *PartitionManager) Run(ctx context.Context) {
	pm.ensure(ctx)
	pm.cleanup(ctx)

	ticker := time.NewTicker(partitionCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pm.ensure(ctx)
			pm.cleanup(ctx)
		}
	}
}

// ensure delegates to OutboxRepository.EnsurePartitions to pre-create future
// quarterly partitions. Errors are logged but do not stop the manager.
func (pm *PartitionManager) ensure(ctx context.Context) {
	now := pm.clock.Now()
	if err := pm.outbox.EnsurePartitions(ctx, now, pm.lookahead); err != nil {
		pm.logger.Error("partition manager: failed to ensure partitions",
			slog.Any("error", err),
		)
		return
	}
	pm.logger.Info("partition manager: ensured partitions",
		slog.Int("lookahead_quarters", pm.lookahead),
	)
}

// cleanup finds partitions whose upper bound is entirely before now - retention
// and whose rows are all published, then detaches and drops them.
func (pm *PartitionManager) cleanup(ctx context.Context) {
	if pm.retention == 0 {
		return
	}

	cutoff := pm.clock.Now().Add(-pm.retention)

	candidates, err := pm.listExpiredPartitions(ctx, cutoff)
	if err != nil {
		pm.logger.Error("partition manager: failed to list expired partitions",
			slog.Any("error", err),
		)
		return
	}

	var dropped int
	for _, name := range candidates {
		unpublished, err := pm.hasUnpublished(ctx, name)
		if err != nil {
			pm.logger.Error("partition manager: failed to check unpublished events",
				slog.String("partition", name),
				slog.Any("error", err),
			)
			continue
		}
		if unpublished {
			pm.logger.Warn("partition manager: skipping partition with unpublished events",
				slog.String("partition", name),
			)
			continue
		}

		if err := pm.outbox.DetachAndDropPartition(ctx, name); err != nil {
			pm.logger.Error("partition manager: failed to drop partition",
				slog.String("partition", name),
				slog.Any("error", err),
			)
			continue
		}

		pm.logger.Info("partition manager: dropped expired partition",
			slog.String("partition", name),
		)
		dropped++
	}

	if dropped > 0 {
		pm.logger.Info("partition manager: cleanup complete",
			slog.Int("dropped", dropped),
		)
	}
}

// listExpiredPartitions returns the names of outbox partitions whose upper
// bound date is before cutoff. Only partitions matching the expected naming
// pattern are considered; the default partition is always skipped.
func (pm *PartitionManager) listExpiredPartitions(ctx context.Context, cutoff time.Time) ([]string, error) {
	rows, err := pm.pool.Query(ctx, listOutboxPartitionsSQL)
	if err != nil {
		return nil, fmt.Errorf("list outbox partitions: %w", err)
	}
	defer rows.Close()

	var expired []string
	for rows.Next() {
		var name, boundExpr string
		if err := rows.Scan(&name, &boundExpr); err != nil {
			return nil, fmt.Errorf("scan partition row: %w", err)
		}

		// Skip the default partition and names that don't match.
		if !outboxPartitionPattern.MatchString(name) || name == "outbox_default" {
			continue
		}

		upperBound, err := parseUpperBound(boundExpr)
		if err != nil {
			pm.logger.Warn("partition manager: could not parse bound expression",
				slog.String("partition", name),
				slog.String("bound_expr", boundExpr),
				slog.Any("error", err),
			)
			continue
		}

		if upperBound.Before(cutoff) {
			expired = append(expired, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate partition rows: %w", err)
	}

	return expired, nil
}

// parseUpperBound extracts the TO date from a pg_get_expr partition bound
// expression like: FOR VALUES FROM ('2026-01-01') TO ('2026-04-01')
func parseUpperBound(boundExpr string) (time.Time, error) {
	matches := partitionBoundPattern.FindStringSubmatch(boundExpr)
	if len(matches) < 2 {
		return time.Time{}, fmt.Errorf("no TO bound found in: %s", boundExpr)
	}
	t, err := time.Parse(time.DateOnly, matches[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("parse upper bound date %q: %w", matches[1], err)
	}
	return t, nil
}

// hasUnpublished checks whether the named partition contains any rows with
// published = false. The partition name MUST be validated against
// outboxPartitionPattern before calling this method.
func (pm *PartitionManager) hasUnpublished(ctx context.Context, partitionName string) (bool, error) {
	if !outboxPartitionPattern.MatchString(partitionName) {
		return false, fmt.Errorf("invalid partition name: %s", partitionName)
	}

	query := fmt.Sprintf(hasUnpublishedSQL, partitionName)
	var exists bool
	if err := pm.pool.QueryRow(ctx, query).Scan(&exists); err != nil {
		return false, fmt.Errorf("check unpublished in %s: %w", partitionName, err)
	}
	return exists, nil
}
