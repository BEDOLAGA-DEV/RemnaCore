-- Plugin collections: structured JSON document storage for plugins.
-- Each plugin can create multiple named collections with JSON documents.
CREATE TABLE IF NOT EXISTS plugins.collections (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    plugin_slug TEXT       NOT NULL,
    collection  TEXT       NOT NULL,
    document    JSONB      NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_collections_plugin_collection
    ON plugins.collections(plugin_slug, collection);

DROP TRIGGER IF EXISTS trigger_plugin_collections_updated ON plugins.collections;
CREATE TRIGGER trigger_plugin_collections_updated
    BEFORE UPDATE ON plugins.collections
    FOR EACH ROW EXECUTE FUNCTION identity.set_updated_at();
