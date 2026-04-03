-- name: CreatePlugin :exec
INSERT INTO plugins.plugin_registry (id, slug, name, version, description, author, license, sdk_version, lang, wasm_bytes, manifest, status, config, permissions, installed_at, enabled_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17);

-- name: GetPluginByID :one
SELECT * FROM plugins.plugin_registry WHERE id = $1;

-- name: GetPluginBySlug :one
SELECT * FROM plugins.plugin_registry WHERE slug = $1;

-- name: GetAllPlugins :many
SELECT * FROM plugins.plugin_registry ORDER BY installed_at DESC;

-- name: GetEnabledPlugins :many
SELECT * FROM plugins.plugin_registry WHERE status = 'enabled' ORDER BY installed_at DESC;

-- name: UpdatePluginStatus :exec
UPDATE plugins.plugin_registry SET status = $2, error_log = $3, enabled_at = $4, updated_at = now() WHERE id = $1;

-- name: UpdatePlugin :exec
UPDATE plugins.plugin_registry
SET name = $2, version = $3, description = $4, author = $5, license = $6,
    sdk_version = $7, lang = $8, wasm_bytes = $9, manifest = $10,
    permissions = $11, updated_at = $12
WHERE id = $1;

-- name: UpdatePluginConfig :exec
UPDATE plugins.plugin_registry SET config = $2, updated_at = now() WHERE id = $1;

-- name: DeletePlugin :exec
DELETE FROM plugins.plugin_registry WHERE id = $1;

-- Plugin storage queries

-- name: StorageGet :one
SELECT value, expires_at FROM plugins.plugin_storage WHERE plugin_slug = $1 AND key = $2;

-- name: StorageSet :exec
INSERT INTO plugins.plugin_storage (plugin_slug, key, value, expires_at, updated_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (plugin_slug, key) DO UPDATE SET value = EXCLUDED.value, expires_at = EXCLUDED.expires_at, updated_at = now();

-- name: StorageDelete :exec
DELETE FROM plugins.plugin_storage WHERE plugin_slug = $1 AND key = $2;

-- name: StorageDeleteAll :exec
DELETE FROM plugins.plugin_storage WHERE plugin_slug = $1;

-- name: StorageGetSize :one
SELECT COALESCE(SUM(pg_column_size(value)), 0)::BIGINT AS total_bytes
FROM plugins.plugin_storage WHERE plugin_slug = $1;

-- name: StorageDeleteExpired :exec
DELETE FROM plugins.plugin_storage WHERE expires_at IS NOT NULL AND expires_at < now();
