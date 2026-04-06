-- ============================================================================
-- Migration 023: Add single-column status index for status-only queries
-- ============================================================================
-- Migration 011 dropped idx_bindings_status in favor of the composite
-- idx_bindings_user_status (platform_user_id, status), relying on PG18 skip
-- scan for status-only queries. While skip scan CAN serve these queries, it
-- is not guaranteed at all table sizes and NDV distributions.
--
-- Three production queries filter on status without platform_user_id:
--   1. GetAllActiveBindings:              WHERE status = 'active'
--   2. GetFailedBindingsWithRemnawaveUUID: WHERE status = 'failed' AND remnawave_uuid IS NOT NULL
--   3. resetAbandonedReconcilingQuery:    WHERE status = 'reconciling' AND updated_at < ...
--
-- Adding a dedicated single-column index on status guarantees the planner
-- always has an efficient access path for these queries, regardless of
-- skip scan heuristics. The storage overhead is minimal (single text column).
-- ============================================================================

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_bindings_status
    ON multisub.remnawave_bindings (status);
