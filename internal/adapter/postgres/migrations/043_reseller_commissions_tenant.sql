-- ============================================================================
-- Migration 043: reseller.commissions tenant scoping (RBAC Phase C / C4)
-- ============================================================================
-- Adds a denormalized tenant_id to reseller.commissions so the row can be
-- isolated by RLS without a cross-schema join. tenant_id is sourced from the
-- owning reseller account: commissions.reseller_id -> reseller_accounts.id ->
-- reseller_accounts.tenant_id. No cross-schema FK is added (tenant_id is a plain
-- UUID column, consistent with balance.wallets and the §3 convention).
--
-- Today every reseller account is platform-owned, so the backfill is a safe
-- no-op that self-corrects as shop reseller accounts are created.
--
-- This migration mutates a table that becomes FORCE-RLS at the end of the run,
-- and the runtime/migration `platform` role is non-superuser + non-BYPASSRLS,
-- so the body runs under the platform sentinel (= tenantctx.PlatformScopeSentinel,
-- the Go-side '*' constant). Column-add -> backfill -> ENABLE/FORCE/policy, with
-- FORCE applied LAST (after the backfill).
-- ============================================================================

BEGIN;

SET LOCAL app.tenant_id = '*';   -- '*' == tenantctx.PlatformScopeSentinel: satisfy sentinel RLS during migration

-- Step 1: add the nullable tenant_id column (NULL = platform-owned commission).
ALTER TABLE reseller.commissions
    ADD COLUMN IF NOT EXISTS tenant_id uuid;

COMMENT ON COLUMN reseller.commissions.tenant_id IS
    'Denormalized from reseller.reseller_accounts.tenant_id via reseller_id (no FK: same-schema denormalization for RLS). NULL = platform-owned.';

-- Step 2: backfill from the owning reseller account.
UPDATE reseller.commissions c
SET tenant_id = ra.tenant_id
FROM reseller.reseller_accounts ra
WHERE c.reseller_id = ra.id
  AND c.tenant_id IS DISTINCT FROM ra.tenant_id;

-- Step 3 (REWRITTEN above for honesty): supporting index for tenant-scoped commission listing.
CREATE INDEX IF NOT EXISTS idx_commissions_tenant_status
    ON reseller.commissions (tenant_id, status);

COMMIT;

-- ============================================================================
-- Step 4: enable RLS with the fail-closed sentinel policy (contract §2 / spec §3.1).
-- FORCE is applied LAST, after the backfill above is committed.
--   USING: unset/'' -> zero rows; '*' (= tenantctx.PlatformScopeSentinel) -> all;
--          a shop UUID -> only that shop's commissions. NO `IS NULL` branch in
--          USING (NULL-tenant commissions are platform-private, visible only
--          under the sentinel).
--   WITH CHECK: adds `tenant_id IS NULL` so platform-owned commission inserts
--          (created by the system RecordCommission path with no active shop)
--          succeed; a shop GUC can never insert a foreign tenant_id.
-- ============================================================================

ALTER TABLE reseller.commissions ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_commissions ON reseller.commissions;
CREATE POLICY tenant_isolation_commissions ON reseller.commissions
    USING (
        current_setting('app.tenant_id', true) = '*'
        OR tenant_id::text = current_setting('app.tenant_id', true)
    )
    WITH CHECK (
        current_setting('app.tenant_id', true) = '*'
        OR tenant_id IS NULL
        OR tenant_id::text = current_setting('app.tenant_id', true)
    );
ALTER TABLE reseller.commissions FORCE ROW LEVEL SECURITY;
