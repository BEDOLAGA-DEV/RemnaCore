// Package postgres provides the PostgreSQL connection pool as an Fx module.
package postgres

//go:generate sqlc generate -f ../../../sqlc.yaml

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/observability"
)

// ioMethodIOURing is the recommended io_method value for best I/O performance
// on Linux 5.1+ kernels.
const ioMethodIOURing = "io_uring"

// Module provides a *pgxpool.Pool to the Fx dependency graph and ensures
// the pool is closed gracefully on application shutdown.
var Module = fx.Module("postgres",
	fx.Provide(NewPool),
	fx.Invoke(registerPoolMetrics),
	fx.Invoke(registerPgStatMetrics),
)

// NewPool creates a pgxpool.Pool configured from the application's database
// settings and registers a shutdown hook to close it.
func NewPool(lc fx.Lifecycle, cfg *config.Config, logger *slog.Logger) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.Database.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	poolCfg.MaxConns = int32(cfg.Database.MaxOpenConns)
	poolCfg.MinConns = int32(cfg.Database.MinConns)
	poolCfg.MaxConnLifetime = cfg.Database.ConnMaxLifetime
	poolCfg.MaxConnIdleTime = cfg.Database.ConnMaxIdleTime
	poolCfg.HealthCheckPeriod = cfg.Database.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	logIOMethod(pool, logger)

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			pool.Close()
			return nil
		},
	})

	return pool, nil
}

// logIOMethod queries the PostgreSQL io_method setting and logs a warning if
// it is not set to io_uring. This is informational only — io_uring requires
// Linux 5.1+ and may not be available on all deployment targets.
func logIOMethod(pool *pgxpool.Pool, logger *slog.Logger) {
	var ioMethod string
	if err := pool.QueryRow(context.Background(), "SHOW io_method").Scan(&ioMethod); err != nil {
		logger.Warn("failed to query PostgreSQL io_method", slog.Any("error", err))
		return
	}

	logger.Info("PostgreSQL io_method", slog.String("io_method", ioMethod))

	if ioMethod != ioMethodIOURing {
		logger.Warn("PostgreSQL io_method is not io_uring — consider enabling for better I/O performance on Linux 5.1+",
			slog.String("current", ioMethod),
			slog.String("recommended", ioMethodIOURing),
		)
	}
}

// registerPoolMetrics creates and registers the PG pool stats collector with
// the default Prometheus registry. Duplicate registrations (e.g. during tests
// that construct the Fx graph multiple times) are silently ignored.
func registerPoolMetrics(pool *pgxpool.Pool) {
	// Register may fail if metric already registered (safe to ignore).
	_ = prometheus.Register(NewMetricsCollector(pool))
}

// registerPgStatMetrics creates and registers the pg_stat_statements collector
// with the default Prometheus registry. Duplicate registrations are silently
// ignored to support tests that construct the Fx graph multiple times.
func registerPgStatMetrics(pool *pgxpool.Pool, logger *slog.Logger) {
	// Register may fail if metric already registered (safe to ignore).
	_ = prometheus.Register(observability.NewPgStatCollector(pool, logger))
}
