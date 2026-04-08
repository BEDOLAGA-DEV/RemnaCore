-- ============================================================================
-- Migration 031: System settings runtime override table
-- ============================================================================
-- Stores runtime configuration overrides as a single JSONB blob. On startup
-- and after each PUT /api/admin/settings, env-var defaults from config.Load
-- are merged with these overrides so that administrators can tune editable
-- settings without restarting the service.
-- ============================================================================

CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE IF NOT EXISTS platform.system_settings (
    id         TEXT        PRIMARY KEY DEFAULT 'runtime',
    overrides  JSONB       NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed the single row that all reads/writes target.
INSERT INTO platform.system_settings (id, overrides)
VALUES ('runtime', '{}')
ON CONFLICT (id) DO NOTHING;
