-- ============================================================================
-- Migration 025: Add entity_id column for worker-affined relay ordering
-- ============================================================================
-- Problem: Multiple outbox relay workers can deliver events for the same entity
-- out of order because any worker can grab any row. The FOR UPDATE SKIP LOCKED
-- pattern prevents double-processing but does not guarantee per-entity ordering
-- when two workers each pick up different events for the same entity.
--
-- Solution: Materialise entity_id as a top-level column so the relay query can
-- partition work by hash(entity_id) % worker_count = worker_id. Events for the
-- same entity always go to the same worker, preserving sequence ordering.
--
-- The column is extracted from the JSONB payload's entity_id field, which has
-- been populated by all domain event constructors since the EntityID migration.
-- Events without entity_id (backward compat) get an empty string default,
-- meaning all such events hash to the same worker slot — safe and ordered.
-- ============================================================================

-- Step 1: Add entity_id column with empty string default for existing rows.
ALTER TABLE public.outbox ADD COLUMN IF NOT EXISTS entity_id TEXT NOT NULL DEFAULT '';

-- Step 2: Backfill entity_id from JSONB payload for unpublished events.
-- Only unpublished events matter for relay ordering; published events are
-- historical and will be cleaned up by retention.
UPDATE public.outbox
SET entity_id = COALESCE(payload ->> 'entity_id', '')
WHERE published = false AND entity_id = '';

-- Step 3: Index for worker-affined relay queries.
-- The hash-based filter (abs(hashtext(entity_id)) % N = M) benefits from an
-- index on (entity_id) with the same partial filter as the relay query.
-- However, since the relay already uses idx_outbox_unpublished_active
-- (created_at WHERE published=false AND relay_failed=false), and the hash
-- filter is applied post-scan, a dedicated index would only help if the
-- table has many unpublished events. We add a partial index on entity_id
-- for the common relay filter to support future optimizations.
CREATE INDEX IF NOT EXISTS idx_outbox_entity_worker
    ON public.outbox (entity_id, sequence_number)
    WHERE published = false AND relay_failed = false;
