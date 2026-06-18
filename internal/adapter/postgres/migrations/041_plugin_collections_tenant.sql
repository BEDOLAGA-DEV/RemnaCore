-- ============================================================================
-- Migration 041: plugins.collections tenant scoping (RBAC Phase C / C2)
-- ============================================================================
-- plugins.collections is the LIVE backing store for balance, checkout, AND
-- tariff plugin documents (confirmed by tracing internal/builtin/tariff/handler.go
-- and internal/adapter/postgres/collections_repo.go read paths). Adding a
-- nullable tenant_id makes shop-scoped money/tariff data isolatable.
--
-- tenant_id is a PLAIN UUID column (NO cross-schema FK), matching the
-- established no-cross-schema-FK convention (cf. balance.wallets). NULL =
-- platform-owned document, visible only under the platform sentinel.
--
-- The repository (CollectionsRepository, refactored in C2) self-stamps tenant_id
-- on insert from the app.tenant_id GUC; reads rely on the GUC + RLS below.
--
-- RLS policy uses the fail-closed sentinel form: an unset/empty GUC sees zero
-- rows; the platform sentinel '*' (Go: tenantctx.PlatformScopeSentinel) sees
-- all rows; a shop UUID sees only its own. WITH CHECK additionally permits
-- NULL-tenant (platform) inserts. FORCE is applied LAST, after the column add.
-- ============================================================================

BEGIN;

-- The migration role is the non-superuser, non-BYPASSRLS `platform` role, which
-- is subject to FORCE RLS. Run the DDL/DML under the sentinel so any touch of
-- an already-FORCE-RLS table in this txn is permitted. '*' == tenantctx.PlatformScopeSentinel.
SET LOCAL app.tenant_id = '*';

ALTER TABLE plugins.collections
    ADD COLUMN IF NOT EXISTS tenant_id uuid NULL;

COMMENT ON COLUMN plugins.collections.tenant_id IS
    'Owning shop tenant (no FK: cross-schema boundary, enforced by application + RLS). NULL = platform-owned document, visible only under the platform sentinel.';

-- This composite index leads with tenant_id to serve future tenant-scoped
-- scans of plugins.collections. It overlaps 034's existing
-- (plugin_slug, collection) index on the trailing columns; that index is
-- retained for the legacy non-tenant lookup path and is NOT dropped here.
CREATE INDEX IF NOT EXISTS idx_collections_tenant_plugin_collection
    ON plugins.collections (tenant_id, plugin_slug, collection);

COMMIT;

-- ---------------------------------------------------------------------------
-- Row-level security (sentinel + WITH CHECK form). ENABLE/FORCE after the
-- column exists; FORCE LAST so the column-add above is never blocked by it.
-- ---------------------------------------------------------------------------
ALTER TABLE plugins.collections ENABLE ROW LEVEL SECURITY;
ALTER TABLE plugins.collections FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation_collections ON plugins.collections;
CREATE POLICY tenant_isolation_collections ON plugins.collections
    USING (
        current_setting('app.tenant_id', true) = '*'
        OR tenant_id::text = current_setting('app.tenant_id', true)
    )
    WITH CHECK (
        current_setting('app.tenant_id', true) = '*'
        OR tenant_id IS NULL
        OR tenant_id::text = current_setting('app.tenant_id', true)
    );
