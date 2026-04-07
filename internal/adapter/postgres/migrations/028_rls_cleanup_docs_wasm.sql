-- ============================================================================
-- Migration 028: RLS fixes, cross-schema FK docs, cleanup indexes, WASM CAS
-- ============================================================================
-- Addresses issues #5, #7, #9, #11, #12 from the database audit.
-- ============================================================================


-- ============================================================================
-- Issue #9: Fix reseller RLS to allow admin/system access without tenant context
-- ============================================================================
-- The platform_users RLS policy allows tenant_id IS NULL for system-level rows.
-- Admin/system operations run without app.tenant_id set, which means
-- current_setting('app.tenant_id', true) returns NULL or ''. The reseller
-- policy was missing this fallback, so admin queries to reseller_accounts
-- silently returned zero rows.
--
-- Fix: Allow access when app.tenant_id is NULL (not set) or '' (empty string),
-- matching the platform_users policy semantics.
-- ============================================================================

DROP POLICY IF EXISTS tenant_isolation_reseller_accounts ON reseller.reseller_accounts;
CREATE POLICY tenant_isolation_reseller_accounts ON reseller.reseller_accounts
    USING (
        tenant_id::text = current_setting('app.tenant_id', true)
        OR current_setting('app.tenant_id', true) IS NULL
        OR current_setting('app.tenant_id', true) = ''
    );


-- ============================================================================
-- Issue #5: Cross-schema foreign key documentation
-- ============================================================================
-- Several tables reference entities in other schemas (e.g., billing.subscriptions
-- references identity.platform_users) but deliberately omit FK constraints.
-- This is intentional for bounded context isolation:
--
--   1. Bounded context isolation: billing and identity are separate DDD contexts
--   2. Application-level enforcement: the service layer validates entity existence
--      before creating dependent records
--   3. Microservice extraction readiness: cross-schema FKs would prevent future
--      schema/service separation without data migration
--
-- The COMMENT ON COLUMN statements below document these intentional decisions
-- so that future developers do not add FKs without understanding the rationale.
-- ============================================================================

COMMENT ON COLUMN billing.subscriptions.user_id IS
    'References identity.platform_users.id (no FK: cross-schema boundary, enforced by application)';

COMMENT ON COLUMN billing.invoices.user_id IS
    'References identity.platform_users.id (no FK: cross-schema boundary, enforced by application)';

COMMENT ON COLUMN multisub.remnawave_bindings.platform_user_id IS
    'References identity.platform_users.id (no FK: cross-schema boundary, enforced by application)';

COMMENT ON COLUMN multisub.remnawave_bindings.subscription_id IS
    'References billing.subscriptions.id (no FK: cross-schema boundary, enforced by application)';

COMMENT ON COLUMN reseller.reseller_accounts.user_id IS
    'References identity.platform_users.id (no FK: cross-schema boundary, enforced by application)';

COMMENT ON COLUMN payment.payment_records.invoice_id IS
    'References billing.invoices.id (no FK: cross-schema boundary, enforced by application)';


-- ============================================================================
-- Issue #7: Cleanup support — add missing expired password resets query
-- ============================================================================
-- Sessions and email verifications already have DeleteExpired queries, but
-- password_resets did not. This index accelerates the cleanup DELETE.
-- ============================================================================

CREATE INDEX IF NOT EXISTS idx_password_reset_expires
    ON identity.password_resets (expires_at);


-- ============================================================================
-- Issue #12: Cleanup indexes for idempotency keys and binding sync log
-- ============================================================================
-- The idempotency_keys table already has idx_idempotency_expires.
-- binding_sync_log needs a created_at index for the retention-based cleanup.
-- ============================================================================

CREATE INDEX IF NOT EXISTS idx_sync_log_created
    ON multisub.binding_sync_log (created_at);


-- ============================================================================
-- Issue #11: Migrate inline WASM bytes to content-addressable store
-- ============================================================================
-- Plugins that still have wasm_bytes stored inline (wasm_hash IS NULL) are
-- migrated to the CAS table (plugins.wasm_store). After this migration,
-- all plugins reference WASM by hash and the inline bytes are cleared.
--
-- ON CONFLICT DO NOTHING handles the case where the same binary was already
-- stored in the CAS (e.g., two plugins sharing identical WASM).
-- ============================================================================

INSERT INTO plugins.wasm_store (hash, data, size_bytes)
SELECT encode(sha256(wasm_bytes), 'hex'), wasm_bytes, octet_length(wasm_bytes)
FROM plugins.plugin_registry
WHERE wasm_bytes IS NOT NULL AND wasm_hash IS NULL
ON CONFLICT DO NOTHING;

UPDATE plugins.plugin_registry
SET wasm_hash = encode(sha256(wasm_bytes), 'hex'), wasm_bytes = NULL
WHERE wasm_bytes IS NOT NULL AND wasm_hash IS NULL;
