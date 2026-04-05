-- Content-addressable WASM binary storage.
-- Plugins reference WASM by hash instead of storing inline.
CREATE TABLE IF NOT EXISTS plugins.wasm_store (
    hash       TEXT PRIMARY KEY,
    data       BYTEA NOT NULL,
    size_bytes BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE plugins.plugin_registry ADD COLUMN IF NOT EXISTS wasm_hash TEXT;
