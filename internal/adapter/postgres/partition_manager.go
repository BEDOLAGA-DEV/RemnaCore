package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

// partitionCountSQL returns the number of child partitions of public.outbox.
const partitionCountSQL = `SELECT count(*) FROM pg_inherits WHERE inhparent = 'outbox'::regclass`

var partitionGaugeOnce sync.Once
var partitionCountGauge prometheus.Gauge

func initPartitionGauge() {
	partitionGaugeOnce.Do(func() {
		partitionCountGauge = prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Subsystem: "outbox",
			Name:      "partition_count",
			Help:      "Current number of outbox partitions.",
		})
		// Register may fail if metric already registered (safe to ignore).
		_ = prometheus.Register(partitionCountGauge)
	})
}

// partitionCheckInterval is the period between automatic partition
// ensure/cleanup runs. Partitions change quarterly, so daily is sufficient.
const partitionCheckInterval = 24 * time.Hour

// partitionManagerLockID is the fixed key for pg_try_advisory_lock.
// Only one pod acquires it at a time, preventing concurrent DDL on
// outbox partitions.
const partitionManagerLockID = 847293651

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

// isSafeToDropSQL verifies that all events in a partition have been processed by
// checking that the partition's max sequence_number is less than the minimum
// unpublished sequence_number across the entire outbox. When no unpublished
// events exist anywhere, the right-hand side returns maxInt64, making the
// comparison safe (all partitions are droppable).
const isSafeToDropSQL = `SELECT (
	SELECT COALESCE(MAX(sequence_number), 0) FROM %s
) < (
	SELECT COALESCE(MIN(sequence_number), 9223372036854775807)
	FROM public.outbox WHERE published = false
)`

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

	// lockMu guards lockConn. A session-level advisory lock lives on the
	// connection that acquired it, so the lock must be held on a dedicated
	// connection for its whole lifetime — pool.QueryRow would run the lock on
	// an arbitrary pooled connection and hand it straight back, leaving the
	// lock stranded on a random connection and pg_advisory_unlock running on a
	// different one (a silent no-op → leaked lock).
	lockMu   sync.Mutex
	lockConn *pgxpool.Conn
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
	initPartitionGauge()
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
	pm.updatePartitionCount(ctx)

	ticker := time.NewTicker(partitionCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pm.ensure(ctx)
			pm.cleanup(ctx)
			pm.updatePartitionCount(ctx)
		}
	}
}

// tryAdvisoryLock attempts to acquire a session-level advisory lock on a
// dedicated connection held for the lock's lifetime. Returns true if the lock
// was acquired, false if another session holds it or the query fails.
func (pm *PartitionManager) tryAdvisoryLock(ctx context.Context) bool {
	pm.lockMu.Lock()
	defer pm.lockMu.Unlock()

	if pm.lockConn != nil {
		// This manager already holds the lock; do not stack a re-entrant
		// acquisition (session advisory locks nest, and a single unlock would
		// then leave the lock held).
		return false
	}

	conn, err := pm.pool.Acquire(ctx)
	if err != nil {
		pm.logger.Warn("partition manager: advisory lock connection acquire failed", slog.Any("error", err))
		return false
	}

	var acquired bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", partitionManagerLockID).Scan(&acquired); err != nil {
		pm.logger.Warn("partition manager: advisory lock query failed", slog.Any("error", err))
		conn.Release()
		return false
	}
	if !acquired {
		conn.Release()
		return false
	}

	pm.lockConn = conn
	return true
}

// releaseAdvisoryLock releases the session-level advisory lock previously
// acquired by tryAdvisoryLock, unlocking on the SAME connection that holds it
// and returning that connection to the pool. Errors are logged but not
// propagated. A no-op if this manager does not currently hold the lock.
func (pm *PartitionManager) releaseAdvisoryLock(ctx context.Context) {
	pm.lockMu.Lock()
	defer pm.lockMu.Unlock()

	if pm.lockConn == nil {
		return
	}
	if _, err := pm.lockConn.Exec(ctx, "SELECT pg_advisory_unlock($1)", partitionManagerLockID); err != nil {
		pm.logger.Warn("partition manager: advisory unlock failed", slog.Any("error", err))
	}
	pm.lockConn.Release()
	pm.lockConn = nil
}

// ensure delegates to OutboxRepository.EnsurePartitions to pre-create future
// quarterly partitions. Errors are logged but do not stop the manager.
func (pm *PartitionManager) ensure(ctx context.Context) {
	if !pm.tryAdvisoryLock(ctx) {
		pm.logger.Debug("partition manager: another instance holds lock, skipping ensure")
		return
	}
	defer pm.releaseAdvisoryLock(ctx)

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
//
// This is the PartitionManager side of the two-tier outbox cleanup strategy.
// It drops entire past-retention partitions (instant O(1) via DETACH + DROP)
// that contain no unpublished events and pass the sequence safety check.
// Relay cleanup (outbox_relay.go) skips rows in these partitions by scoping
// its DELETE to created_at >= current quarter start, so there is no redundant
// DELETE + autovacuum work on partitions destined for DROP.
//
// Complementary cleanup by OutboxRelay.cleanup handles intra-partition purging
// for the active quarter using DELETE WHERE published=true. See the godoc on
// OutboxRelay.cleanup for the full division of responsibility.
func (pm *PartitionManager) cleanup(ctx context.Context) {
	if pm.retention == 0 {
		return
	}

	if !pm.tryAdvisoryLock(ctx) {
		pm.logger.Debug("partition manager: another instance holds lock, skipping cleanup")
		return
	}
	defer pm.releaseAdvisoryLock(ctx)

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

		safe, err := pm.isSafeToDrop(ctx, name)
		if err != nil {
			pm.logger.Error("partition manager: failed sequence safety check",
				slog.String("partition", name),
				slog.Any("error", err),
			)
			continue
		}
		if !safe {
			pm.logger.Warn("partition manager: skipping partition that fails sequence safety check",
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

// isSafeToDrop verifies that all events in the partition have been processed
// by checking that the partition's max sequence_number is less than the
// minimum unpublished sequence_number across the entire outbox. This prevents
// dropping a partition whose events have not yet been consumed downstream,
// even if all rows within the partition are marked as published.
//
// When no unpublished events exist globally, the right-hand side defaults to
// maxInt64 (9223372036854775807), making the comparison always true.
func (pm *PartitionManager) isSafeToDrop(ctx context.Context, partitionName string) (bool, error) {
	if !outboxPartitionPattern.MatchString(partitionName) {
		return false, fmt.Errorf("invalid partition name: %s", partitionName)
	}

	query := fmt.Sprintf(isSafeToDropSQL, partitionName)

	var safe bool
	if err := pm.pool.QueryRow(ctx, query).Scan(&safe); err != nil {
		return false, fmt.Errorf("check sequence safety for %s: %w", partitionName, err)
	}
	return safe, nil
}

// updatePartitionCount queries the current number of outbox partitions and
// updates the Prometheus gauge.
func (pm *PartitionManager) updatePartitionCount(ctx context.Context) {
	var count int64
	if err := pm.pool.QueryRow(ctx, partitionCountSQL).Scan(&count); err != nil {
		pm.logger.Warn("partition manager: failed to count partitions",
			slog.Any("error", err),
		)
		return
	}
	if partitionCountGauge != nil {
		partitionCountGauge.Set(float64(count))
	}
}
