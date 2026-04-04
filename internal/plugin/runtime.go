package plugin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Drain configuration.
const (
	// DrainTimeout is the maximum time to wait for in-flight requests to
	// complete before force-closing a draining pool.
	DrainTimeout = 30 * time.Second
)

// WASM runner corruption indicators. If a runner error contains any of these
// substrings (case-insensitive), the runner is considered corrupted and must
// not be returned to the pool.
var wasmCorruptionIndicators = []string{
	"wasm",
	"memory",
	"unreachable",
	"out of fuel",
	"panic",
	"trap",
}

// WASMRunner abstracts the execution of WASM functions so the runtime pool and
// dispatcher can be tested without a real Extism/wazero runtime.
type WASMRunner interface {
	// Call invokes an exported WASM function by name with the given input bytes
	// and returns the output bytes.
	Call(ctx context.Context, funcName string, input []byte) ([]byte, error)
	// Close releases all resources held by the runner.
	Close() error
}

// WASMRunnerFactory creates a WASMRunner from compiled WASM bytes and
// configuration. The real implementation uses the Extism Go SDK; tests supply a
// mock factory.
type WASMRunnerFactory func(wasmBytes []byte, config map[string]string) (WASMRunner, error)

// PluginInstancePool manages a pool of WASMRunner instances for a single
// plugin. It uses a buffered channel as a semaphore-style pool, allowing
// concurrent callers to each acquire their own instance.
//
// The pool supports graceful drain: when replaced during a hot reload, the old
// pool stops accepting new acquires while in-flight requests finish on their
// existing runners. Once all in-flight runners are released, the pool closes
// them and terminates.
type PluginInstancePool struct {
	slug     string
	factory  WASMRunnerFactory
	wasm     []byte
	config   map[string]string
	manifest *Manifest
	pool     chan WASMRunner
	poolSize int

	// Drain state.
	mu       sync.Mutex
	draining bool
	active   int32        // count of currently acquired (in-flight) runners
	drained  chan struct{} // closed when active reaches 0 after drain starts
}

// newPluginInstancePool pre-creates size WASM instances and returns a pool.
// If any instance fails to create, already-created instances are closed and
// the error is returned.
func newPluginInstancePool(slug string, factory WASMRunnerFactory, wasm []byte, config map[string]string, manifest *Manifest, size int) (*PluginInstancePool, error) {
	if size <= 0 {
		size = DefaultPoolSize
	}
	if size > MaxPoolSize {
		size = MaxPoolSize
	}

	p := &PluginInstancePool{
		slug:     slug,
		factory:  factory,
		wasm:     wasm,
		config:   config,
		manifest: manifest,
		pool:     make(chan WASMRunner, size),
		poolSize: size,
		drained:  make(chan struct{}),
	}

	// Pre-create all instances.
	for i := range size {
		runner, err := factory(wasm, config)
		if err != nil {
			p.Close() // cleanup already created
			return nil, fmt.Errorf("create instance %d for %s: %w", i, slug, err)
		}
		p.pool <- runner
	}

	return p, nil
}

// Acquire gets an instance from the pool, blocking until one is available or
// the context is cancelled/timed out. Returns an error if the pool is
// draining.
func (p *PluginInstancePool) Acquire(ctx context.Context) (WASMRunner, error) {
	p.mu.Lock()
	if p.draining {
		p.mu.Unlock()
		return nil, fmt.Errorf("%w: plugin %s", ErrPluginDraining, p.slug)
	}
	p.mu.Unlock()

	select {
	case runner := <-p.pool:
		atomic.AddInt32(&p.active, 1)
		return runner, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("acquire instance for %s: %w", p.slug, ctx.Err())
	}
}

