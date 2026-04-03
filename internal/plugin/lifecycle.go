package plugin

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// LifecycleManager orchestrates all plugin state transitions: install, enable,
// disable, uninstall, and configuration updates. It is the single source of
// truth for plugin lifecycle operations.
type LifecycleManager struct {
	repo       PluginRepository
	storage    StorageService
	runtime    *RuntimePool
	dispatcher *HookDispatcher
	publisher  domainevent.Publisher
	logger     *slog.Logger
}

// NewLifecycleManager creates a LifecycleManager with all required
// dependencies.
func NewLifecycleManager(
	repo PluginRepository,
	storage StorageService,
	runtime *RuntimePool,
	dispatcher *HookDispatcher,
	publisher domainevent.Publisher,
	logger *slog.Logger,
) *LifecycleManager {
	return &LifecycleManager{
		repo:       repo,
		storage:    storage,
		runtime:    runtime,
		dispatcher: dispatcher,
		publisher:  publisher,
		logger:     logger,
	}
}

// Install parses a manifest, validates uniqueness, persists the plugin, and
// publishes a plugin.installed event. The plugin starts in StatusInstalled
// (not yet running).
func (lm *LifecycleManager) Install(ctx context.Context, manifestBytes, wasmBytes []byte) (*Plugin, error) {
	manifest, err := ParseManifest(manifestBytes)
	if err != nil {
		return nil, err
	}

	// Check slug uniqueness.
	existing, err := lm.repo.GetBySlug(ctx, manifest.Plugin.ID)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("%w: slug %q", ErrPluginAlreadyExists, manifest.Plugin.ID)
	}

	p, err := NewPlugin(manifest, wasmBytes)
	if err != nil {
		return nil, err
	}

	if err := lm.repo.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("persisting plugin: %w", err)
	}

	if lm.publisher != nil {
		_ = lm.publisher.Publish(ctx, NewPluginInstalledEvent(p.ID, p.Slug, p.Version))
	}

	lm.logger.Info("plugin installed", "slug", p.Slug, "id", p.ID)
	return p, nil
}

// Enable transitions a plugin from installed/disabled to enabled: loads WASM
// into the runtime pool, registers hooks in the dispatcher, and persists the
// new status.
func (lm *LifecycleManager) Enable(ctx context.Context, pluginID string) error {
	p, err := lm.repo.GetByID(ctx, pluginID)
	if err != nil {
		return fmt.Errorf("get plugin for enable: %w", err)
	}

	if err := p.Enable(); err != nil {
		return fmt.Errorf("transition plugin to enabled: %w", err)
	}

	// Load into runtime pool.
	if err := lm.runtime.LoadPlugin(p); err != nil {
		return fmt.Errorf("loading plugin into runtime: %w", err)
	}

	// Register hooks in dispatcher.
	if p.Manifest != nil {
		regs := p.Manifest.HookRegistrations(p.ID)
		lm.dispatcher.RegisterHooks(regs)
	}

	// Persist status.
	if err := lm.repo.UpdateStatus(ctx, p.ID, p.Status, "", p.EnabledAt); err != nil {
		return fmt.Errorf("persisting enabled status: %w", err)
	}

	if lm.publisher != nil {
		_ = lm.publisher.Publish(ctx, NewPluginEnabledEvent(p.ID, p.Slug))
	}

	lm.logger.Info("plugin enabled", "slug", p.Slug, "id", p.ID)
	return nil
}

// Disable unregisters hooks, unloads WASM from the runtime, and persists the
// disabled status.
func (lm *LifecycleManager) Disable(ctx context.Context, pluginID string) error {
	p, err := lm.repo.GetByID(ctx, pluginID)
	if err != nil {
		return fmt.Errorf("get plugin for disable: %w", err)
	}

	// Unregister hooks first.
	lm.dispatcher.UnregisterHooks(p.Slug)

	// Unload from runtime pool (ignore not-found if not loaded).
	if unloadErr := lm.runtime.UnloadPlugin(p.Slug); unloadErr != nil && unloadErr != ErrPluginNotFound {
		return fmt.Errorf("unloading plugin from runtime: %w", unloadErr)
	}

	p.Disable()

	if err := lm.repo.UpdateStatus(ctx, p.ID, p.Status, "", nil); err != nil {
		return fmt.Errorf("persisting disabled status: %w", err)
	}

	if lm.publisher != nil {
		_ = lm.publisher.Publish(ctx, NewPluginDisabledEvent(p.ID, p.Slug))
	}

	lm.logger.Info("plugin disabled", "slug", p.Slug, "id", p.ID)
	return nil
}

