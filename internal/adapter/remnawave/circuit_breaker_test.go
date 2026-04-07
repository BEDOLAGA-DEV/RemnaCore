package remnawave

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/sony/gobreaker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/circuitbreaker"
)

// testLogger returns a slog.Logger that suppresses all output below fatal.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError + 4, // suppress all output
	}))
}

// errRemnawave is a sentinel error simulating a Remnawave failure.
var errRemnawave = errors.New("remnawave: connection refused")

// testCBConfig returns the default CB config used in production.
func testCBConfig() circuitbreaker.Config {
	return circuitbreaker.DefaultConfig()
}

func TestCircuitBreaker_TripsAfterConsecutiveFailures(t *testing.T) {
	cfg := testCBConfig()
	cb := circuitbreaker.NewBreaker[any](CBName, cfg, nil)

	// Record consecutive failures up to the threshold.
	for i := range cfg.MaxFailures {
		t.Run(fmt.Sprintf("failure_%d", i+1), func(t *testing.T) {
			_, err := cb.Execute(func() (any, error) {
				return nil, errRemnawave
			})
			require.Error(t, err)
		})
	}

	assert.Equal(t, gobreaker.StateOpen, cb.State(),
		"breaker should be open after %d consecutive failures", cfg.MaxFailures)

	// Next call must be rejected without executing the function.
	functionCalled := false
	_, err := cb.Execute(func() (any, error) {
		functionCalled = true
		return nil, nil
	})
	assert.Error(t, err)
	assert.False(t, functionCalled, "function must not be called when breaker is open")
}

func TestCircuitBreaker_Constants(t *testing.T) {
	cfg := testCBConfig()

	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "name is non-empty",
			check: func(t *testing.T) {
				assert.NotEmpty(t, CBName)
			},
		},
		{
			name: "max requests is positive",
			check: func(t *testing.T) {
				assert.Greater(t, int(cfg.MaxRequests), 0)
			},
		},
		{
			name: "interval is positive",
			check: func(t *testing.T) {
				assert.Greater(t, cfg.Interval, time.Duration(0))
			},
		},
		{
			name: "timeout is positive",
			check: func(t *testing.T) {
				assert.Greater(t, cfg.Timeout, time.Duration(0))
			},
		},
		{
			name: "consecutive failures threshold is positive",
			check: func(t *testing.T) {
				assert.Greater(t, int(cfg.MaxFailures), 0)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.check)
	}
}

func TestCircuitBreaker_StateTransitionUpdatesMetrics(t *testing.T) {
	cfg := testCBConfig()

	// Use a fresh Prometheus registry to avoid conflicts with other tests
	// that may have registered the same metrics via sync.Once.
	registry := prometheus.NewRegistry()

	stateGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: cbMetricNamespace,
		Subsystem: cbMetricSubsystem,
		Name:      "test_state",
		Help:      cbMetricStateHelp,
	})
	transitionsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: cbMetricNamespace,
		Subsystem: cbMetricSubsystem,
		Name:      "test_transitions_total",
		Help:      cbMetricTransitionsHelp,
	}, []string{cbLabelToState})
	registry.MustRegister(stateGauge)
	registry.MustRegister(transitionsTotal)

	logger := testLogger()

	cb := circuitbreaker.NewBreaker[any](CBName, cfg, func(name string, from, to gobreaker.State) {
		logger.Warn("circuit breaker state changed",
			slog.String("name", name),
			slog.String("from", from.String()),
			slog.String("to", to.String()),
		)
		stateGauge.Set(float64(to))
		transitionsTotal.WithLabelValues(to.String()).Inc()
	})

	// Trip the breaker: closed -> open.
	for range cfg.MaxFailures {
		_, _ = cb.Execute(func() (any, error) {
			return nil, errRemnawave
		})
	}

	require.Equal(t, gobreaker.StateOpen, cb.State())

	// Verify gauge reflects the open state (gobreaker.StateOpen == 2).
	var gaugeMetric dto.Metric
	err := stateGauge.Write(&gaugeMetric)
	require.NoError(t, err)
	assert.Equal(t, float64(gobreaker.StateOpen), gaugeMetric.GetGauge().GetValue(),
		"state gauge should reflect the open state")

	// Verify transitions counter incremented for "open".
	var counterMetric dto.Metric
	openCounter, err := transitionsTotal.GetMetricWithLabelValues(gobreaker.StateOpen.String())
	require.NoError(t, err)
	err = openCounter.Write(&counterMetric)
	require.NoError(t, err)
	assert.Equal(t, float64(1), counterMetric.GetCounter().GetValue(),
		"transitions counter for 'open' should be 1")
}

func TestRegisterCBMetrics_Idempotent(t *testing.T) {
	// registerCBMetrics uses sync.Once, so calling it multiple times must
	// not panic or double-register.
	assert.NotPanics(t, func() {
		registerCBMetrics()
		registerCBMetrics()
	})
}

func TestPerOperationTimeouts_Positive(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{name: "create_user", timeout: TimeoutCreateUser},
		{name: "delete_user", timeout: TimeoutDeleteUser},
		{name: "enable_user", timeout: TimeoutEnableUser},
		{name: "disable_user", timeout: TimeoutDisableUser},
		{name: "get_user", timeout: TimeoutGetUser},
		{name: "update_user", timeout: TimeoutUpdateUser},
		{name: "get_nodes", timeout: TimeoutGetNodes},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Greater(t, tt.timeout, time.Duration(0),
				"per-operation timeout must be positive")
		})
	}
}
