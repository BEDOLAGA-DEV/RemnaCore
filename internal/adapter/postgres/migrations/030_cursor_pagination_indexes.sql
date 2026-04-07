-- ============================================================================
-- Migration 030: Composite indexes for cursor-based pagination
-- ============================================================================
-- GetAllCursor queries use ORDER BY created_at DESC, id DESC with
-- WHERE (created_at, id) < ($2, $3). Without a matching composite index
-- PostgreSQL falls back to a sequential scan or a sort on every page fetch.
--
-- Using regular CREATE INDEX (not CONCURRENTLY) because Atlas wraps
-- migrations in a transaction block. For production tables with heavy
-- write load, apply these indexes manually with CONCURRENTLY outside
-- of the migration framework.
-- ============================================================================

CREATE INDEX IF NOT EXISTS idx_subs_cursor
    ON billing.subscriptions (created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_invoices_cursor
    ON billing.invoices (created_at DESC, id DESC);
