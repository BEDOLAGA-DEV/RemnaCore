package valkey

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
)

// NewClient creates a Valkey (Redis-compatible) client and registers Fx
// lifecycle hooks for health-checking on start and graceful shutdown on stop.
func NewClient(lc fx.Lifecycle, cfg *config.Config) (*redis.Client, error) {
	opts, err := redis.ParseURL(cfg.Valkey.URL)
	if err != nil {
		return nil, fmt.Errorf("parsing valkey URL: %w", err)
	}

	client := redis.NewClient(opts)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := client.Ping(ctx).Err(); err != nil {
				return fmt.Errorf("valkey ping failed: %w", err)
			}
			return nil
		},
		OnStop: func(_ context.Context) error {
			return client.Close()
		},
	})

	return client, nil
}
