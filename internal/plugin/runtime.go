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

// PluginInstance represents a loaded plugin in the runtime pool.
type PluginInstance struct {
	Slug        string
	PluginID    string
	WASMModule  []byte
	Manifest    *Manifest
	Config      map[string]string
	Permissions []PermissionScope
	Runner      WASMRunner
}

// RuntimePool manages the lifecycle of loaded WASM plugin instances. It is
// safe for concurrent use.
type RuntimePool struct {
	mu            sync.RWMutex
	plugins       map[string]*PluginInstance // keyed by plugin slug
	logger        *slog.Logger
	runnerFactory WASMRunnerFactory
}

// NewRuntimePool creates an empty runtime pool. If factory is nil, CallHook
// will return an error for any plugin that has not had a runner manually
// assigned (useful only in tests).
func NewRuntimePool(logger *slog.Logger, factory WASMRunnerFactory) *RuntimePool {
	return &RuntimePool{
		plugins:       make(map[string]*PluginInstance),
		logger:        logger,
		runnerFactory: factory,
	}
}

// LoadPlugin compiles a plugin's WASM bytes (via the factory) and stores the
// instance in the pool. If a plugin with the same slug is already loaded it is
// unloaded first.
func (rp *RuntimePool) LoadPlugin(p *Plugin) error {
	if p == nil {
		return fmt.Errorf("cannot load nil plugin")
	}

	rp.mu.Lock()
	defer rp.mu.Unlock()

	// Unload existing instance for this slug if present.
	if existing, ok := rp.plugins[p.Slug]; ok {
		if existing.Runner != nil {
			_ = existing.Runner.Close()
		}
		delete(rp.plugins, p.Slug)
	}

	var runner WASMRunner
	if rp.runnerFactory != nil && len(p.WASMBytes) > 0 {
		var err error
		runner, err = rp.runnerFactory(p.WASMBytes, p.Config)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrWASMCompilationFailed, err)
		}
	}

	rp.plugins[p.Slug] = &PluginInstance{
		Slug:        p.Slug,
		PluginID:    p.ID,
		WASMModule:  p.WASMBytes,
		Manifest:    p.Manifest,
		Config:      p.Config,
		Permissions: p.Permissions,
		Runner:      runner,
	}

	rp.logger.Info("plugin loaded into runtime pool", "slug", p.Slug, "id", p.ID)
	return nil
}

// UnloadPlugin removes a plugin instance from the pool and closes its runner.
func (rp *RuntimePool) UnloadPlugin(slug string) error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	inst, ok := rp.plugins[slug]
	if !ok {
		return ErrPluginNotFound
	}

	if inst.Runner != nil {
		_ = inst.Runner.Close()
	}

	delete(rp.plugins, slug)
	rp.logger.Info("plugin unloaded from runtime pool", "slug", slug)
	return nil
}

// GetInstance returns the PluginInstance for the given slug, or
// ErrPluginNotFound.
func (rp *RuntimePool) GetInstance(slug string) (*PluginInstance, error) {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	inst, ok := rp.plugins[slug]
	if !ok {
		return nil, ErrPluginNotFound
	}
	return inst, nil
}

// CallHook invokes an exported WASM function on the plugin identified by slug.
// The context controls timeout/cancellation.
func (rp *RuntimePool) CallHook(ctx context.Context, slug, funcName string, input []byte) ([]byte, error) {
	rp.mu.RLock()
	inst, ok := rp.plugins[slug]
	rp.mu.RUnlock()

	if !ok {
		return nil, ErrPluginNotFound
	}

	if inst.Runner == nil {
		return nil, fmt.Errorf("plugin %q has no WASM runner", slug)
	}

	// Honour context deadline/cancellation.
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: %v", ErrHookTimeout, ctx.Err())
	default:
	}

	output, err := inst.Runner.Call(ctx, funcName, input)
	if err != nil {
		// Check if the context was cancelled while executing.
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%w: %v", ErrHookTimeout, ctx.Err())
		}
		return nil, err
	}
	return output, nil
}

// SetRunnerForTest replaces the WASMRunner for the given plugin slug. This is
// intended for use in tests that need to inject a mock runner after LoadPlugin.
func (rp *RuntimePool) SetRunnerForTest(slug string, runner WASMRunner) {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if inst, ok := rp.plugins[slug]; ok {
		inst.Runner = runner
	}
}

// LoadedSlugs returns the slugs of all currently loaded plugins.
func (rp *RuntimePool) LoadedSlugs() []string {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	slugs := make([]string, 0, len(rp.plugins))
	for slug := range rp.plugins {
		slugs = append(slugs, slug)
	}
	return slugs
}
