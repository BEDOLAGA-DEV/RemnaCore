-- ============================================================================
-- Migration 040: RLS policy rewrite — fail-closed sentinel + WITH CHECK
-- ============================================================================
-- Rewrites the existing tenant-isolation policies (018 platform_users, 028
-- reseller_accounts, 035 balance.*, 036 checkout.*) from fail-OPEN to
-- fail-CLOSED, and adds a WITH CHECK clause that closes the cross-tenant /
-- NULL-tenant write hole the prior USING-only policies left open.
--
-- New USING semantics: unset/empty app.tenant_id -> ZERO rows; the platform
-- sentinel '*' -> all rows; a shop UUID -> only that shop's rows. A NULL-tenant
-- row is PLATFORM-PRIVATE: visible ONLY under the sentinel, never to a shop GUC
-- and never when the GUC is unset/empty. USING deliberately has NO `IS NULL`
-- branch (that was the fail-open leak).
--
-- New WITH CHECK semantics: permits a write when GUC = sentinel, OR the row's
-- tenant_id IS NULL (tenant-less public paths: register / accept-invitation /
-- payment-webhook completion), OR the row's tenant_id matches the shop GUC — so
-- a shop GUC can never insert/update a foreign tenant_id.
--
-- The SQL literal '*' is the platform sentinel; in Go it is the named const
-- tenantctx.PlatformScopeSentinel (pkg/tenantctx) — no magic string.
--
-- Policy-only migration (no DML / no backfill), so no `SET LOCAL app.tenant_id`
-- body is needed. FORCE ROW LEVEL SECURITY is re-affirmed for each table (it is
-- idempotent and already set by 018/035/036).
-- ============================================================================

-- ---------------------------------------------------------------------------
-- identity.platform_users  (was 018: USING tenant_id IS NULL OR ...)
-- ---------------------------------------------------------------------------
ALTER TABLE identity.platform_users ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.platform_users FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_platform_users ON identity.platform_users;
CREATE POLICY tenant_isolation_platform_users ON identity.platform_users
    USING (
        current_setting('app.tenant_id', true) = '*'
        OR tenant_id::text = current_setting('app.tenant_id', true)
    )
    WITH CHECK (
        current_setting('app.tenant_id', true) = '*'
        OR tenant_id IS NULL
        OR tenant_id::text = current_setting('app.tenant_id', true)
    );

-- ---------------------------------------------------------------------------
-- reseller.reseller_accounts  (drops the 028 3-branch empty-GUC fail-open form)
-- ---------------------------------------------------------------------------
ALTER TABLE reseller.reseller_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE reseller.reseller_accounts FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_reseller_accounts ON reseller.reseller_accounts;
CREATE POLICY tenant_isolation_reseller_accounts ON reseller.reseller_accounts
    USING (
        current_setting('app.tenant_id', true) = '*'
        OR tenant_id::text = current_setting('app.tenant_id', true)
    )
    WITH CHECK (
        current_setting('app.tenant_id', true) = '*'
        OR tenant_id IS NULL
        OR tenant_id::text = current_setting('app.tenant_id', true)
    );

-- ---------------------------------------------------------------------------
-- balance.wallets  (was 035: USING tenant_id IS NULL OR ...)
-- ---------------------------------------------------------------------------
ALTER TABLE balance.wallets ENABLE ROW LEVEL SECURITY;
ALTER TABLE balance.wallets FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_wallets ON balance.wallets;
CREATE POLICY tenant_isolation_wallets ON balance.wallets
    USING (
        current_setting('app.tenant_id', true) = '*'
        OR tenant_id::text = current_setting('app.tenant_id', true)
    )
    WITH CHECK (
        current_setting('app.tenant_id', true) = '*'
        OR tenant_id IS NULL
        OR tenant_id::text = current_setting('app.tenant_id', true)
    );

-- ---------------------------------------------------------------------------
-- balance.ledger_entries  (was 035: policy tenant_isolation_ledger)
-- ---------------------------------------------------------------------------
ALTER TABLE balance.ledger_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE balance.ledger_entries FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_ledger ON balance.ledger_entries;
CREATE POLICY tenant_isolation_ledger ON balance.ledger_entries
    USING (
        current_setting('app.tenant_id', true) = '*'
        OR tenant_id::text = current_setting('app.tenant_id', true)
    )
    WITH CHECK (
        current_setting('app.tenant_id', true) = '*'
        OR tenant_id IS NULL
        OR tenant_id::text = current_setting('app.tenant_id', true)
    );

-- ---------------------------------------------------------------------------
-- balance.topups  (was 035: policy tenant_isolation_topups)
-- ---------------------------------------------------------------------------
ALTER TABLE balance.topups ENABLE ROW LEVEL SECURITY;
ALTER TABLE balance.topups FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_topups ON balance.topups;
CREATE POLICY tenant_isolation_topups ON balance.topups
    USING (
        current_setting('app.tenant_id', true) = '*'
        OR tenant_id::text = current_setting('app.tenant_id', true)
    )
    WITH CHECK (
        current_setting('app.tenant_id', true) = '*'
        OR tenant_id IS NULL
        OR tenant_id::text = current_setting('app.tenant_id', true)
    );

-- ---------------------------------------------------------------------------
-- checkout.sessions  (was 036: policy tenant_isolation_checkout)
-- ---------------------------------------------------------------------------
ALTER TABLE checkout.sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE checkout.sessions FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_checkout ON checkout.sessions;
CREATE POLICY tenant_isolation_checkout ON checkout.sessions
    USING (
        current_setting('app.tenant_id', true) = '*'
        OR tenant_id::text = current_setting('app.tenant_id', true)
    )
    WITH CHECK (
        current_setting('app.tenant_id', true) = '*'
        OR tenant_id IS NULL
        OR tenant_id::text = current_setting('app.tenant_id', true)
    );

-- ---------------------------------------------------------------------------
-- checkout.saved_methods  (was 036: policy tenant_isolation_methods)
-- ---------------------------------------------------------------------------
ALTER TABLE checkout.saved_methods ENABLE ROW LEVEL SECURITY;
ALTER TABLE checkout.saved_methods FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_methods ON checkout.saved_methods;
CREATE POLICY tenant_isolation_methods ON checkout.saved_methods
    USING (
        current_setting('app.tenant_id', true) = '*'
        OR tenant_id::text = current_setting('app.tenant_id', true)
    )
    WITH CHECK (
        current_setting('app.tenant_id', true) = '*'
        OR tenant_id IS NULL
        OR tenant_id::text = current_setting('app.tenant_id', true)
    );
