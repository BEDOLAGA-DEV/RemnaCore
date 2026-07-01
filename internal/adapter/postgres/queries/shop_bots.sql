-- name: UpsertShopBot :exec
INSERT INTO reseller.shop_bots (tenant_id, bot_token_enc, webhook_secret, cabinet_url, bot_username, enabled, bot_plugin_slug)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (tenant_id) DO UPDATE SET
    bot_token_enc = EXCLUDED.bot_token_enc,
    webhook_secret = EXCLUDED.webhook_secret,
    cabinet_url = EXCLUDED.cabinet_url,
    bot_username = EXCLUDED.bot_username,
    enabled = EXCLUDED.enabled,
    bot_plugin_slug = EXCLUDED.bot_plugin_slug;

-- name: GetShopBotByTenant :one
SELECT tenant_id, bot_token_enc, webhook_secret, cabinet_url, bot_username, enabled, bot_plugin_slug, created_at, updated_at
FROM reseller.shop_bots WHERE tenant_id = $1;

-- name: ListEnabledShopBots :many
SELECT tenant_id, bot_token_enc, webhook_secret, cabinet_url, bot_username, enabled, bot_plugin_slug, created_at, updated_at
FROM reseller.shop_bots WHERE enabled = true;
