-- ============================================================================
-- Migration 046: one Telegram account per (tenant_id, telegram_id) (SP2).
-- A Telegram user maps to a SEPARATE customer per shop; this composite unique
-- index makes find-or-create race-safe. Replaces the non-unique
-- idx_users_telegram_id (lookups are served by the composite index).
-- platform_users is RLS-FORCE'd, so run under the sentinel.
-- ============================================================================
BEGIN;
SET LOCAL app.tenant_id = '*';
DROP INDEX IF EXISTS identity.idx_users_telegram_id;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_tenant_telegram
    ON identity.platform_users (tenant_id, telegram_id)
    WHERE telegram_id IS NOT NULL;
COMMIT;
