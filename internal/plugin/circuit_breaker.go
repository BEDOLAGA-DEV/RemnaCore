package plugin

import (
	"sync"
	"sync/atomic"
	"time"
)

// hookBreakerState represents the state of a per-plugin-hook circuit breaker.
type hookBreakerState int32

const (
	hookBreakerClosed   hookBreakerState = 0
	hookBreakerOpen     hookBreakerState = 1
	hookBreakerHalfOpen hookBreakerState = 2
)

// Circuit breaker thresholds.
const (
	// HookBreakerFailureThreshold is the number of consecutive failures
	// before the breaker transitions from closed to open.
	HookBreakerFailureThreshold int64 = 5

	// HookBreakerResetTimeout is how long the breaker stays open before
	// transitioning to half-open to allow a test call.
	HookBreakerResetTimeout = 30 * time.Second

	// HookBreakerHalfOpenMaxCalls is the number of test calls allowed in
	// the half-open state. A success closes the breaker; a failure reopens it.
	HookBreakerHalfOpenMaxCalls int64 = 1
)

// hookBreaker tracks failure rate for a single plugin-hook pair and
// short-circuits dispatch when the failure threshold is exceeded.
type hookBreaker struct {
	failures    atomic.Int64
	successes   atomic.Int64
	lastFailure atomic.Int64 // unix nanoseconds of last failure
	state       atomic.Int32 // hookBreakerState
	halfOpen    atomic.Int64 // test calls allowed in half-open state
}

// allow reports whether the breaker permits a call. Returns true when the
// breaker is closed, half-open (for test calls), or when the reset timeout
// has elapsed on an open breaker.
func (b *hookBreaker) allow(now time.Time) bool {
	state := hookBreakerState(b.state.Load())
	switch state {
	case hookBreakerClosed:
		return true
	case hookBreakerOpen:
		if now.UnixNano()-b.lastFailure.Load() > HookBreakerResetTimeout.Nanoseconds() {
			if b.state.CompareAndSwap(int32(hookBreakerOpen), int32(hookBreakerHalfOpen)) {
				b.halfOpen.Store(0)
			}
			return true
		}
		return false
	case hookBreakerHalfOpen:
		// Allow up to HookBreakerHalfOpenMaxCalls test calls.
		return b.halfOpen.Add(1) <= HookBreakerHalfOpenMaxCalls
	}
	return true
}

// recordSuccess resets the failure counter and closes the breaker.
func (b *hookBreaker) recordSuccess() {
	b.failures.Store(0)
	b.successes.Add(1)
	b.state.Store(int32(hookBreakerClosed))
}

// recordFailure increments the failure counter. When the consecutive failure
// count reaches HookBreakerFailureThreshold, the breaker opens.
func (b *hookBreaker) recordFailure(now time.Time) {
	b.lastFailure.Store(now.UnixNano())
	newCount := b.failures.Add(1)

	state := hookBreakerState(b.state.Load())
	switch state {
	case hookBreakerClosed:
		if newCount >= HookBreakerFailureThreshold {
			b.state.Store(int32(hookBreakerOpen))
		}
	case hookBreakerHalfOpen:
		// Test call failed -- reopen immediately.
		b.state.Store(int32(hookBreakerOpen))
	case hookBreakerOpen:
		// Already open, nothing to do.
	}
}

// isOpen reports whether the breaker is currently in the open state.
func (b *hookBreaker) isOpen() bool {
	return hookBreakerState(b.state.Load()) == hookBreakerOpen
}

// hookCircuitBreakers manages per-plugin-hook circuit breakers. Breakers are
// created lazily on first access and keyed by "pluginSlug:hookName".
type hookCircuitBreakers struct {
	mu       sync.RWMutex
	breakers map[string]*hookBreaker
	nowFn    func() time.Time // injectable for testing
}

// breakerKeySeparator separates plugin slug from hook name in the breaker map key.
const breakerKeySeparator = ":"

// newHookCircuitBreakers creates an empty breaker registry.
func newHookCircuitBreakers() *hookCircuitBreakers {
	return &hookCircuitBreakers{
		breakers: make(map[string]*hookBreaker),
		nowFn:    time.Now,
	}
}

// now returns the current time using the injectable clock function.
func (cb *hookCircuitBreakers) now() time.Time {
	return cb.nowFn()
}

// get returns the breaker for the given key, creating one lazily if it does
// not exist. The key is expected to be "pluginSlug:hookName".
func (cb *hookCircuitBreakers) get(key string) *hookBreaker {
	cb.mu.RLock()
	b, ok := cb.breakers[key]
	cb.mu.RUnlock()
	if ok {
		return b
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Double-check under write lock.
	if b, ok = cb.breakers[key]; ok {
		return b
	}

	b = &hookBreaker{}
	cb.breakers[key] = b
	return b
}

// remove deletes the breaker for the given key. Used when a plugin is
// unregistered so stale breakers do not accumulate.
func (cb *hookCircuitBreakers) remove(key string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	delete(cb.breakers, key)
}

// removeByPlugin removes all breakers whose key starts with the given plugin
// slug prefix. Used when a plugin is fully unregistered.
func (cb *hookCircuitBreakers) removeByPlugin(pluginSlug string) {
	prefix := pluginSlug + breakerKeySeparator
	cb.mu.Lock()
	defer cb.mu.Unlock()
	for key := range cb.breakers {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(cb.breakers, key)
		}
	}
}
