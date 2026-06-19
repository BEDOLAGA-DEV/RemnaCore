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

-- ---------------------------------------------------------------------------
-- 2. Backfill tenant_id from the owning platform_users row.
--    Runs under the platform sentinel ('*' = tenantctx.PlatformScopeSentinel)
--    so the non-superuser, FORCE-RLS-subject migration role can read the
--    source tables. Today every row resolves to NULL (no shop users exist),
--    which is the correct, safe no-op; it self-corrects as shop users appear.
--    Order is significant: invoice_line_items depends on invoices.tenant_id,
--    and binding_sync_log depends on remnawave_bindings.tenant_id.
-- ---------------------------------------------------------------------------
BEGIN;
SET LOCAL app.tenant_id = '*';

-- subscriptions.tenant_id <- platform_users.tenant_id via subscriptions.user_id
UPDATE billing.subscriptions s
SET tenant_id = u.tenant_id
FROM identity.platform_users u
WHERE u.id = s.user_id;

-- invoices.tenant_id <- platform_users.tenant_id via invoices.user_id (direct)
UPDATE billing.invoices i
SET tenant_id = u.tenant_id
FROM identity.platform_users u
WHERE u.id = i.user_id;

-- invoice_line_items.tenant_id <- billing.invoices.tenant_id via invoice_id
UPDATE billing.invoice_line_items li
SET tenant_id = i.tenant_id
FROM billing.invoices i
WHERE i.id = li.invoice_id;

-- family_groups.tenant_id <- platform_users.tenant_id via family_groups.owner_id
UPDATE billing.family_groups g
SET tenant_id = u.tenant_id
FROM identity.platform_users u
WHERE u.id = g.owner_id;

-- family_members.tenant_id <- platform_users.tenant_id via family_members.user_id
UPDATE billing.family_members m
SET tenant_id = u.tenant_id
FROM identity.platform_users u
WHERE u.id = m.user_id;

-- payment_records.tenant_id <- billing.invoices.user_id -> platform_users.tenant_id via invoice_id
UPDATE payment.payment_records p
SET tenant_id = u.tenant_id
FROM billing.invoices i
JOIN identity.platform_users u ON u.id = i.user_id
WHERE i.id = p.invoice_id;

-- remnawave_bindings.tenant_id <- platform_users.tenant_id via platform_user_id
UPDATE multisub.remnawave_bindings b
SET tenant_id = u.tenant_id
FROM identity.platform_users u
WHERE u.id = b.platform_user_id;

-- binding_sync_log.tenant_id <- remnawave_bindings.tenant_id via binding_id (after bindings backfilled)
UPDATE multisub.binding_sync_log l
SET tenant_id = b.tenant_id
FROM multisub.remnawave_bindings b
WHERE b.id = l.binding_id;

COMMIT;

-- ---------------------------------------------------------------------------
-- 3. Enable sentinel-aware FORCE RLS on every Tier-2 table. FORCE goes LAST,
--    after the section-2 backfill, so the migration role can still write
--    tenant_id during backfill. Policy shape (canonical, contract §2):
--      USING     : '*' (sentinel) OR tenant_id matches the GUC  (NO IS NULL branch)
--      WITH CHECK : '*' OR a NULL tenant_id OR tenant_id matches the GUC (permits
--                   tenant-less public/system inserts; blocks foreign tenant_id)
--    The literal '*' is the platform sentinel, = tenantctx.PlatformScopeSentinel.
--    payment.webhook_log is deliberately NOT included (see header).
-- ---------------------------------------------------------------------------

-- billing.subscriptions
ALTER TABLE billing.subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE billing.subscriptions FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE tablename = 'subscriptions' AND schemaname = 'billing' AND policyname = 'tenant_isolation_subscriptions'
    ) THEN
        CREATE POLICY tenant_isolation_subscriptions ON billing.subscriptions
            USING (
                current_setting('app.tenant_id', true) = '*'
                OR tenant_id::text = current_setting('app.tenant_id', true)
            )
            WITH CHECK (
                current_setting('app.tenant_id', true) = '*'
                OR tenant_id IS NULL
                OR tenant_id::text = current_setting('app.tenant_id', true)
            );
    END IF;
END $$;

-- billing.invoices
ALTER TABLE billing.invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE billing.invoices FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE tablename = 'invoices' AND schemaname = 'billing' AND policyname = 'tenant_isolation_invoices'
    ) THEN
        CREATE POLICY tenant_isolation_invoices ON billing.invoices
            USING (
                current_setting('app.tenant_id', true) = '*'
                OR tenant_id::text = current_setting('app.tenant_id', true)
            )
            WITH CHECK (
                current_setting('app.tenant_id', true) = '*'
                OR tenant_id IS NULL
                OR tenant_id::text = current_setting('app.tenant_id', true)
            );
    END IF;
