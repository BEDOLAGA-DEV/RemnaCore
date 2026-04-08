package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
)

// seedBuiltInPlugins ensures every built-in plugin definition exists in the
// database. If a built-in plugin is already present (matched by slug), it is
// skipped. For new entries the Remnawave connection config is seeded from env
// vars as a migration path from the old env-only configuration.
func seedBuiltInPlugins(repo plugin.PluginRepository, cfg *config.Config, logger *slog.Logger) error {
	ctx := context.Background()

	for _, def := range plugin.BuiltInPlugins() {
		existing, err := repo.GetBySlug(ctx, def.Slug)
		if err == nil && existing != nil {
			logger.Debug("built-in plugin already seeded", slog.String("slug", def.Slug))
			continue
		}

		now := time.Now()

		p := &plugin.Plugin{
			ID:          uuid.Must(uuid.NewV7()).String(),
			Slug:        def.Slug,
			Name:        def.Name,
			Version:     def.Version,
			Description: def.Description,
			Author:      def.Author,
			IsBuiltIn:   true,
			Status:      plugin.StatusInstalled,
			Config:      make(map[string]string),
			Manifest:    buildBuiltInManifest(def),
			InstalledAt: now,
			UpdatedAt:   now,
		}

		// Seed config from env vars if available (migration path from
		// env-only configuration to plugin-managed configuration).
		seedRemnawaveConfig(p, cfg)

		if err := repo.Create(ctx, p); err != nil {
			return fmt.Errorf("seed built-in plugin %s: %w", def.Slug, err)
		}

		logger.Info("seeded built-in plugin",
			slog.String("slug", def.Slug),
			slog.String("id", p.ID),
		)
	}

	return nil
}

// seedRemnawaveConfig populates the plugin config from env-based Remnawave
// settings when they are set, providing a smooth migration path.
func seedRemnawaveConfig(p *plugin.Plugin, cfg *config.Config) {
	if cfg.Remnawave.URL != "" {
		p.Config[plugin.RemnawaveConfigKeyURL] = cfg.Remnawave.URL
	}
	if cfg.Remnawave.APIToken.Expose() != "" {
		p.Config[plugin.RemnawaveConfigKeyAPIToken] = cfg.Remnawave.APIToken.Expose()
	}
	if cfg.Remnawave.WebhookSecret.Expose() != "" {
		p.Config[plugin.RemnawaveConfigKeyWebhookSecret] = cfg.Remnawave.WebhookSecret.Expose()
	}
}

// buildBuiltInManifest constructs a minimal Manifest for a built-in plugin.
// Built-in plugins have no WASM hooks — the manifest is stored purely so the
// config schema (field types, required flags, secret markers) is available
// to the admin UI and the RedactedConfig logic.
func buildBuiltInManifest(def plugin.BuiltInPluginDef) *plugin.Manifest {
	return &plugin.Manifest{
		Plugin: plugin.ManifestPlugin{
			ID:          def.Slug,
			Name:        def.Name,
			Version:     def.Version,
			Description: def.Description,
			Author:      def.Author,
		},
		Config: def.ConfigFields,
	}
}
