package plugin

import (
	"context"
	"fmt"
	"log/slog"
	"time"

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
		if err := lm.publisher.Publish(ctx, NewPluginInstalledEvent(p.ID, p.Slug, p.Version)); err != nil {
			lm.logger.Warn("failed to publish event",
				"event_type", string(EventPluginInstalled),
				"error", err.Error(),
			)
		}
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
		if err := lm.publisher.Publish(ctx, NewPluginEnabledEvent(p.ID, p.Slug)); err != nil {
			lm.logger.Warn("failed to publish event",
				"event_type", string(EventPluginEnabled),
				"error", err.Error(),
			)
		}
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
		if err := lm.publisher.Publish(ctx, NewPluginDisabledEvent(p.ID, p.Slug)); err != nil {
			lm.logger.Warn("failed to publish event",
				"event_type", string(EventPluginDisabled),
				"error", err.Error(),
			)
		}
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
		if err := lm.publisher.Publish(ctx, NewPluginUninstalledEvent(p.ID, p.Slug)); err != nil {
			lm.logger.Warn("failed to publish event",
				"event_type", string(EventPluginUninstalled),
				"error", err.Error(),
			)
		}
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

// HotReload atomically replaces a running plugin with a new version. The old
// version continues serving hooks until the new one is ready, ensuring zero
// downtime. The plugin must be in StatusEnabled. The slug (plugin.id) must
// match between old and new manifests — identity cannot change during reload.
func (lm *LifecycleManager) HotReload(ctx context.Context, pluginID string, manifestBytes, wasmBytes []byte) error {
	// 1. Fetch current plugin.
	old, err := lm.repo.GetByID(ctx, pluginID)
	if err != nil {
		return fmt.Errorf("get plugin for hot reload: %w", err)
	}

	if old.Status != StatusEnabled {
		return fmt.Errorf("hot reload requires enabled plugin: %w", ErrPluginNotRunning)
	}

	// 2. Parse and validate new manifest.
	newManifest, err := ParseManifest(manifestBytes)
	if err != nil {
		return fmt.Errorf("parse new manifest: %w", err)
	}

	// 3. Verify slug matches — cannot change identity during hot reload.
	if newManifest.Plugin.ID != old.Slug {
		return fmt.Errorf("%w: expected %q, got %q", ErrSlugMismatch, old.Slug, newManifest.Plugin.ID)
	}

	oldVersion := old.Version

	// 4. Build updated plugin (preserving ID, config, enabled state).
	now := time.Now()
	updated := &Plugin{
		ID:          old.ID,
		Slug:        old.Slug,
		Name:        newManifest.Plugin.Name,
		Version:     newManifest.Plugin.Version,
		Description: newManifest.Plugin.Description,
		Author:      newManifest.Plugin.Author,
		License:     newManifest.Plugin.License,
		SDKVersion:  newManifest.Plugin.SDKVersion,
		Lang:        newManifest.Plugin.Lang,
		WASMBytes:   wasmBytes,
		Manifest:    newManifest,
		Status:      StatusEnabled,
		Config:      old.Config,
		Permissions: newManifest.ParsePermissions(),
		InstalledAt: old.InstalledAt,
		EnabledAt:   old.EnabledAt,
		UpdatedAt:   now,
	}

	// 5. Pre-compile new WASM while old version is still serving.
	if err := lm.runtime.LoadPlugin(updated); err != nil {
		return fmt.Errorf("load new plugin version: %w", err)
	}

	// 6. Atomic hook swap: replace old hooks with new hooks under a single lock.
	newRegs := newManifest.HookRegistrations(old.ID)
	lm.dispatcher.SwapHooks(old.Slug, newRegs)

	// 7. Persist the updated plugin to database.
	if err := lm.repo.UpdatePlugin(ctx, updated); err != nil {
		// Rollback: atomically swap back to old hooks and reload old WASM.
		var oldRegs []HookRegistration
		if old.Manifest != nil {
			oldRegs = old.Manifest.HookRegistrations(old.ID)
		}
		lm.dispatcher.SwapHooks(old.Slug, oldRegs)

		if loadErr := lm.runtime.LoadPlugin(old); loadErr != nil {
			lm.logger.Error("hot reload rollback failed — plugin in broken state",
				slog.String("slug", old.Slug),
				slog.String("error", loadErr.Error()),
			)
			// Set plugin to error state in DB so operators can see it is broken.
			reason := "hot reload rollback failed: " + loadErr.Error()
			if statusErr := lm.repo.UpdateStatus(ctx, old.ID, StatusError, reason, nil); statusErr != nil {
				lm.logger.Error("failed to set plugin error status after rollback failure",
					slog.String("slug", old.Slug),
					slog.String("error", statusErr.Error()),
				)
			}
		}
		return fmt.Errorf("persist hot reload: %w", err)
	}

	// 8. Publish event.
	if lm.publisher != nil {
		if err := lm.publisher.Publish(ctx, NewPluginHotReloadedEvent(updated.ID, updated.Slug, oldVersion, updated.Version)); err != nil {
			lm.logger.Warn("failed to publish event",
				"event_type", string(EventPluginHotReloaded),
				"error", err.Error(),
			)
		}
	}

	lm.logger.Info("plugin hot reloaded",
		slog.String("slug", old.Slug),
		slog.String("old_version", oldVersion),
		slog.String("new_version", newManifest.Plugin.Version),
	)

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
