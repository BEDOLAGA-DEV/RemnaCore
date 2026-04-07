-- ============================================================================
-- Migration 026: Add trace_parent column for explicit trace propagation
-- ============================================================================
-- Problem: The outbox relay extracts trace_parent from the JSON payload via
-- byte scanning (bytes.Index), which is fragile — it can break on escaped
-- quotes, reordered fields, or payload format changes.
--
-- Solution: Materialise trace_parent as a top-level TEXT column so the relay
-- can read it directly without payload scanning. The byte-scan fallback is
-- retained for backward compatibility with events written before this
-- migration.
--
-- Old events get an empty string default. The relay checks the column first
-- and falls back to byte-scanning the payload when the column is empty.
-- ============================================================================

-- Step 1: Add trace_parent column with empty string default for existing rows.
ALTER TABLE public.outbox ADD COLUMN IF NOT EXISTS trace_parent TEXT NOT NULL DEFAULT '';
