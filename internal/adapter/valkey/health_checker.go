package valkey

import (
	"context"

	"github.com/redis/go-redis/v9"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/health"
)

// componentName is the health check component name for Valkey.
const componentName = "valkey"

// HealthChecker implements health.Checker for the Valkey (Redis-compatible)
// connection.
type HealthChecker struct {
	client *redis.Client
}

// NewHealthChecker returns a Valkey health checker that validates connectivity
// by executing a PING command.
func NewHealthChecker(client *redis.Client) *HealthChecker {
	return &HealthChecker{client: client}
}

// HealthCheck pings Valkey and returns the component status.
func (c *HealthChecker) HealthCheck(ctx context.Context) health.ComponentCheck {
	if err := c.client.Ping(ctx).Err(); err != nil {
		return health.ComponentCheck{
			Name:   componentName,
			Status: health.StatusUnhealthy,
		}
	}

	stats := c.client.PoolStats()
	totalConns := stats.TotalConns
	// Degraded when stale connections exceed 25% of pool.
	const staleRatioThreshold = 0.25
	if totalConns > 0 && float64(stats.StaleConns)/float64(totalConns) > staleRatioThreshold {
		return health.ComponentCheck{
			Name:    componentName,
			Status:  health.StatusDegraded,
			Message: "high stale connection ratio",
		}
	}

	return health.ComponentCheck{
		Name:   componentName,
		Status: health.StatusHealthy,
	}
}
