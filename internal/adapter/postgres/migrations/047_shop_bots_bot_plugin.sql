-- ============================================================================
-- Migration 047: per-shop Telegram bot PLUGIN selection (BP1).
-- bot_plugin_slug chooses which bot plugin runs a shop's Telegram bot.
-- NULL means the default built-in behavior ("cabinet-bot"). shop_bots is
-- RLS-FORCE'd, so run under the sentinel.
-- ============================================================================
BEGIN;
SET LOCAL app.tenant_id = '*';
ALTER TABLE reseller.shop_bots ADD COLUMN IF NOT EXISTS bot_plugin_slug text;
COMMIT;
