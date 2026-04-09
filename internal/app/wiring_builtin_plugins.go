package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
)

// seedBuiltInPlugins ensures every built-in plugin definition exists in the
// database. If a built-in plugin is already present (matched by slug), its
// manifest is updated. New entries are seeded with empty config — connection
// settings are managed exclusively through the plugin UI.
func seedBuiltInPlugins(repo plugin.PluginRepository, logger *slog.Logger) error {
	ctx := context.Background()

	allPlugins := append(plugin.BuiltInPlugins(), builtin.AllPlugins()...)
	for _, def := range allPlugins {
		existing, err := repo.GetBySlug(ctx, def.Slug)
		if err == nil && existing != nil {
			// Always update manifest, name, description, version for existing
			// built-in plugins so new fields/pages/routes take effect.
			newManifest := buildBuiltInManifest(def)
			existing.Manifest = newManifest
			existing.Name = def.Name
			existing.Version = def.Version
			existing.Description = def.Description
			if updateErr := repo.UpdatePlugin(ctx, existing); updateErr != nil {
				logger.Warn("failed to update built-in plugin manifest",
					slog.String("slug", def.Slug),
					slog.Any("error", updateErr),
				)
			} else {
				logger.Info("updated built-in plugin manifest", slog.String("slug", def.Slug))
			}
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
			Status:      plugin.StatusEnabled,
			Config:      make(map[string]string),
			Manifest:    buildBuiltInManifest(def),
			InstalledAt: now,
			EnabledAt:   &now,
			UpdatedAt:   now,
		}

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
		Pages:  def.Pages,
		Routes: def.Routes,
	}
}
