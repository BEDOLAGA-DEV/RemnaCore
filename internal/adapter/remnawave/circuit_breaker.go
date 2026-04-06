package remnawave

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sony/gobreaker/v2"
)

const (
	// CBName is the circuit breaker instance name.
	CBName = "remnawave"

	// CBMaxRequests is the number of requests allowed in the half-open state.
	CBMaxRequests = 3

	// CBInterval is the cyclic period of the closed state for clearing internal
	// counts. If zero, the internal counts are never cleared.
	CBInterval = 30 * time.Second

	// CBTimeout is the duration the circuit stays open before transitioning to
	// half-open.
	CBTimeout = 10 * time.Second

	// CBConsecutiveFailures is the number of consecutive failures required to
	// trip the breaker from closed to open.
	CBConsecutiveFailures = 5
)

// Prometheus metric constants for the circuit breaker.
const (
	cbMetricNamespace = "platform"
	cbMetricSubsystem = "circuit_breaker"

	cbMetricStateName       = "remnawave_state"
	cbMetricStateHelp       = "Current circuit breaker state (0=closed, 1=half-open, 2=open)"
	cbMetricTransitionsName = "remnawave_transitions_total"
	cbMetricTransitionsHelp = "Circuit breaker state transitions"

	cbLabelToState = "to_state"
)

var cbMetricsOnce sync.Once
var (
	cbStateGauge       prometheus.Gauge
	cbTransitionsTotal *prometheus.CounterVec
)

// registerCBMetrics registers the circuit breaker Prometheus metrics once.
func registerCBMetrics() {
	cbMetricsOnce.Do(func() {
		cbStateGauge = prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: cbMetricNamespace,
			Subsystem: cbMetricSubsystem,
			Name:      cbMetricStateName,
			Help:      cbMetricStateHelp,
		})
		cbTransitionsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: cbMetricNamespace,
			Subsystem: cbMetricSubsystem,
			Name:      cbMetricTransitionsName,
			Help:      cbMetricTransitionsHelp,
		}, []string{cbLabelToState})
		prometheus.MustRegister(cbStateGauge)
		prometheus.MustRegister(cbTransitionsTotal)
	})
}

// cbExec executes fn through the circuit breaker with type-safe generics.
func cbExec[T any](cb *gobreaker.CircuitBreaker[any], fn func() (T, error)) (T, error) {
	result, err := cb.Execute(func() (any, error) {
		return fn()
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return result.(T), nil
}

// cbExecNoResult executes fn through the circuit breaker when no result value
// is needed.
func cbExecNoResult(cb *gobreaker.CircuitBreaker[any], fn func() error) error {
	_, err := cb.Execute(func() (any, error) {
		return nil, fn()
	})
	return err
}

// ResilientClient wraps a Client with a circuit breaker that opens after
// consecutive failures and prevents cascading failures. State transitions
// are recorded as Prometheus metrics.
type ResilientClient struct {
	client *Client
	cb     *gobreaker.CircuitBreaker[any]
}

// NewResilientClient wraps the provided Client with a circuit breaker.
func NewResilientClient(client *Client, logger *slog.Logger) *ResilientClient {
	registerCBMetrics()

	settings := gobreaker.Settings{
		Name:        CBName,
		MaxRequests: CBMaxRequests,
		Interval:    CBInterval,
		Timeout:     CBTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= CBConsecutiveFailures
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			logger.Warn("circuit breaker state changed",
				slog.String("name", name),
				slog.String("from", from.String()),
				slog.String("to", to.String()),
			)
			cbStateGauge.Set(float64(to))
			cbTransitionsTotal.WithLabelValues(to.String()).Inc()
		},
	}

	return &ResilientClient{
		client: client,
		cb:     gobreaker.NewCircuitBreaker[any](settings),
	}
}

// CreateUser provisions a new VPN user through the circuit breaker.
func (rc *ResilientClient) CreateUser(ctx context.Context, req CreateUserRequest) (*RemnawaveUser, error) {
	return cbExec(rc.cb, func() (*RemnawaveUser, error) {
		return rc.client.CreateUser(ctx, req)
	})
}

// GetNodes returns all proxy nodes through the circuit breaker.
func (rc *ResilientClient) GetNodes(ctx context.Context) ([]RemnawaveNode, error) {
	return cbExec(rc.cb, func() ([]RemnawaveNode, error) {
		return rc.client.GetNodes(ctx)
	})
}

// GetUserByUUID retrieves a single VPN user with traffic stats through the
// circuit breaker.
func (rc *ResilientClient) GetUserByUUID(ctx context.Context, uuid string) (*RemnawaveUserWithTraffic, error) {
	return cbExec(rc.cb, func() (*RemnawaveUserWithTraffic, error) {
		return rc.client.GetUserByUUID(ctx, uuid)
	})
}

// UpdateUser modifies an existing VPN user through the circuit breaker.
func (rc *ResilientClient) UpdateUser(ctx context.Context, req UpdateUserRequest) (*RemnawaveUser, error) {
	return cbExec(rc.cb, func() (*RemnawaveUser, error) {
		return rc.client.UpdateUser(ctx, req)
	})
}

// DeleteUser removes a VPN user through the circuit breaker.
func (rc *ResilientClient) DeleteUser(ctx context.Context, uuid string) error {
	return cbExecNoResult(rc.cb, func() error {
		return rc.client.DeleteUser(ctx, uuid)
	})
}

// EnableUser activates a VPN user through the circuit breaker.
func (rc *ResilientClient) EnableUser(ctx context.Context, uuid string) error {
	return cbExecNoResult(rc.cb, func() error {
		return rc.client.EnableUser(ctx, uuid)
	})
}

// DisableUser deactivates a VPN user through the circuit breaker.
func (rc *ResilientClient) DisableUser(ctx context.Context, uuid string) error {
	return cbExecNoResult(rc.cb, func() error {
		return rc.client.DisableUser(ctx, uuid)
	})
}
