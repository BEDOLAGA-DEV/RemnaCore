-- ============================================================================
-- Remnawave Bindings
-- ============================================================================

-- name: CreateBinding :exec
INSERT INTO multisub.remnawave_bindings (
    id, subscription_id, platform_user_id, remnawave_uuid, remnawave_short_uuid,
    remnawave_username, purpose, status, traffic_limit_bytes,
    allowed_nodes, inbound_tags, synced_at, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);

-- name: GetBindingByID :one
SELECT id, subscription_id, platform_user_id, remnawave_uuid, remnawave_short_uuid,
       remnawave_username, purpose, status, traffic_limit_bytes,
       allowed_nodes, inbound_tags, synced_at, created_at, updated_at
FROM multisub.remnawave_bindings WHERE id = $1;

-- name: GetBindingsBySubscriptionID :many
SELECT id, subscription_id, platform_user_id, remnawave_uuid, remnawave_short_uuid,
       remnawave_username, purpose, status, traffic_limit_bytes,
       allowed_nodes, inbound_tags, synced_at, created_at, updated_at
FROM multisub.remnawave_bindings WHERE subscription_id = $1 ORDER BY created_at;

-- name: GetBindingsByPlatformUserID :many
SELECT id, subscription_id, platform_user_id, remnawave_uuid, remnawave_short_uuid,
       remnawave_username, purpose, status, traffic_limit_bytes,
       allowed_nodes, inbound_tags, synced_at, created_at, updated_at
FROM multisub.remnawave_bindings WHERE platform_user_id = $1 ORDER BY created_at;

-- name: GetBindingByRemnawaveUUID :one
SELECT id, subscription_id, platform_user_id, remnawave_uuid, remnawave_short_uuid,
       remnawave_username, purpose, status, traffic_limit_bytes,
       allowed_nodes, inbound_tags, synced_at, created_at, updated_at
FROM multisub.remnawave_bindings WHERE remnawave_uuid = $1;

-- name: GetActiveBindingsBySubscriptionID :many
SELECT id, subscription_id, platform_user_id, remnawave_uuid, remnawave_short_uuid,
       remnawave_username, purpose, status, traffic_limit_bytes,
       allowed_nodes, inbound_tags, synced_at, created_at, updated_at
FROM multisub.remnawave_bindings WHERE subscription_id = $1 AND status = 'active' ORDER BY created_at;

-- name: GetAllActiveBindings :many
SELECT id, subscription_id, platform_user_id, remnawave_uuid, remnawave_short_uuid,
       remnawave_username, purpose, status, traffic_limit_bytes,
       allowed_nodes, inbound_tags, synced_at, created_at, updated_at
FROM multisub.remnawave_bindings WHERE status = 'active' ORDER BY created_at;

-- name: GetFailedBindingsWithRemnawaveUUID :many
SELECT id, subscription_id, platform_user_id, remnawave_uuid, remnawave_short_uuid,
       remnawave_username, purpose, status, traffic_limit_bytes,
       allowed_nodes, inbound_tags, synced_at, created_at, updated_at
FROM multisub.remnawave_bindings
WHERE status = 'failed' AND remnawave_uuid IS NOT NULL
ORDER BY created_at;

-- name: UpdateBinding :exec
UPDATE multisub.remnawave_bindings
SET remnawave_uuid = $2, remnawave_short_uuid = $3, status = $4,
    traffic_limit_bytes = $5, allowed_nodes = $6, inbound_tags = $7, synced_at = $8
WHERE id = $1;

-- name: DeleteBinding :exec
DELETE FROM multisub.remnawave_bindings WHERE id = $1;

-- ============================================================================
-- Idempotency Keys
-- ============================================================================

-- name: TryAcquireIdempotencyKey :execresult
INSERT INTO multisub.idempotency_keys (key, created_at, expires_at)
VALUES ($1, now(), now() + interval '24 hours')
ON CONFLICT (key) DO NOTHING;

-- name: CleanupExpiredIdempotencyKeys :exec
DELETE FROM multisub.idempotency_keys WHERE expires_at < now();
