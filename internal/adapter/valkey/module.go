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
// the default Prometheus registry. Duplicate registrations (e.g. during tests
// that construct the Fx graph multiple times) are silently ignored.
func registerMetrics(client *redis.Client) {
	_ = prometheus.Register(NewMetricsCollector(client))
}
