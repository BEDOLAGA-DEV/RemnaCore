-- ============================================================================
-- Migration 042: Tier-2 tenant_id + row-level security (RBAC Phase C / C3)
-- ============================================================================
-- Adds a nullable tenant_id (NULL = platform-owned row) to the Tier-2 tables
-- that previously relied solely on an application-side user_id FK chain, then
-- backfills tenant_id from the owning platform_users row and enables the
-- sentinel-aware FORCE RLS policy (defense-in-depth behind the C1 app gate).
--
-- payment.webhook_log is INTENTIONALLY EXCLUDED: it has no user_id, no FK, and
-- is written by public unauthenticated webhook ingestion, so there is no
-- resolvable tenant key. It stays platform-only + application-filtered (same
-- isolation model as metrics.samples, cf. migration 037). Do NOT add tenant_id
-- or RLS to it.
--
-- Cross-schema references (tenant_id, user_id) are PLAIN UUID columns enforced
-- by the application, matching the no-cross-schema-FK convention (cf.
-- balance.wallets / migration 035). Pre-037 tables keep their existing
-- gen_random_uuid() PK default; this migration adds no PK default.
--
-- Backfill + the FORCE-RLS column writes run under SET LOCAL app.tenant_id = '*'
-- (the platform sentinel, = tenantctx.PlatformScopeSentinel) because the
-- runtime/migration role is non-superuser, non-BYPASSRLS and is itself subject
-- to FORCE RLS. Order per migration: ADD COLUMN -> backfill -> ENABLE+FORCE+policy.
-- ============================================================================

-- ---------------------------------------------------------------------------
-- 1. Add tenant_id (nullable) + partial index to each Tier-2 table.
-- ---------------------------------------------------------------------------

ALTER TABLE billing.subscriptions       ADD COLUMN IF NOT EXISTS tenant_id uuid NULL;
ALTER TABLE billing.invoices            ADD COLUMN IF NOT EXISTS tenant_id uuid NULL;
ALTER TABLE billing.invoice_line_items  ADD COLUMN IF NOT EXISTS tenant_id uuid NULL;
ALTER TABLE billing.family_groups       ADD COLUMN IF NOT EXISTS tenant_id uuid NULL;
ALTER TABLE billing.family_members      ADD COLUMN IF NOT EXISTS tenant_id uuid NULL;
ALTER TABLE payment.payment_records     ADD COLUMN IF NOT EXISTS tenant_id uuid NULL;
ALTER TABLE multisub.remnawave_bindings ADD COLUMN IF NOT EXISTS tenant_id uuid NULL;
ALTER TABLE multisub.binding_sync_log   ADD COLUMN IF NOT EXISTS tenant_id uuid NULL;

CREATE INDEX IF NOT EXISTS idx_subscriptions_tenant ON billing.subscriptions (tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_invoices_tenant ON billing.invoices (tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_line_items_tenant ON billing.invoice_line_items (tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_family_groups_tenant ON billing.family_groups (tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_family_members_tenant ON billing.family_members (tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_payments_tenant ON payment.payment_records (tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_bindings_tenant ON multisub.remnawave_bindings (tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_sync_log_tenant ON multisub.binding_sync_log (tenant_id) WHERE tenant_id IS NOT NULL;

COMMENT ON COLUMN billing.subscriptions.tenant_id       IS 'Owning shop (no FK: cross-schema boundary, enforced by application). NULL = platform-owned.';
COMMENT ON COLUMN billing.invoices.tenant_id            IS 'Owning shop (no FK: cross-schema boundary, enforced by application). NULL = platform-owned.';
COMMENT ON COLUMN billing.invoice_line_items.tenant_id  IS 'Owning shop (no FK: cross-schema boundary, enforced by application). NULL = platform-owned.';
COMMENT ON COLUMN billing.family_groups.tenant_id       IS 'Owning shop (no FK: cross-schema boundary, enforced by application). NULL = platform-owned.';
COMMENT ON COLUMN billing.family_members.tenant_id      IS 'Owning shop (no FK: cross-schema boundary, enforced by application). NULL = platform-owned.';
COMMENT ON COLUMN payment.payment_records.tenant_id     IS 'Owning shop (no FK: cross-schema boundary, enforced by application). NULL = platform-owned.';
COMMENT ON COLUMN multisub.remnawave_bindings.tenant_id IS 'Owning shop (no FK: cross-schema boundary, enforced by application). NULL = platform-owned.';
COMMENT ON COLUMN multisub.binding_sync_log.tenant_id   IS 'Owning shop (no FK: cross-schema boundary, enforced by application). NULL = platform-owned.';
