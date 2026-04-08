// Package health defines the component health checking contract used by the
// readiness endpoint to report per-component status with degraded support.
//
// Components implement the Checker interface. The readiness handler aggregates
// results: if any component is Unhealthy the overall status is Unhealthy; if
// any component is Degraded (and none Unhealthy) the overall is Degraded.
package health

import "context"

// Status represents the health of a single component or the aggregate system.
type Status string

const (
	// StatusHealthy means the component is operating normally.
	StatusHealthy Status = "healthy"

	// StatusDegraded means the component is operational but impaired. In
	// Kubernetes this keeps the pod running but may trigger drain policies.
	StatusDegraded Status = "degraded"

	// StatusUnhealthy means the component cannot serve requests and the pod
	// should be restarted or removed from the load balancer.
	StatusUnhealthy Status = "unhealthy"
)

// ComponentCheck reports the health of a single infrastructure component.
// Message is intentionally omitted from JSON when empty to avoid leaking
// internal details in production; it is populated only for operational
// diagnostics (e.g. "backlog count: 12345").
type ComponentCheck struct {
	Name      string  `json:"name"`
	Status    Status  `json:"status"`
	Message   string  `json:"message,omitempty"`
	LatencyMs float64 `json:"latency_ms"`
}

// Checker reports the health of a single infrastructure component.
// Implementations must complete within a reasonable timeout (1-2s max);
// the readiness handler enforces a global deadline.
type Checker interface {
	HealthCheck(ctx context.Context) ComponentCheck
}

// Aggregate computes the overall status from a slice of component checks.
// The worst status wins: Unhealthy > Degraded > Healthy.
func Aggregate(checks []ComponentCheck) Status {
	overall := StatusHealthy
	for _, c := range checks {
		switch c.Status {
		case StatusUnhealthy:
			return StatusUnhealthy
		case StatusDegraded:
			overall = StatusDegraded
		}
	}
	return overall
}
