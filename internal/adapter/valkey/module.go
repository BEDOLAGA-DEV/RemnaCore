package valkey

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

// Module provides the Valkey client, rate limiters, and Prometheus metrics
// collector to the Fx dependency graph.
var Module = fx.Module("valkey",
	fx.Provide(NewClient),
	fx.Provide(func(client *redis.Client, clk clock.Clock) *SlidingWindowRateLimiter {
		return NewSlidingWindowRateLimiter(client, DefaultRateLimit, DefaultRateLimitWindow, clk)
	}),
	fx.Provide(NewDomainRateLimiter),
	fx.Invoke(registerMetrics),
)

// registerMetrics creates and registers the Valkey pool stats collector with
// the default Prometheus registry. Called by Fx on application start.
func registerMetrics(client *redis.Client) {
	prometheus.MustRegister(NewMetricsCollector(client))
}
