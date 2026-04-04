package valkey

import (
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

// Module provides the Valkey client and rate limiters to the Fx dependency graph.
var Module = fx.Module("valkey",
	fx.Provide(NewClient),
	fx.Provide(func(client *redis.Client) RateLimiter {
		return NewSlidingWindowRateLimiter(client, DefaultRateLimit, DefaultRateLimitWindow)
	}),
	fx.Provide(NewDomainRateLimiter),
)
