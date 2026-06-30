-- ============================================================================
-- Migration 044: gate the NULL-tenant WITH CHECK branch on the platform
-- sentinel (RBAC Phase C hardening / audit finding L4-07)
-- ============================================================================
-- The Phase-C policies (041 collections, 042 tier-2 tables, 043 commissions)
-- each carry an UNCONDITIONAL `OR tenant_id IS NULL` WITH CHECK branch, so the
-- policy alone would let ANY session insert a platform-owned (NULL) row;
-- isolation rested solely on each repository's self-stamp SQL. This migration
-- removes that standalone branch: a NULL-tenant insert is now permitted ONLY
-- under the platform sentinel '*' (== tenantctx.PlatformScopeSentinel), where
-- the first branch already allows it. Under a shop UUID GUC a NULL row is
-- rejected. USING (read) clauses are left untouched.
--
-- ALTER POLICY ... WITH CHECK changes only the check expression, so this never
-- has to replicate each table's USING clause. A missing policy errors loudly
-- (catches drift); the schema_migrations ledger prevents re-application.
-- ============================================================================

BEGIN;

-- Touch FORCE-RLS tables under the sentinel so the ALTERs are never blocked.
SET LOCAL app.tenant_id = '*';

DO $$
DECLARE
    target RECORD;
BEGIN
    FOR target IN
        SELECT * FROM (VALUES
            ('plugins',  'collections',         'tenant_isolation_collections'),
            ('billing',  'subscriptions',       'tenant_isolation_subscriptions'),
            ('billing',  'invoices',            'tenant_isolation_invoices'),
            ('billing',  'invoice_line_items',  'tenant_isolation_invoice_line_items'),
            ('billing',  'family_groups',       'tenant_isolation_family_groups'),
            ('billing',  'family_members',      'tenant_isolation_family_members'),
            ('payment',  'payment_records',     'tenant_isolation_payment_records'),
            ('multisub', 'remnawave_bindings',  'tenant_isolation_remnawave_bindings'),
            ('multisub', 'binding_sync_log',    'tenant_isolation_binding_sync_log'),
            ('reseller', 'commissions',         'tenant_isolation_commissions')
        ) AS t(schema_name, table_name, policy_name)
    LOOP
        EXECUTE format(
            'ALTER POLICY %I ON %I.%I WITH CHECK (' ||
            'current_setting(''app.tenant_id'', true) = ''*'' ' ||
            'OR tenant_id::text = current_setting(''app.tenant_id'', true))',
            target.policy_name, target.schema_name, target.table_name
        );
    END LOOP;
END $$;

COMMIT;
