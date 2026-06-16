package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

// DefaultMetricsInterval is the period between metrics snapshots. Hourly
// snapshots give a coarse-grained time series suitable for dashboard trends
// without bloating the samples table.
const DefaultMetricsInterval = 1 * time.Hour

// insertSampleSQL snapshots the DB-derivable platform metrics into
// metrics.samples in a single statement. $1 is the capture timestamp (the
// collector's injected clock) so the captured_at is deterministic/testable.
//
// mrr_cents normalizes each active subscription's plan price to a monthly
// figure by billing interval, byte-for-byte identical to the metricsSQL used
// by GET /api/admin/metrics (month=base, quarter=base/3, year=base/12).
//
// active_users has no direct source (identity.platform_users has no active
// column), so it is defined as the count of distinct users holding an active
// subscription — consistent with the billing-oriented metrics around it.
const insertSampleSQL = `
INSERT INTO metrics.samples (captured_at, active_users, active_subs, mrr_cents, total_subs, paying_users)
SELECT
    $1::timestamptz,
    (SELECT count(DISTINCT user_id) FROM billing.subscriptions WHERE status = 'active'),
    (SELECT count(*)                FROM billing.subscriptions WHERE status = 'active'),
    (SELECT coalesce(sum(
        CASE p.billing_interval
            WHEN 'month'   THEN p.base_price_amount
            WHEN 'quarter' THEN p.base_price_amount / 3
            WHEN 'year'    THEN p.base_price_amount / 12
            ELSE p.base_price_amount
        END
     ), 0)
     FROM billing.subscriptions s
     JOIN billing.plans p ON p.id = s.plan_id
     WHERE s.status = 'active'),
    (SELECT count(*)                FROM billing.subscriptions),
    (SELECT count(DISTINCT user_id) FROM billing.invoices WHERE status = 'paid')
`

// MetricsCollector periodically snapshots platform metrics into
// metrics.samples so the admin dashboard can render time-series trends. It
// reads DB-derivable metrics directly from the pool (cross-schema), mirroring
// the StatsHandler query pattern.
type MetricsCollector struct {
	pool     *pgxpool.Pool
	clock    clock.Clock
	logger   *slog.Logger
	interval time.Duration
}

// NewMetricsCollector creates a MetricsCollector that snapshots at
// DefaultMetricsInterval.
func NewMetricsCollector(pool *pgxpool.Pool, clk clock.Clock, logger *slog.Logger) *MetricsCollector {
	return &MetricsCollector{
		pool:     pool,
		clock:    clk,
		logger:   logger,
		interval: DefaultMetricsInterval,
	}
}

// Run starts the snapshot loop. It blocks until the context is cancelled.
func (s *MetricsCollector) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.snapshot(ctx)
		}
	}
}

// snapshot captures one metrics sample. Failures are logged and do not stop
// the loop.
func (s *MetricsCollector) snapshot(ctx context.Context) {
	tag, err := s.pool.Exec(ctx, insertSampleSQL, s.clock.Now())
	if err != nil {
		s.logger.Warn("metrics collector: snapshot failed", slog.Any("error", err))
		return
	}
	s.logger.Info("metrics collector: snapshot captured", slog.Int64("rows", tag.RowsAffected()))
}
