package nats

import (
	"context"
	"fmt"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/health"
)

// outboxComponentName is the health check component name for the outbox relay.
const outboxComponentName = "outbox"

// OutboxHealthChecker implements health.Checker for the transactional outbox.
// It reports degraded when the unpublished event backlog exceeds the
// backpressure threshold, indicating the relay is falling behind.
type OutboxHealthChecker struct {
	outbox *postgres.OutboxRepository
}

// NewOutboxHealthChecker returns an outbox health checker.
func NewOutboxHealthChecker(outbox *postgres.OutboxRepository) *OutboxHealthChecker {
	return &OutboxHealthChecker{outbox: outbox}
}

// HealthCheck queries the unpublished event count and returns degraded when
// the backlog exceeds the backpressure threshold.
func (c *OutboxHealthChecker) HealthCheck(ctx context.Context) health.ComponentCheck {
	count, err := c.outbox.CountUnpublished(ctx)
	if err != nil {
		return health.ComponentCheck{
			Name:   outboxComponentName,
			Status: health.StatusDegraded,
			Message: "unable to query backlog",
		}
	}

	if count > OutboxBackpressureThreshold {
		return health.ComponentCheck{
			Name:    outboxComponentName,
			Status:  health.StatusDegraded,
			Message: fmt.Sprintf("backlog: %d events (threshold: %d)", count, OutboxBackpressureThreshold),
		}
	}

	return health.ComponentCheck{
		Name:   outboxComponentName,
		Status: health.StatusHealthy,
	}
}