// Release returns an instance to the pool. If the pool is draining, the
// runner is closed immediately instead, and when all in-flight runners have
// been released the drain-complete signal is sent.
func (p *PluginInstancePool) Release(runner WASMRunner) {
	remaining := atomic.AddInt32(&p.active, -1)

	p.mu.Lock()
	isDraining := p.draining
	p.mu.Unlock()

	if isDraining {
		// Don't return to pool — close the runner.
		_ = runner.Close()
		p.signalDrainIfNeeded(remaining)
		return
	}

	select {
	case p.pool <- runner:
	default:
		// Pool full (shouldn't happen), close the extra instance.
		_ = runner.Close()
	}
}

// signalDrainIfNeeded checks if drain should be completed after an active
// count decrement. It is safe to call multiple times; the drained channel is
// closed at most once.
func (p *PluginInstancePool) signalDrainIfNeeded(remaining int32) {
	p.mu.Lock()
	isDraining := p.draining
	p.mu.Unlock()

	if isDraining && remaining <= 0 {
		select {
		case <-p.drained:
		default:
			close(p.drained)
		}
	}
}

// Drain stops accepting new acquires, waits for all in-flight runners to be
// released, then closes all remaining idle instances. Returns when fully
// drained or the timeout is exceeded.
func (p *PluginInstancePool) Drain(ctx context.Context) error {
	p.mu.Lock()
	if p.draining {
		p.mu.Unlock()
		return nil
	}
	p.draining = true
	p.drained = make(chan struct{})
	// Check-and-signal under the same lock that set draining=true to
	// eliminate the TOCTOU window between loading active and closing drained.
	if atomic.LoadInt32(&p.active) <= 0 {
		close(p.drained)
	}
	p.mu.Unlock()

	// Wait for in-flight runners to complete.
	drainCtx, cancel := context.WithTimeout(ctx, DrainTimeout)
	defer cancel()

	select {
	case <-p.drained:
		// All in-flight runners completed gracefully.
	case <-drainCtx.Done():
		// Timeout — force close remaining.
		slog.Warn("drain timeout exceeded, force closing remaining runners",
			slog.String("slug", p.slug),
			slog.Int("active", int(atomic.LoadInt32(&p.active))),
		)
	}

	// Close all remaining idle instances in the pool channel.
	close(p.pool)
	for runner := range p.pool {
		_ = runner.Close()
	}

	return nil
}

// Close immediately closes all idle instances without waiting for in-flight
// runners. Use Drain for graceful shutdown during hot reloads.
func (p *PluginInstancePool) Close() {
	p.mu.Lock()
	p.draining = true
	p.mu.Unlock()

	close(p.pool)
	for runner := range p.pool {
		_ = runner.Close()
	}
}

// Size returns the configured pool size.
func (p *PluginInstancePool) Size() int {
	return p.poolSize
}

// replaceInstance creates a new WASM runner via the factory and attempts to
// add it to the pool. If the pool is draining, full, or creation fails, the
// attempt is silently abandoned (the pool shrinks by one until traffic drops).
func (p *PluginInstancePool) replaceInstance() {
	if p.factory == nil {
		return
	}

	// Don't replace if the pool is draining — the channel may already be
	// closed, and sending on a closed channel would panic.
	p.mu.Lock()
	isDraining := p.draining
	p.mu.Unlock()
	if isDraining {
		return
	}

	newRunner, err := p.factory(p.wasm, p.config)
	if err != nil {
		slog.Warn("failed to replace corrupted WASM instance",
			slog.String("slug", p.slug),
			slog.String("error", err.Error()),
		)
		return
	}

	// There is an inherent race between this goroutine and Drain closing the
	// pool channel, even after the draining check above. Recover from the
	// send-on-closed-channel panic rather than adding lock contention to the
	// hot path.
	if !p.trySendToPool(newRunner) {
		_ = newRunner.Close()
	}
}

// trySendToPool attempts a non-blocking send of runner to the pool channel.
// Returns false if the pool is full or has been closed by Drain.
func (p *PluginInstancePool) trySendToPool(runner WASMRunner) (sent bool) {
	defer func() {
		if r := recover(); r != nil {
			// Pool channel was closed by Drain — this is expected during
			// concurrent drain+replace and not a bug.
			sent = false
		}
	}()

	select {
	case p.pool <- runner:
		slog.Info("replaced corrupted WASM instance", slog.String("slug", p.slug))
		return true
	default:
		return false
	}
}

