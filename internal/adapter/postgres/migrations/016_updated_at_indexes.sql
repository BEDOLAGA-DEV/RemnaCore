-- ============================================================================
-- Migration 016: Indexes for updated_at sorting (admin dashboard)
-- ============================================================================
-- The admin dashboard lists subscriptions and invoices ordered by most
-- recently updated. Without a covering index, PG must seq-scan + sort.
-- These DESC indexes turn the ORDER BY updated_at DESC queries into
-- index-only scans.
--
-- CONCURRENTLY avoids blocking DML during index creation.
-- ============================================================================

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subs_updated_at
    ON billing.subscriptions (updated_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_invoices_updated_at
    ON billing.invoices (updated_at DESC);