END $$;

-- billing.invoice_line_items
ALTER TABLE billing.invoice_line_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE billing.invoice_line_items FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE tablename = 'invoice_line_items' AND schemaname = 'billing' AND policyname = 'tenant_isolation_invoice_line_items'
    ) THEN
        CREATE POLICY tenant_isolation_invoice_line_items ON billing.invoice_line_items
            USING (
                current_setting('app.tenant_id', true) = '*'
                OR tenant_id::text = current_setting('app.tenant_id', true)
            )
            WITH CHECK (
                current_setting('app.tenant_id', true) = '*'
                OR tenant_id IS NULL
                OR tenant_id::text = current_setting('app.tenant_id', true)
            );
    END IF;
END $$;

-- billing.family_groups
ALTER TABLE billing.family_groups ENABLE ROW LEVEL SECURITY;
ALTER TABLE billing.family_groups FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE tablename = 'family_groups' AND schemaname = 'billing' AND policyname = 'tenant_isolation_family_groups'
    ) THEN
        CREATE POLICY tenant_isolation_family_groups ON billing.family_groups
            USING (
                current_setting('app.tenant_id', true) = '*'
                OR tenant_id::text = current_setting('app.tenant_id', true)
            )
            WITH CHECK (
                current_setting('app.tenant_id', true) = '*'
                OR tenant_id IS NULL
                OR tenant_id::text = current_setting('app.tenant_id', true)
            );
    END IF;
END $$;

-- billing.family_members
ALTER TABLE billing.family_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE billing.family_members FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE tablename = 'family_members' AND schemaname = 'billing' AND policyname = 'tenant_isolation_family_members'
    ) THEN
        CREATE POLICY tenant_isolation_family_members ON billing.family_members
            USING (
                current_setting('app.tenant_id', true) = '*'
                OR tenant_id::text = current_setting('app.tenant_id', true)
            )
            WITH CHECK (
                current_setting('app.tenant_id', true) = '*'
                OR tenant_id IS NULL
                OR tenant_id::text = current_setting('app.tenant_id', true)
            );
    END IF;
END $$;

-- payment.payment_records
ALTER TABLE payment.payment_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE payment.payment_records FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE tablename = 'payment_records' AND schemaname = 'payment' AND policyname = 'tenant_isolation_payment_records'
    ) THEN
        CREATE POLICY tenant_isolation_payment_records ON payment.payment_records
            USING (
                current_setting('app.tenant_id', true) = '*'
                OR tenant_id::text = current_setting('app.tenant_id', true)
            )
            WITH CHECK (
                current_setting('app.tenant_id', true) = '*'
                OR tenant_id IS NULL
                OR tenant_id::text = current_setting('app.tenant_id', true)
            );
    END IF;
END $$;

-- multisub.remnawave_bindings
ALTER TABLE multisub.remnawave_bindings ENABLE ROW LEVEL SECURITY;
ALTER TABLE multisub.remnawave_bindings FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE tablename = 'remnawave_bindings' AND schemaname = 'multisub' AND policyname = 'tenant_isolation_remnawave_bindings'
    ) THEN
        CREATE POLICY tenant_isolation_remnawave_bindings ON multisub.remnawave_bindings
            USING (
                current_setting('app.tenant_id', true) = '*'
                OR tenant_id::text = current_setting('app.tenant_id', true)
            )
            WITH CHECK (
                current_setting('app.tenant_id', true) = '*'
                OR tenant_id IS NULL
                OR tenant_id::text = current_setting('app.tenant_id', true)
            );
    END IF;
END $$;

-- multisub.binding_sync_log
ALTER TABLE multisub.binding_sync_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE multisub.binding_sync_log FORCE ROW LEVEL SECURITY;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE tablename = 'binding_sync_log' AND schemaname = 'multisub' AND policyname = 'tenant_isolation_binding_sync_log'
    ) THEN
        CREATE POLICY tenant_isolation_binding_sync_log ON multisub.binding_sync_log
            USING (
                current_setting('app.tenant_id', true) = '*'
                OR tenant_id::text = current_setting('app.tenant_id', true)
            )
            WITH CHECK (
                current_setting('app.tenant_id', true) = '*'
                OR tenant_id IS NULL
                OR tenant_id::text = current_setting('app.tenant_id', true)
            );
    END IF;
END $$;