// Uninstall disables a plugin (if enabled), deletes all its storage, removes
// the registry record, and publishes a plugin.uninstalled event.
func (lm *LifecycleManager) Uninstall(ctx context.Context, pluginID string) error {
	p, err := lm.repo.GetByID(ctx, pluginID)
	if err != nil {
		return fmt.Errorf("get plugin for uninstall: %w", err)
	}

	// Disable if currently enabled.
	if p.Status == StatusEnabled {
		if err := lm.Disable(ctx, pluginID); err != nil {
			return fmt.Errorf("disabling plugin before uninstall: %w", err)
		}
	}

	// Delete all plugin storage.
	if lm.storage != nil {
		if err := lm.storage.DeleteAll(ctx, p.Slug); err != nil {
			lm.logger.Warn("failed to delete plugin storage during uninstall",
				"slug", p.Slug, "error", err)
		}
	}

	// Delete from repository.
	if err := lm.repo.Delete(ctx, p.ID); err != nil {
		return fmt.Errorf("deleting plugin record: %w", err)
	}

	if lm.publisher != nil {
		_ = lm.publisher.Publish(ctx, NewPluginUninstalledEvent(p.ID, p.Slug))
	}

	lm.logger.Info("plugin uninstalled", "slug", p.Slug, "id", p.ID)
	return nil
}

// UpdateConfig validates the new configuration against the manifest schema,
// persists it, and performs a hot-reload if the plugin is currently enabled.
func (lm *LifecycleManager) UpdateConfig(ctx context.Context, pluginID string, config map[string]string) error {
	p, err := lm.repo.GetByID(ctx, pluginID)
	if err != nil {
		return fmt.Errorf("get plugin for config update: %w", err)
	}

	// Validate required config fields against manifest schema.
	if p.Manifest != nil {
		for key, field := range p.Manifest.Config {
			if field.Required {
				if _, ok := config[key]; !ok {
					return fmt.Errorf("%w: missing required config key %q", ErrInvalidManifest, key)
				}
			}
		}
	}

	if err := lm.repo.UpdateConfig(ctx, p.ID, config); err != nil {
		return fmt.Errorf("persisting plugin config: %w", err)
	}

	// Hot-reload if the plugin is currently enabled.
	if p.Status == StatusEnabled {
		if err := lm.Disable(ctx, pluginID); err != nil {
			return fmt.Errorf("hot-reload disable: %w", err)
		}
		if err := lm.Enable(ctx, pluginID); err != nil {
			return fmt.Errorf("hot-reload enable: %w", err)
		}
	}

	lm.logger.Info("plugin config updated", "slug", p.Slug, "id", p.ID)
	return nil
}

// LoadAllEnabled loads all plugins with status=enabled from the database into
// the runtime pool. Called on application startup to recover state.
func (lm *LifecycleManager) LoadAllEnabled(ctx context.Context) error {
	plugins, err := lm.repo.GetEnabled(ctx)
	if err != nil {
		return fmt.Errorf("loading enabled plugins: %w", err)
	}

	for _, p := range plugins {
		if err := lm.runtime.LoadPlugin(p); err != nil {
			lm.logger.Error("failed to load enabled plugin on startup",
				"slug", p.Slug, "id", p.ID, "error", err)
			continue
		}

		if p.Manifest != nil {
			regs := p.Manifest.HookRegistrations(p.ID)
			lm.dispatcher.RegisterHooks(regs)
		}

		lm.logger.Info("loaded enabled plugin on startup", "slug", p.Slug)
	}

	lm.logger.Info("all enabled plugins loaded", "count", len(plugins))
	return nil
}
