package valkey

import (
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

// Module provides the Valkey client and rate limiters to the Fx dependency graph.
var Module = fx.Module("valkey",
	fx.Provide(NewClient),
	fx.Provide(func(client *redis.Client, clk clock.Clock) *SlidingWindowRateLimiter {
		return NewSlidingWindowRateLimiter(client, DefaultRateLimit, DefaultRateLimitWindow, clk)
	}),
	fx.Provide(NewDomainRateLimiter),
)
