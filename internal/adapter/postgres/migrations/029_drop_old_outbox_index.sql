-- ============================================================================
-- Migration 029: Drop superseded outbox index
-- ============================================================================
-- Drop idx_outbox_unpublished: superseded by idx_outbox_unpublished_active
-- (added in migration 024) which includes the relay_failed filter.
-- The relay query always filters on relay_failed=false, so the old partial
-- index (published=false only) is never chosen by the planner and wastes
-- disk space and write amplification on every INSERT.
-- ============================================================================

DROP INDEX IF EXISTS public.idx_outbox_unpublished;
