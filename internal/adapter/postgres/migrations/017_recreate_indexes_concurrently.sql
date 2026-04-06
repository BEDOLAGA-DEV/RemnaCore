-- ============================================================================
-- Migration 017: Recreate indexes with CONCURRENTLY for zero-downtime
-- ============================================================================
-- Migration 011 created four composite indexes without CONCURRENTLY, which
-- holds an ACCESS EXCLUSIVE lock for the full build duration — blocking all
-- reads and writes on the table.
--
-- This migration drops and recreates them with CONCURRENTLY so the build
-- only holds a weaker SHARE UPDATE EXCLUSIVE lock that does not block DML.
--
-- NOTE: CONCURRENTLY cannot run inside a transaction block.
-- Atlas runs each top-level statement as a separate implicit transaction
-- when not wrapped in BEGIN/COMMIT, which is required here.
--
-- ============================================================================
-- Covering indexes decision (audit point 7)
-- ============================================================================
-- Covering indexes (INCLUDE clause) are intentionally not used at this stage.
-- Current table sizes (< 100K rows) do not benefit measurably from
-- index-only scans — heap fetches are cheap with small tables and warm
-- buffer cache.
--
-- When to add covering indexes:
--   - subscriptions table exceeds 500K rows
--   - pg_stat_user_indexes shows high idx_scan but low idx_tup_fetch
--     (indicating frequent heap fetches after index scan)
--   - GetActiveSubscriptionsByUserID becomes a latency bottleneck
--
-- Candidate covering index:
--   CREATE INDEX CONCURRENTLY idx_subs_user_status_covering
--       ON billing.subscriptions (user_id, status)
--       INCLUDE (plan_id, period_start, period_end, addon_ids, created_at);
-- ============================================================================

-- billing.subscriptions (user_id, status)
DROP INDEX IF EXISTS billing.idx_subs_user_status;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subs_user_status
    ON billing.subscriptions (user_id, status);

-- billing.invoices (user_id, status)
DROP INDEX IF EXISTS billing.idx_invoices_user_status;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_invoices_user_status
    ON billing.invoices (user_id, status);

-- multisub.remnawave_bindings (platform_user_id, status)
DROP INDEX IF EXISTS multisub.idx_bindings_user_status;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_bindings_user_status
    ON multisub.remnawave_bindings (platform_user_id, status);

-- reseller.commissions (reseller_id, status)
DROP INDEX IF EXISTS reseller.idx_commissions_reseller_status;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_commissions_reseller_status
    ON reseller.commissions (reseller_id, status);
