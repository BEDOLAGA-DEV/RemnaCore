package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/health"
)

// componentName is the health check component name for PostgreSQL.
const componentName = "postgres"

// HealthChecker implements health.Checker for the PostgreSQL connection pool.
type HealthChecker struct {
	pool *pgxpool.Pool
}

// NewHealthChecker returns a PostgreSQL health checker that validates
// connectivity by executing a lightweight ping.
func NewHealthChecker(pool *pgxpool.Pool) *HealthChecker {
	return &HealthChecker{pool: pool}
}

// HealthCheck pings the database and returns the component status.
func (c *HealthChecker) HealthCheck(ctx context.Context) health.ComponentCheck {
	if err := c.pool.Ping(ctx); err != nil {
		return health.ComponentCheck{
			Name:   componentName,
			Status: health.StatusUnhealthy,
		}
	}

	stats := c.pool.Stat()
	totalConns := stats.TotalConns()
	maxConns := stats.MaxConns()

	// Degraded when connection pool utilisation exceeds 90%.
	const poolUtilisationThreshold = 0.9
	if totalConns > 0 && float64(totalConns)/float64(maxConns) > poolUtilisationThreshold {
		return health.ComponentCheck{
			Name:    componentName,
			Status:  health.StatusDegraded,
			Message: fmt.Sprintf("pool utilisation %.0f%%", float64(totalConns)/float64(maxConns)*100),
		}
	}

	return health.ComponentCheck{
		Name:   componentName,
		Status: health.StatusHealthy,
	}
}
