-- ============================================================================
-- Migration 018: Row-level security for multi-tenant isolation
-- ============================================================================
-- Defense-in-depth: RLS ensures tenant isolation at the database level,
-- independent of application-layer WHERE clauses. The application sets
-- app.tenant_id via SET LOCAL at the start of each transaction (see
-- TxManager.RunInTx).
--
-- Policy semantics:
--   * Rows with tenant_id IS NULL are platform-level and visible to all.
--   * Rows whose tenant_id matches current_setting('app.tenant_id') are
--     visible to that tenant.
--   * When app.tenant_id is not set (empty/null), only platform-level rows
--     are visible. Admin/system operations that need cross-tenant access
--     must run without RLS (e.g. via a superuser role or by resetting the
--     session variable).
--
-- The second boolean parameter of current_setting (true) makes it return
-- NULL instead of raising an error when the variable is not set.
-- ============================================================================

-- ---------------------------------------------------------------------------
-- identity.platform_users
-- ---------------------------------------------------------------------------

ALTER TABLE identity.platform_users ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.platform_users FORCE ROW LEVEL SECURITY;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE tablename = 'platform_users' AND schemaname = 'identity' AND policyname = 'tenant_isolation_platform_users'
    ) THEN
        CREATE POLICY tenant_isolation_platform_users ON identity.platform_users
            USING (
                tenant_id IS NULL
                OR tenant_id::text = current_setting('app.tenant_id', true)
            );
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- reseller.reseller_accounts
-- ---------------------------------------------------------------------------

ALTER TABLE reseller.reseller_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE reseller.reseller_accounts FORCE ROW LEVEL SECURITY;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE tablename = 'reseller_accounts' AND schemaname = 'reseller' AND policyname = 'tenant_isolation_reseller_accounts'
    ) THEN
        CREATE POLICY tenant_isolation_reseller_accounts ON reseller.reseller_accounts
            USING (
                tenant_id::text = current_setting('app.tenant_id', true)
            );
    END IF;
END $$;
