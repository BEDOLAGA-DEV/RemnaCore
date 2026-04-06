-- ============================================================================
-- Migration 021: Schema sync — add missing columns for domain model fields
-- ============================================================================
-- Fixes data-loss bug where domain struct fields (PendingPlanID, FailReason,
-- PricingReason, Discounts) had no corresponding DB column and the
-- 'reconciling' binding status was rejected by the CHECK constraint.
-- ============================================================================

-- ---------------------------------------------------------------------------
-- 1a. billing.subscriptions: add pending_plan_id for deferred downgrades
-- ---------------------------------------------------------------------------
ALTER TABLE billing.subscriptions
    ADD COLUMN IF NOT EXISTS pending_plan_id UUID;

-- ---------------------------------------------------------------------------
-- 1b. multisub.remnawave_bindings: add fail_reason for failure diagnostics
-- ---------------------------------------------------------------------------
ALTER TABLE multisub.remnawave_bindings
    ADD COLUMN IF NOT EXISTS fail_reason TEXT NOT NULL DEFAULT '';

-- ---------------------------------------------------------------------------
-- 1c. multisub.remnawave_bindings: add 'reconciling' to status CHECK
-- ---------------------------------------------------------------------------
ALTER TABLE multisub.remnawave_bindings
    DROP CONSTRAINT IF EXISTS remnawave_bindings_status_check;

ALTER TABLE multisub.remnawave_bindings
    ADD CONSTRAINT remnawave_bindings_status_check
    CHECK (status IN ('pending', 'active', 'failed', 'disabled', 'deprovisioned', 'reconciling'));

-- ---------------------------------------------------------------------------
-- 1d. billing.invoices: add pricing_reason for plugin pricing audit trail
-- ---------------------------------------------------------------------------
ALTER TABLE billing.invoices
    ADD COLUMN IF NOT EXISTS pricing_reason TEXT NOT NULL DEFAULT '';

-- ---------------------------------------------------------------------------
-- 1e. billing.invoices: add discounts JSONB for structured discount storage
-- ---------------------------------------------------------------------------
-- JSONB is preferred over a separate table because:
--   * Discounts are part of the invoice aggregate (not independently queried)
--   * Invoice is immutable after creation (no need for relational updates)
--   * Keeps invoice reads as single-row fetches
ALTER TABLE billing.invoices
    ADD COLUMN IF NOT EXISTS discounts JSONB NOT NULL DEFAULT '[]';
