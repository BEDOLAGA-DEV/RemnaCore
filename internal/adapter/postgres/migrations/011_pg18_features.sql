-- ============================================================================
-- Migration 011: PostgreSQL 18 Feature Adoption
-- ============================================================================
-- Leverages PG 18 capabilities: UUIDv7, temporal constraints (WITHOUT OVERLAPS),
-- skip scan composite indexes, expression indexes, and async I/O tuning.
-- ============================================================================

-- ============================================================================
-- 1. UUIDv7 defaults — replace gen_random_uuid() with uuidv7()
-- ============================================================================
-- UUIDv7 produces timestamp-ordered UUIDs, reducing B-tree page splits and
-- improving insert locality across all tables that use DB-side UUID generation.

ALTER TABLE billing.invoice_line_items ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE billing.family_members     ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE multisub.binding_sync_log  ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE payment.webhook_log        ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE public.outbox              ALTER COLUMN id SET DEFAULT uuidv7();

-- ============================================================================
-- 2. Temporal constraints — WITHOUT OVERLAPS for subscriptions
-- ============================================================================
-- Guarantees at the database level that a single user cannot have two
-- subscriptions to the same plan with overlapping billing periods.

-- NOTE: btree_gist requires the CREATE privilege on the database. In hosted
-- environments (RDS, Cloud SQL) this may need to be run by a privileged user
-- or enabled via the provider's extension management (e.g., RDS parameter group).
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- Add a computed tstzrange column derived from period_start/period_end.
-- period_end is NOT NULL (see 002_billing.sql), so the range is always bounded.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'billing' AND table_name = 'subscriptions' AND column_name = 'billing_period'
    ) THEN
        ALTER TABLE billing.subscriptions
            ADD COLUMN billing_period tstzrange
            GENERATED ALWAYS AS (tstzrange(period_start, period_end, '[)')) STORED;
    END IF;
END $$;

-- Temporal exclusion constraint: no overlapping periods per user+plan among
-- non-terminal subscriptions. Uses EXCLUDE USING gist (not UNIQUE WITHOUT
-- OVERLAPS) because PG18's WITHOUT OVERLAPS syntax does not support WHERE
-- clauses. The WHERE filter excludes cancelled/expired subscriptions so that
-- a user can resubscribe to the same plan after cancellation.
-- (The PG18 native UNIQUE WITHOUT OVERLAPS is used in migration 012 for the
-- temporal FK target constraint where no WHERE filter is needed.)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uq_subs_user_plan_no_overlap'
          AND conrelid = 'billing.subscriptions'::regclass
    ) THEN
        ALTER TABLE billing.subscriptions
            ADD CONSTRAINT uq_subs_user_plan_no_overlap
            EXCLUDE USING gist (
                user_id  WITH =,
                plan_id  WITH =,
                billing_period WITH &&
            ) WHERE (status IN ('trial', 'active', 'past_due', 'paused'));
    END IF;
END $$;

-- ============================================================================
-- 3. Skip scan composite indexes — merge single-column pairs
-- ============================================================================
-- PG 18 skip scan allows a composite (a, b) index to serve queries on just b
-- when a has low cardinality. This lets us drop redundant single-column indexes.

-- subscriptions: merge (user_id) + (status) → (user_id, status)
DROP INDEX IF EXISTS billing.idx_subs_user;
DROP INDEX IF EXISTS billing.idx_subs_status;
CREATE INDEX IF NOT EXISTS idx_subs_user_status ON billing.subscriptions (user_id, status);

-- invoices: merge (user_id) + (status) → (user_id, status)
-- Keep idx_invoices_sub as it serves a different access pattern.
DROP INDEX IF EXISTS billing.idx_invoices_user;
DROP INDEX IF EXISTS billing.idx_invoices_status;
CREATE INDEX IF NOT EXISTS idx_invoices_user_status ON billing.invoices (user_id, status);

-- bindings: merge (platform_user_id) + (status) → (platform_user_id, status)
-- Keep idx_bindings_sub as it serves a different access pattern.
DROP INDEX IF EXISTS multisub.idx_bindings_user;
DROP INDEX IF EXISTS multisub.idx_bindings_status;
CREATE INDEX IF NOT EXISTS idx_bindings_user_status ON multisub.remnawave_bindings (platform_user_id, status);

-- commissions: merge (reseller_id) + (status) → (reseller_id, status)
DROP INDEX IF EXISTS reseller.idx_commissions_reseller;
DROP INDEX IF EXISTS reseller.idx_commissions_status;
CREATE INDEX IF NOT EXISTS idx_commissions_reseller_status ON reseller.commissions (reseller_id, status);

-- ============================================================================
-- 4. Expression index — replace stored generated column for email
-- ============================================================================
-- The stored generated column email_lower duplicates data on disk. An
-- expression index on lower(email) achieves the same uniqueness guarantee
-- without the extra column. Queries must use lower(email) instead of
-- referencing email_lower directly.

DROP INDEX IF EXISTS identity.idx_users_email_lower;
ALTER TABLE identity.platform_users DROP COLUMN IF EXISTS email_lower;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower ON identity.platform_users (lower(email));

-- ============================================================================
-- 5. Outbox event version column
-- ============================================================================
-- Supports future event schema evolution by tagging each outbox row with a
-- version number.

ALTER TABLE public.outbox ADD COLUMN IF NOT EXISTS version INT NOT NULL DEFAULT 1;