// PluginInstance holds metadata for a loaded plugin. It is used by
// GetInstance callers (e.g. the dispatcher) to read manifest information
// without needing to acquire a runner.
type PluginInstance struct {
	Slug        string
	PluginID    string
	Manifest    *Manifest
	Config      map[string]string
	Permissions []PermissionScope
}

// RuntimePool manages the lifecycle of loaded WASM plugin instance pools. It
// is safe for concurrent use.
type RuntimePool struct {
	mu            sync.RWMutex
	plugins       map[string]*PluginInstancePool // keyed by plugin slug
	metadata      map[string]*PluginInstance     // keyed by plugin slug
	logger        *slog.Logger
	runnerFactory WASMRunnerFactory
}

// NewRuntimePool creates an empty runtime pool. If factory is nil, LoadPlugin
// will store metadata but skip WASM compilation (useful only in tests).
func NewRuntimePool(logger *slog.Logger, factory WASMRunnerFactory) *RuntimePool {
	return &RuntimePool{
		plugins:       make(map[string]*PluginInstancePool),
		metadata:      make(map[string]*PluginInstance),
		logger:        logger,
		runnerFactory: factory,
	}
}

// LoadPlugin creates a pool of WASM instances for the plugin and stores
// metadata. If a plugin with the same slug is already loaded, the old pool is
// drained gracefully in the background while the new pool starts serving
// immediately.
func (rp *RuntimePool) LoadPlugin(p *Plugin) error {
	if p == nil {
		return fmt.Errorf("cannot load nil plugin")
	}

	rp.mu.Lock()
	defer rp.mu.Unlock()

	// Drain existing pool gracefully in the background. The new pool is
	// installed immediately so new requests are never blocked by the drain.
	if existing, ok := rp.plugins[p.Slug]; ok {
		go existing.Drain(context.Background())
		delete(rp.plugins, p.Slug)
		delete(rp.metadata, p.Slug)
	}

	meta := &PluginInstance{
		Slug:        p.Slug,
		PluginID:    p.ID,
		Manifest:    p.Manifest,
		Config:      p.Config,
		Permissions: p.Permissions,
	}

	if rp.runnerFactory != nil && len(p.WASMBytes) > 0 {
		poolSize := DefaultPoolSize
		if p.Manifest != nil {
			poolSize = p.Manifest.EffectiveLimits().PoolSize
		}

		pool, err := newPluginInstancePool(p.Slug, rp.runnerFactory, p.WASMBytes, p.Config, p.Manifest, poolSize)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrWASMCompilationFailed, err)
		}
		rp.plugins[p.Slug] = pool
	}

	rp.metadata[p.Slug] = meta

	rp.logger.Info("plugin loaded into runtime pool",
		"slug", p.Slug,
		"id", p.ID,
		"pool_size", rp.poolSizeForSlug(p.Slug),
	)
	return nil
}

// poolSizeForSlug returns the pool size for the slug, or 0 if no pool exists.
// Must be called with at least a read lock held.
func (rp *RuntimePool) poolSizeForSlug(slug string) int {
	if pool, ok := rp.plugins[slug]; ok {
		return pool.Size()
	}
	return 0
}

// UnloadPlugin removes a plugin instance pool from the runtime and drains all
// its runners gracefully.
func (rp *RuntimePool) UnloadPlugin(slug string) error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if _, ok := rp.metadata[slug]; !ok {
		return ErrPluginNotFound
	}

	if pool, ok := rp.plugins[slug]; ok {
		go pool.Drain(context.Background())
		delete(rp.plugins, slug)
	}

	delete(rp.metadata, slug)
	rp.logger.Info("plugin unloaded from runtime pool", "slug", slug)
	return nil
}

