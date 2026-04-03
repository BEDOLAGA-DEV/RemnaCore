// Package postgres provides the PostgreSQL connection pool as an Fx module.
package postgres

//go:generate sqlc generate -f ../../../sqlc.yaml

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
)

// Module provides a *pgxpool.Pool to the Fx dependency graph and ensures
// the pool is closed gracefully on application shutdown.
var Module = fx.Module("postgres",
	fx.Provide(NewPool),
)

// NewPool creates a pgxpool.Pool configured from the application's database
// settings and registers a shutdown hook to close it.
func NewPool(lc fx.Lifecycle, cfg *config.Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.Database.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	poolCfg.MaxConns = int32(cfg.Database.MaxOpenConns)
	poolCfg.MinConns = int32(cfg.Database.MaxIdleConns)
	poolCfg.MaxConnLifetime = cfg.Database.ConnMaxLifetime

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			pool.Close()
			return nil
		},
	})

	return pool, nil
}
