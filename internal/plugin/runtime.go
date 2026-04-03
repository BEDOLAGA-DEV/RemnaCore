package plugin

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

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
type PluginInstancePool struct {
	slug     string
	factory  WASMRunnerFactory
	wasm     []byte
	config   map[string]string
	manifest *Manifest
	pool     chan WASMRunner
	poolSize int
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
// the context is cancelled/timed out.
func (p *PluginInstancePool) Acquire(ctx context.Context) (WASMRunner, error) {
	select {
	case runner := <-p.pool:
		return runner, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("acquire instance for %s: %w", p.slug, ctx.Err())
	}
}

// Release returns an instance to the pool. If the pool channel is
// unexpectedly full (which should not happen during normal operation), the
// extra instance is closed instead.
func (p *PluginInstancePool) Release(runner WASMRunner) {
	select {
	case p.pool <- runner:
	default:
		// Pool full (shouldn't happen), close the extra instance.
		_ = runner.Close()
	}
}

// Close drains and shuts down all instances in the pool.
func (p *PluginInstancePool) Close() {
	close(p.pool)
	for runner := range p.pool {
		_ = runner.Close()
	}
}

// Size returns the configured pool size.
func (p *PluginInstancePool) Size() int {
	return p.poolSize
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
// metadata. If a plugin with the same slug is already loaded it is unloaded
// first.
func (rp *RuntimePool) LoadPlugin(p *Plugin) error {
	if p == nil {
		return fmt.Errorf("cannot load nil plugin")
	}

	rp.mu.Lock()
	defer rp.mu.Unlock()

	// Unload existing pool for this slug if present.
	if existing, ok := rp.plugins[p.Slug]; ok {
		existing.Close()
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

// UnloadPlugin removes a plugin instance pool from the runtime and closes all
// its runners.
func (rp *RuntimePool) UnloadPlugin(slug string) error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if _, ok := rp.metadata[slug]; !ok {
		return ErrPluginNotFound
	}

	if pool, ok := rp.plugins[slug]; ok {
		pool.Close()
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
// function, and returns the runner to the pool. The context controls
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
		return nil, fmt.Errorf("%w: %v", ErrHookTimeout, err)
	}
	defer pool.Release(runner)

	output, err := runner.Call(ctx, funcName, input)
	if err != nil {
		// Check if the context was cancelled while executing.
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%w: %v", ErrHookTimeout, ctx.Err())
		}
		return nil, err
	}
	return output, nil
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
