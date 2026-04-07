-- ============================================================================
-- Migration 027: Schema corrections for domain model alignment
-- ============================================================================
-- Fixes 6 critical schema/domain mismatches:
--   #1  binding status CHECK missing 'limited'
--   #2  plans missing max_addons column
--   #3  subscriptions missing pending_original_plan_id / pending_requested_at
--   #4  invoices discounts JSONB missing array CHECK
--   #5  payment_records missing status CHECK
--   #6  subscriptions missing dedicated GiST index for temporal queries
-- ============================================================================

-- ---------------------------------------------------------------------------
-- #1: Add 'limited' to binding status CHECK constraint.
-- Domain defines BindingLimited with full state machine (active→limited→active).
-- Without this, UPDATE SET status='limited' causes CHECK violation.
-- ---------------------------------------------------------------------------
ALTER TABLE multisub.remnawave_bindings DROP CONSTRAINT IF EXISTS remnawave_bindings_status_check;
ALTER TABLE multisub.remnawave_bindings ADD CONSTRAINT remnawave_bindings_status_check
    CHECK (status IN ('pending', 'active', 'failed', 'disabled', 'deprovisioned', 'reconciling', 'limited'));

-- ---------------------------------------------------------------------------
-- #2: Add max_addons column to plans.
-- AddonSpec.CanAddAddon checks this value; without the column it is always 0
-- (unlimited) regardless of what the domain model intends.
-- ---------------------------------------------------------------------------
ALTER TABLE billing.plans ADD COLUMN IF NOT EXISTS max_addons INT NOT NULL DEFAULT 0;

-- ---------------------------------------------------------------------------
-- #3: Add PendingPlanChange audit columns to subscriptions.
-- Domain PendingPlanChange VO stores OriginalPlanID and RequestedAt alongside
-- PlanID. Without these columns the audit trail is lost on each restart.
-- ---------------------------------------------------------------------------
ALTER TABLE billing.subscriptions ADD COLUMN IF NOT EXISTS pending_original_plan_id UUID;
ALTER TABLE billing.subscriptions ADD COLUMN IF NOT EXISTS pending_requested_at TIMESTAMPTZ;

-- ---------------------------------------------------------------------------
-- #4: Add CHECK on invoice discounts JSONB structure.
-- Ensures only JSON arrays are stored, preventing deserialization errors in Go.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    ALTER TABLE billing.invoices ADD CONSTRAINT invoices_discounts_is_array
        CHECK (jsonb_typeof(discounts) = 'array');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- ---------------------------------------------------------------------------
-- #5: Add CHECK on payment record status.
-- All other status columns have CHECK constraints; payment was missing.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    ALTER TABLE payment.payment_records ADD CONSTRAINT payment_records_status_check
        CHECK (status IN ('pending', 'completed', 'failed', 'refunded'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- ---------------------------------------------------------------------------
-- #6: Dedicated GiST index for temporal subscription queries.
-- The exclusion constraint index has a WHERE clause that makes it suboptimal
-- for general @> containment and && overlap queries.
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_subs_user_billing_period
    ON billing.subscriptions USING gist (user_id, billing_period);