// GetInstance returns the PluginInstance metadata for the given slug, or
// ErrPluginNotFound.
func (rp *RuntimePool) GetInstance(slug string) (*PluginInstance, error) {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	inst, ok := rp.metadata[slug]
	if !ok {
		return nil, ErrPluginNotFound
	}
	return inst, nil
}

// CallHook acquires a WASM runner from the plugin's pool, invokes the named
// function, and returns the runner to the pool. If the runner returns an error
// indicating WASM corruption (memory fault, trap, etc.), the runner is
// discarded and a replacement is created asynchronously. The context controls
// both pool acquisition timeout and function execution timeout.
func (rp *RuntimePool) CallHook(ctx context.Context, slug, funcName string, input []byte) ([]byte, error) {
	rp.mu.RLock()
	pool, poolOK := rp.plugins[slug]
	_, metaOK := rp.metadata[slug]
	rp.mu.RUnlock()

	if !metaOK {
		return nil, ErrPluginNotFound
	}

	if !poolOK || pool == nil {
		return nil, fmt.Errorf("plugin %q has no WASM runner", slug)
	}

	// Honour context deadline/cancellation before acquiring.
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: %v", ErrHookTimeout, ctx.Err())
	default:
	}

	runner, err := pool.Acquire(ctx)
	if err != nil {
		if errors.Is(err, ErrPluginDraining) {
			return nil, err
		}
		return nil, fmt.Errorf("acquire runner: %w", err)
	}

	output, callErr := runner.Call(ctx, funcName, input)
	if callErr != nil {
		// Check if the runner is corrupted — if so, discard it and replace.
		if isRunnerCorrupted(callErr) {
			_ = runner.Close()
			// Decrement active count manually since we are NOT calling Release.
			remaining := atomic.AddInt32(&pool.active, -1)
			// Signal drain if this was the last active runner.
			pool.signalDrainIfNeeded(remaining)
			// Create a replacement runner in the background.
			go pool.replaceInstance()

			rp.logger.Warn("corrupted WASM runner discarded",
				slog.String("slug", slug),
				slog.String("func", funcName),
				slog.String("error", callErr.Error()),
			)
			return nil, fmt.Errorf("plugin %s runner corrupted: %w", slug, callErr)
		}

		// Non-corruption error — return runner to pool normally.
		pool.Release(runner)

		// Check if the context was cancelled while executing.
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%w: %v", ErrHookTimeout, ctx.Err())
		}
		return nil, callErr
	}

	pool.Release(runner)
	return output, nil
}

// isRunnerCorrupted inspects an error from a WASM runner call and returns true
// if the error indicates the runner is in a corrupted state and must not be
// reused. Context cancellation/timeout errors are NOT treated as corruption
// since the runner itself may still be healthy.
func isRunnerCorrupted(err error) bool {
	// Context errors are not corruption — the runner may still be fine.
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}
	errMsg := strings.ToLower(err.Error())
	for _, indicator := range wasmCorruptionIndicators {
		if strings.Contains(errMsg, indicator) {
			return true
		}
	}
	return false
}

// SetRunnerForTest replaces the entire pool for the given plugin slug with a
// single-instance pool containing the provided runner. This is intended for
// use in tests that need to inject a mock runner after LoadPlugin.
func (rp *RuntimePool) SetRunnerForTest(slug string, runner WASMRunner) {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if _, ok := rp.metadata[slug]; !ok {
		return
	}

	// Close existing pool if present.
	if existing, ok := rp.plugins[slug]; ok {
		existing.Close()
	}

	// Create a single-slot pool with the test runner.
	pool := make(chan WASMRunner, 1)
	pool <- runner
	rp.plugins[slug] = &PluginInstancePool{
		slug:     slug,
		pool:     pool,
		poolSize: 1,
		drained:  make(chan struct{}),
	}
}

// LoadedSlugs returns the slugs of all currently loaded plugins.
func (rp *RuntimePool) LoadedSlugs() []string {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	slugs := make([]string, 0, len(rp.metadata))
	for slug := range rp.metadata {
		slugs = append(slugs, slug)
	}
	return slugs
}
