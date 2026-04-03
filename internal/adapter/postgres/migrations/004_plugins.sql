CREATE SCHEMA IF NOT EXISTS plugins;

CREATE TABLE plugins.plugin_registry (
    id UUID PRIMARY KEY,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    description TEXT,
    author TEXT,
    license TEXT,
    sdk_version TEXT,
    lang TEXT,
    wasm_bytes BYTEA,
    manifest JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'installed' CHECK (status IN ('installed', 'enabled', 'disabled', 'error')),
    config JSONB NOT NULL DEFAULT '{}',
    permissions TEXT[] NOT NULL DEFAULT '{}',
    error_log TEXT,
    installed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    enabled_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_plugin_slug ON plugins.plugin_registry (slug);
CREATE INDEX idx_plugin_status ON plugins.plugin_registry (status);

-- Plugin isolated key-value storage (shared table, namespaced by plugin_slug)
CREATE TABLE plugins.plugin_storage (
    plugin_slug TEXT NOT NULL,
    key TEXT NOT NULL,
    value JSONB NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (plugin_slug, key)
);

CREATE INDEX idx_storage_expires ON plugins.plugin_storage (expires_at) WHERE expires_at IS NOT NULL;

-- Triggers for updated_at
CREATE TRIGGER trigger_plugin_registry_updated
    BEFORE UPDATE ON plugins.plugin_registry
    FOR EACH ROW EXECUTE FUNCTION identity.set_updated_at();

CREATE TRIGGER trigger_plugin_storage_updated
    BEFORE UPDATE ON plugins.plugin_storage
    FOR EACH ROW EXECUTE FUNCTION identity.set_updated_at();
