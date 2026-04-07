-- ============================================================================
-- Migration 030: Composite indexes for cursor-based pagination
-- ============================================================================
-- GetAllCursor queries use ORDER BY created_at DESC, id DESC with
-- WHERE (created_at, id) < ($2, $3). Without a matching composite index
-- PostgreSQL falls back to a sequential scan or a sort on every page fetch.
--
-- These indexes cover both the first-page query (no WHERE, just ORDER BY)
-- and subsequent pages (WHERE + ORDER BY) because the sort order matches
-- the index definition.
--
-- NOTE: CONCURRENTLY cannot run inside a transaction block.
-- Atlas runs each top-level statement as a separate implicit transaction
-- when not wrapped in BEGIN/COMMIT, which is required here.
-- ============================================================================

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subs_cursor
    ON billing.subscriptions (created_at DESC, id DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_invoices_cursor
    ON billing.invoices (created_at DESC, id DESC);
