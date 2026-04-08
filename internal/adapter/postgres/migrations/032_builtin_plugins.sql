-- Add is_builtin flag to distinguish platform-provided plugins from user-installed ones.
-- Built-in plugins cannot be uninstalled and are auto-seeded on startup.
ALTER TABLE plugins.plugin_registry ADD COLUMN IF NOT EXISTS is_builtin BOOLEAN NOT NULL DEFAULT false;
