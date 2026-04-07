package nats

import (
	"context"

	nc "github.com/nats-io/nats.go"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/health"
)

// componentName is the health check component name for NATS.
const componentName = "nats"

// HealthChecker implements health.Checker for the NATS connection.
type HealthChecker struct {
	conn *nc.Conn
}

// NewHealthChecker returns a NATS health checker that reports status based on
// the underlying connection state.
func NewHealthChecker(conn *nc.Conn) *HealthChecker {
	return &HealthChecker{conn: conn}
}

// HealthCheck inspects the NATS connection status and returns the component
// health. A reconnecting state is reported as degraded rather than unhealthy
// because the client is actively recovering.
func (c *HealthChecker) HealthCheck(_ context.Context) health.ComponentCheck {
	if c.conn == nil {
		return health.ComponentCheck{
			Name:   componentName,
			Status: health.StatusUnhealthy,
		}
	}

	switch c.conn.Status() {
	case nc.CONNECTED:
		return health.ComponentCheck{
			Name:   componentName,
			Status: health.StatusHealthy,
		}
	case nc.RECONNECTING:
		return health.ComponentCheck{
			Name:    componentName,
			Status:  health.StatusDegraded,
			Message: "reconnecting",
		}
	default:
		return health.ComponentCheck{
			Name:   componentName,
			Status: health.StatusUnhealthy,
		}
	}
}
