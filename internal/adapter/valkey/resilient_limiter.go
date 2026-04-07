package valkey

import (
	"context"
	"log/slog"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sony/gobreaker/v2"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/observability"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/circuitbreaker"
)

// rlCBName is the circuit breaker instance name for the rate limiter.
const rlCBName = "valkey-rate-limiter"

// rlFallbackCounter is the singleton Prometheus counter for rate limiter
// fallback events. Registered once to avoid duplicate registration panics.
var (
	rlFallbackOnce    sync.Once
	rlFallbackCounter prometheus.Counter
)

// getRLFallbackCounter returns the singleton fallback counter, registering it
// on first access.
func getRLFallbackCounter() prometheus.Counter {
	rlFallbackOnce.Do(func() {
		rlFallbackCounter = promauto.NewCounter(prometheus.CounterOpts{
			Name: observability.MetricRateLimiterFallback,
			Help: observability.HelpRateLimiterFallback,
		})
	})
	return rlFallbackCounter
}

// Compile-time interface satisfaction check.
var _ RateLimiter = (*ResilientRateLimiter)(nil)

// ResilientRateLimiter wraps a RateLimiter with a circuit breaker. When the
// underlying store is unavailable, the breaker opens and Allow() returns
// (true, nil) immediately (fail-open), avoiding hammering a dead Valkey
// instance.
type ResilientRateLimiter struct {
	inner    RateLimiter
	breaker  *gobreaker.CircuitBreaker[bool]
	fallback prometheus.Counter
	logger   *slog.Logger
}

// NewResilientRateLimiter wraps the provided SlidingWindowRateLimiter with a
// circuit breaker configured by cbCfg. When the breaker opens, Allow returns
// (true, nil) to fail-open gracefully.
func NewResilientRateLimiter(inner *SlidingWindowRateLimiter, cbCfg circuitbreaker.Config, logger *slog.Logger) *ResilientRateLimiter {
	return newResilientRateLimiterFromInterface(inner, cbCfg, logger)
}

// newResilientRateLimiterFromInterface creates a ResilientRateLimiter wrapping
// any RateLimiter implementation. This allows tests to inject stubs without
// requiring a real SlidingWindowRateLimiter.
func newResilientRateLimiterFromInterface(inner RateLimiter, cbCfg circuitbreaker.Config, logger *slog.Logger) *ResilientRateLimiter {
	cb := circuitbreaker.NewBreaker[bool](rlCBName, cbCfg, nil)

	return &ResilientRateLimiter{
		inner:    inner,
		breaker:  cb,
		fallback: getRLFallbackCounter(),
		logger:   logger,
	}
}

// Allow checks the rate limit through the circuit breaker. If the breaker is
// open or the inner limiter returns an error, the request is allowed through
// (fail-open) and the fallback counter is incremented.
func (r *ResilientRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	result, err := r.breaker.Execute(func() (bool, error) {
		return r.inner.Allow(ctx, key)
	})
	if err != nil {
		// Circuit open or Valkey error -- fail open (allow request).
		r.fallback.Inc()
		r.logger.WarnContext(ctx, "rate limiter circuit breaker fallback",
			slog.String("key", key),
			slog.Any("error", err),
		)
		return true, nil
	}
	return result, nil
}
