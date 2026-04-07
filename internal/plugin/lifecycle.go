package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// computeWASMHash returns the SHA-256 hex digest of the given WASM bytes, or
// an empty string if the input is nil/empty.
func computeWASMHash(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// LifecycleManager orchestrates all plugin state transitions: install, enable,
// disable, uninstall, and configuration updates. It is the single source of
// truth for plugin lifecycle operations.
type LifecycleManager struct {
	repo           PluginRepository
	storage        StorageService
	runtime        *RuntimePool
	dispatcher     *HookDispatcher
	hostFunctions  *HostFunctions
	publisher      domainevent.Publisher
	logger         *slog.Logger
	clock          clock.Clock
}

// NewLifecycleManager creates a LifecycleManager with all required
// dependencies.
func NewLifecycleManager(
	repo PluginRepository,
	storage StorageService,
	runtime *RuntimePool,
	dispatcher *HookDispatcher,
	hostFunctions *HostFunctions,
	publisher domainevent.Publisher,
	logger *slog.Logger,
	clk clock.Clock,
) *LifecycleManager {
	return &LifecycleManager{
		repo:          repo,
		storage:       storage,
		runtime:       runtime,
		dispatcher:    dispatcher,
		hostFunctions: hostFunctions,
		publisher:     publisher,
		logger:        logger,
		clock:         clk,
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

	// Verify SDK version compatibility before any persistence.
	if err := checkSDKCompatibility(manifest.Plugin.SDKVersion); err != nil {
		return nil, fmt.Errorf("sdk version check: %w", err)
	}

	// Check slug uniqueness.
	existing, err := lm.repo.GetBySlug(ctx, manifest.Plugin.ID)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("%w: slug %q", ErrPluginAlreadyExists, manifest.Plugin.ID)
	}

	// Enforce WASM binary size limit before any persistence.
	if len(wasmBytes) > MaxWASMBinarySize {
		return nil, fmt.Errorf("%w: %d bytes (max %d)", ErrWASMBinaryTooLarge, len(wasmBytes), MaxWASMBinarySize)
	}

	p, err := NewPlugin(manifest, wasmBytes, lm.clock.Now())
	if err != nil {
		return nil, err
	}

	// Content-addressable WASM storage: store binary once, reference by hash.
	if p.WASMHash != "" {
		if err := lm.repo.StoreWASM(ctx, p.WASMHash, wasmBytes); err != nil {
			return nil, fmt.Errorf("storing WASM binary: %w", err)
		}
		// Clear inline bytes — the repo stores the hash reference only.
		p.WASMBytes = nil
	}

	if err := lm.repo.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("persisting plugin: %w", err)
	}

	if lm.publisher != nil {
		if err := lm.publisher.Publish(ctx, NewPluginInstalledEvent(p.ID, p.Slug, p.Version, lm.clock.Now())); err != nil {
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

	// Verify SDK version compatibility before enabling.
	if p.Manifest != nil {
		if err := checkSDKCompatibility(p.Manifest.Plugin.SDKVersion); err != nil {
			return fmt.Errorf("sdk version check: %w", err)
		}
	}

	// Validate required config keys before enabling.
	if p.Manifest != nil {
		if err := validateRequiredConfig(p.Manifest.Config, p.Config); err != nil {
			return fmt.Errorf("config validation: %w", err)
		}
	}

	// Resolve WASM bytes from content-addressable store if not inline.
	if p.WASMBytes == nil && p.WASMHash != "" {
		wasm, err := lm.repo.GetWASMByHash(ctx, p.WASMHash)
		if err != nil {
			return fmt.Errorf("loading WASM from content store: %w", err)
		}
		p.WASMBytes = wasm
	}

	if err := p.Enable(lm.clock.Now()); err != nil {
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
		if err := lm.publisher.Publish(ctx, NewPluginEnabledEvent(p.ID, p.Slug, lm.clock.Now())); err != nil {
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
	if unloadErr := lm.runtime.UnloadPlugin(p.Slug); unloadErr != nil && !errors.Is(unloadErr, ErrPluginNotFound) {
		return fmt.Errorf("unloading plugin from runtime: %w", unloadErr)
	}

	// Remove stale HTTP rate limiter so a re-enable picks up fresh limits.
	if lm.hostFunctions != nil {
		lm.hostFunctions.ClearPluginHTTPLimiter(p.Slug)
	}

	if err := p.Disable(lm.clock.Now()); err != nil {
		return fmt.Errorf("transition plugin to disabled: %w", err)
	}

	if err := lm.repo.UpdateStatus(ctx, p.ID, p.Status, "", nil); err != nil {
		return fmt.Errorf("persisting disabled status: %w", err)
	}

	if lm.publisher != nil {
		if err := lm.publisher.Publish(ctx, NewPluginDisabledEvent(p.ID, p.Slug, lm.clock.Now())); err != nil {
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
		if err := lm.publisher.Publish(ctx, NewPluginUninstalledEvent(p.ID, p.Slug, lm.clock.Now())); err != nil {
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
		if err := validateRequiredConfig(p.Manifest.Config, config); err != nil {
			return fmt.Errorf("config validation: %w", err)
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

	// 3. Verify SDK version compatibility of new manifest.
	if err := checkSDKCompatibility(newManifest.Plugin.SDKVersion); err != nil {
		return fmt.Errorf("sdk version check: %w", err)
	}

	// 4. Verify slug matches — cannot change identity during hot reload.
	if newManifest.Plugin.ID != old.Slug {
		return fmt.Errorf("%w: expected %q, got %q", ErrSlugMismatch, old.Slug, newManifest.Plugin.ID)
	}

	// Validate required config keys from the NEW manifest against existing config.
	if err := validateRequiredConfig(newManifest.Config, old.Config); err != nil {
		return fmt.Errorf("config validation: %w", err)
	}

	// Enforce WASM binary size limit before any persistence.
	if len(wasmBytes) > MaxWASMBinarySize {
		return fmt.Errorf("%w: %d bytes (max %d)", ErrWASMBinaryTooLarge, len(wasmBytes), MaxWASMBinarySize)
	}

	oldVersion := old.Version

	// Cache old WASM bytes once for potential rollback. This avoids redundant
	// CAS fetches in each rollback path and guarantees the bytes are available
	// even if the CAS store becomes temporarily unreachable mid-reload.
	var oldWASMBytes []byte
	if old.WASMHash != "" {
		oldWASMBytes, err = lm.repo.GetWASMByHash(ctx, old.WASMHash)
		if err != nil {
			return fmt.Errorf("fetch old WASM for rollback safety: %w", err)
		}
	}

	// Content-addressable WASM storage for the new version.
	wasmHash := computeWASMHash(wasmBytes)
	if wasmHash != "" {
		if err := lm.repo.StoreWASM(ctx, wasmHash, wasmBytes); err != nil {
			return fmt.Errorf("storing WASM binary for hot reload: %w", err)
		}
	}

	// Keep a copy of wasmBytes for the runtime (LoadPlugin needs it),
	// but clear inline bytes on the persisted struct below after building it.

	// 4. Build updated plugin (preserving ID, config, enabled state).
	now := lm.clock.Now()
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
		WASMHash:    wasmHash,
		Manifest:    newManifest,
		Status:      StatusEnabled,
		Config:      old.Config,
		Permissions: newManifest.ParsePermissions(),
		InstalledAt: old.InstalledAt,
		EnabledAt:   old.EnabledAt,
		UpdatedAt:   now,
	}

	// 5. Detach the old pool before loading the new version so we can drain
	// it explicitly after the swap (rather than fire-and-forget).
	oldPool := lm.runtime.DetachPool(old.Slug)

	// Clear stale HTTP rate limiter so the new manifest's limits take effect.
	if lm.hostFunctions != nil {
		lm.hostFunctions.ClearPluginHTTPLimiter(old.Slug)
	}

	// 6. Pre-compile new WASM and install as the active pool.
	if err := lm.runtime.LoadPlugin(updated); err != nil {
		// Rollback: re-install old plugin using cached WASM bytes.
		if oldPool != nil {
			if oldWASMBytes != nil {
				old.WASMBytes = oldWASMBytes
			}
			if loadErr := lm.runtime.LoadPlugin(old); loadErr != nil {
				lm.logger.Error("failed to restore old pool after load failure",
					slog.String("slug", old.Slug),
					slog.String("error", loadErr.Error()),
				)
			}
		}
		return fmt.Errorf("load new plugin version: %w", err)
	}

	// Clear inline bytes now that the runtime has the WASM compiled. This
	// prevents double-storing the binary (inline in plugin_registry + CAS).
	updated.WASMBytes = nil

	// 7. Atomic hook swap: replace old hooks with new hooks under a single lock.
	newRegs := newManifest.HookRegistrations(old.ID)
	lm.dispatcher.SwapHooks(old.Slug, newRegs)

	// 8. Persist the updated plugin to database.
	if err := lm.repo.UpdatePlugin(ctx, updated); err != nil {
		// Rollback: atomically swap back to old hooks and reload old WASM
		// using cached bytes.
		var oldRegs []HookRegistration
		if old.Manifest != nil {
			oldRegs = old.Manifest.HookRegistrations(old.ID)
		}
		lm.dispatcher.SwapHooks(old.Slug, oldRegs)

		if oldWASMBytes != nil {
			old.WASMBytes = oldWASMBytes
		}
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

	// 9. Drain old pool: block new acquires, wait for in-flight runners to
	// complete, then close them. This ensures zero dropped requests.
	if oldPool != nil {
		if drainErr := oldPool.Drain(ctx); drainErr != nil {
			lm.logger.Warn("old pool drain returned error during hot reload",
				slog.String("slug", old.Slug),
				slog.String("error", drainErr.Error()),
			)
		}
	}

	// 10. Publish event.
	if lm.publisher != nil {
		if err := lm.publisher.Publish(ctx, NewPluginHotReloadedEvent(updated.ID, updated.Slug, oldVersion, updated.Version, lm.clock.Now())); err != nil {
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
// the runtime pool. Called on application startup to recover state. After
// loading enabled plugins, it attempts best-effort recovery of plugins stuck
// in StatusError.
func (lm *LifecycleManager) LoadAllEnabled(ctx context.Context) error {
	plugins, err := lm.repo.GetEnabled(ctx)
	if err != nil {
		return fmt.Errorf("loading enabled plugins: %w", err)
	}

	for _, p := range plugins {
		if p.Manifest != nil {
			if err := checkSDKCompatibility(p.Manifest.Plugin.SDKVersion); err != nil {
				lm.logger.Warn("skipping incompatible plugin",
					slog.String("slug", p.Slug),
					slog.String("sdk_version", p.Manifest.Plugin.SDKVersion),
					slog.String("error", err.Error()),
				)
				if statusErr := lm.repo.UpdateStatus(ctx, p.ID, StatusError, "incompatible SDK version: "+err.Error(), nil); statusErr != nil {
					lm.logger.Error("failed to set plugin error status",
						slog.String("slug", p.Slug),
						slog.Any("error", statusErr),
					)
				}
				continue
			}
		}

		// Resolve WASM bytes from content-addressable store if not inline.
		if p.WASMBytes == nil && p.WASMHash != "" {
			wasm, err := lm.repo.GetWASMByHash(ctx, p.WASMHash)
			if err != nil {
				lm.logger.Warn("skipping plugin: WASM not found in content store",
					slog.String("slug", p.Slug),
					slog.String("wasm_hash", p.WASMHash),
					slog.String("error", err.Error()),
				)
				continue
			}
			p.WASMBytes = wasm
		}

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

	// Attempt best-effort recovery of plugins stuck in StatusError.
	if recovered := lm.RecoverErrorPlugins(ctx); recovered > 0 {
		lm.logger.Info("recovered error plugins on startup", "count", recovered)
	}

	return nil
}

// RecoverErrorPlugins attempts to recover plugins in StatusError by re-enabling
// them using their last known good WASM hash. Called during startup after
// loading enabled plugins. Recovery is best-effort: individual failures are
// logged and skipped, never propagated.
func (lm *LifecycleManager) RecoverErrorPlugins(ctx context.Context) int {
	plugins, err := lm.repo.GetByStatus(ctx, StatusError)
	if err != nil {
		lm.logger.Warn("failed to fetch error plugins for recovery",
			slog.String("error", err.Error()),
		)
		return 0
	}

	if len(plugins) == 0 {
		return 0
	}

	lm.logger.Info("attempting recovery of error plugins", "count", len(plugins))

	var recovered int
	for _, p := range plugins {
		if p.WASMHash == "" {
			lm.logger.Warn("cannot recover plugin without WASM hash",
				slog.String("slug", p.Slug),
				slog.String("error_log", p.ErrorLog),
			)
			continue
		}

		if err := lm.Enable(ctx, p.ID); err != nil {
			lm.logger.Warn("recovery failed for error plugin",
				slog.String("slug", p.Slug),
				slog.String("previous_error", p.ErrorLog),
				slog.String("error", err.Error()),
			)
			continue
		}

		lm.logger.Info("recovered error plugin",
			slog.String("slug", p.Slug),
			slog.String("previous_error", p.ErrorLog),
		)
		recovered++
	}

	return recovered
}

// validateRequiredConfig checks that all required config fields from the
// manifest schema are present and non-empty in the provided config map.
func validateRequiredConfig(schema map[string]ManifestConfigField, config map[string]string) error {
	for key, field := range schema {
		if field.Required {
			val, ok := config[key]
			if !ok || val == "" {
				return fmt.Errorf("%w: missing required config key %q", ErrMissingConfig, key)
			}
		}
	}
	return nil
}
