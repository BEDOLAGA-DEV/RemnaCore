package remnawave

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sony/gobreaker/v2"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/circuitbreaker"
)

// CBName is the circuit breaker instance name.
const CBName = "remnawave"

// Per-operation timeout constants control how long each Remnawave API call
// is allowed to take before the context is cancelled. These are applied on
// top of the global http.Client timeout as a tighter per-call constraint.
const (
	TimeoutCreateUser  = 5 * time.Second
	TimeoutDeleteUser  = 5 * time.Second
	TimeoutEnableUser  = 3 * time.Second
	TimeoutDisableUser = 3 * time.Second
	TimeoutGetUser     = 5 * time.Second
	TimeoutUpdateUser  = 5 * time.Second
	TimeoutGetNodes    = 10 * time.Second
	TimeoutAssignSquad = 5 * time.Second
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
// are recorded as Prometheus metrics. Each operation applies a per-operation
// context timeout to prevent slow calls from blocking indefinitely.
type ResilientClient struct {
	client *Client
	cb     *gobreaker.CircuitBreaker[any]
}

// NewResilientClient wraps the provided Client with a circuit breaker using
// the given configuration.
func NewResilientClient(client *Client, cfg circuitbreaker.Config, logger *slog.Logger) *ResilientClient {
	registerCBMetrics()

	onStateChange := func(name string, from, to gobreaker.State) {
		logger.Warn("circuit breaker state changed",
			slog.String("name", name),
			slog.String("from", from.String()),
			slog.String("to", to.String()),
		)
		cbStateGauge.Set(float64(to))
		cbTransitionsTotal.WithLabelValues(to.String()).Inc()
	}

	return &ResilientClient{
		client: client,
		cb:     circuitbreaker.NewBreaker[any](CBName, cfg, onStateChange),
	}
}

// IsConfigured delegates to the underlying Client.
func (rc *ResilientClient) IsConfigured() bool {
	return rc.client.IsConfigured()
}

// withTimeout derives a context with the given per-operation timeout.
func withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

// CreateUser provisions a new VPN user through the circuit breaker.
func (rc *ResilientClient) CreateUser(ctx context.Context, req CreateUserRequest) (*RemnawaveUser, error) {
	ctx, cancel := withTimeout(ctx, TimeoutCreateUser)
	defer cancel()
	return cbExec(rc.cb, func() (*RemnawaveUser, error) {
		return rc.client.CreateUser(ctx, req)
	})
}

// GetNodes returns all proxy nodes through the circuit breaker.
func (rc *ResilientClient) GetNodes(ctx context.Context) ([]RemnawaveNode, error) {
	ctx, cancel := withTimeout(ctx, TimeoutGetNodes)
	defer cancel()
	return cbExec(rc.cb, func() ([]RemnawaveNode, error) {
		return rc.client.GetNodes(ctx)
	})
}

// GetUserByUUID retrieves a single VPN user with traffic stats through the
// circuit breaker.
func (rc *ResilientClient) GetUserByUUID(ctx context.Context, uuid string) (*RemnawaveUserWithTraffic, error) {
	ctx, cancel := withTimeout(ctx, TimeoutGetUser)
	defer cancel()
	return cbExec(rc.cb, func() (*RemnawaveUserWithTraffic, error) {
		return rc.client.GetUserByUUID(ctx, uuid)
	})
}

// UpdateUser modifies an existing VPN user through the circuit breaker.
func (rc *ResilientClient) UpdateUser(ctx context.Context, req UpdateUserRequest) (*RemnawaveUser, error) {
	ctx, cancel := withTimeout(ctx, TimeoutUpdateUser)
	defer cancel()
	return cbExec(rc.cb, func() (*RemnawaveUser, error) {
		return rc.client.UpdateUser(ctx, req)
	})
}

// DeleteUser removes a VPN user through the circuit breaker.
func (rc *ResilientClient) DeleteUser(ctx context.Context, uuid string) error {
	ctx, cancel := withTimeout(ctx, TimeoutDeleteUser)
	defer cancel()
	return cbExecNoResult(rc.cb, func() error {
		return rc.client.DeleteUser(ctx, uuid)
	})
}

// EnableUser activates a VPN user through the circuit breaker.
func (rc *ResilientClient) EnableUser(ctx context.Context, uuid string) error {
	ctx, cancel := withTimeout(ctx, TimeoutEnableUser)
	defer cancel()
	return cbExecNoResult(rc.cb, func() error {
		return rc.client.EnableUser(ctx, uuid)
	})
}

// DisableUser deactivates a VPN user through the circuit breaker.
func (rc *ResilientClient) DisableUser(ctx context.Context, uuid string) error {
	ctx, cancel := withTimeout(ctx, TimeoutDisableUser)
	defer cancel()
	return cbExecNoResult(rc.cb, func() error {
		return rc.client.DisableUser(ctx, uuid)
	})
}

// AddUsersToInternalSquad assigns users to an internal squad through the
// circuit breaker.
func (rc *ResilientClient) AddUsersToInternalSquad(ctx context.Context, squadUUID string, userUUIDs []string) error {
	ctx, cancel := withTimeout(ctx, TimeoutAssignSquad)
	defer cancel()
	return cbExecNoResult(rc.cb, func() error {
		return rc.client.AddUsersToInternalSquad(ctx, squadUUID, userUUIDs)
	})
}
