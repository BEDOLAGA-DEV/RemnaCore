// Package circuitbreaker provides a shared circuit breaker configuration type
// and factory function. All adapters that use gobreaker should create their
// breakers through this package to avoid duplicating configuration constants.
package circuitbreaker

import (
	"time"

	"github.com/sony/gobreaker/v2"
)

// Default configuration values shared across all circuit breaker instances.
// These match the values previously hardcoded in each adapter.
const (
	DefaultMaxFailures = uint32(5)
	DefaultTimeout     = 10 * time.Second
	DefaultMaxRequests = uint32(3)
	DefaultInterval    = 30 * time.Second
)

// Config holds circuit breaker parameters configurable per component.
type Config struct {
	// MaxFailures is the number of consecutive failures required to trip the
	// breaker from closed to open.
	MaxFailures uint32 `koanf:"max_failures"`

	// Timeout is the duration the circuit stays open before transitioning to
	// half-open.
	Timeout time.Duration `koanf:"timeout"`

	// MaxRequests is the number of probe requests allowed in the half-open
	// state to test whether the downstream has recovered.
	MaxRequests uint32 `koanf:"max_requests"`

	// Interval is the cyclic period of the closed state for clearing internal
	// counts. Zero means counts are never cleared in the closed state.
	Interval time.Duration `koanf:"interval"`
}

// DefaultConfig returns sensible defaults matching the previously hardcoded
// values used across all adapters.
func DefaultConfig() Config {
	return Config{
		MaxFailures: DefaultMaxFailures,
		Timeout:     DefaultTimeout,
		MaxRequests: DefaultMaxRequests,
		Interval:    DefaultInterval,
	}
}

// DefaultConfigNoInterval returns defaults with Interval set to zero. This is
// appropriate for breakers where only consecutive failures matter and the
// closed-state counter should never be reset on a timer (e.g. outbox relay,
// valkey rate limiter).
func DefaultConfigNoInterval() Config {
	cfg := DefaultConfig()
	cfg.Interval = 0
	return cfg
}

// Settings converts a Config into gobreaker.Settings with the given name.
// An optional OnStateChange callback can be provided for metrics/logging.
func Settings(name string, cfg Config, onStateChange func(name string, from, to gobreaker.State)) gobreaker.Settings {
	return gobreaker.Settings{
		Name:        name,
		MaxRequests: cfg.MaxRequests,
		Interval:    cfg.Interval,
		Timeout:     cfg.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= cfg.MaxFailures
		},
		OnStateChange: onStateChange,
	}
}

// NewBreaker creates a gobreaker.CircuitBreaker with the given config, name,
// and optional state change callback.
func NewBreaker[T any](name string, cfg Config, onStateChange func(name string, from, to gobreaker.State)) *gobreaker.CircuitBreaker[T] {
	return gobreaker.NewCircuitBreaker[T](Settings(name, cfg, onStateChange))
}
