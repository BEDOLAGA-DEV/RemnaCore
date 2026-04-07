package valkey

import (
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/health"
)

// Module provides the Valkey client, rate limiters, and Prometheus metrics
// collector to the Fx dependency graph.
var Module = fx.Module("valkey",
	fx.Provide(NewClient),
	fx.Provide(
		fx.Annotate(
			func(client *redis.Client) health.Checker {
				return NewHealthChecker(client)
			},
			fx.ResultTags(`group:"health.checkers"`),
		),
	),
	fx.Provide(func(client *redis.Client, clk clock.Clock) *SlidingWindowRateLimiter {
		return NewSlidingWindowRateLimiter(client, DefaultRateLimit, DefaultRateLimitWindow, clk)
	}),
	fx.Provide(func(inner *SlidingWindowRateLimiter, cfg *config.Config, logger *slog.Logger) *ResilientRateLimiter {
		return NewResilientRateLimiter(inner, cfg.CircuitBreaker.Valkey, logger)
	}),
	fx.Provide(NewDomainRateLimiter),
	fx.Invoke(registerMetrics),
)

// registerMetrics creates and registers the Valkey pool stats collector with
// the default Prometheus registry. Duplicate registrations (e.g. during tests
// that construct the Fx graph multiple times) are silently ignored.
func registerMetrics(client *redis.Client) {
	// Register may fail if metric already registered (safe to ignore).
	_ = prometheus.Register(NewMetricsCollector(client))
}
