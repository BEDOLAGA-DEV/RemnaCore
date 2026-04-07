-- ============================================================================
-- Migration 024: Add relay dead-letter columns for permanently failed events
-- ============================================================================
-- Problem: If an outbox event has a malformed payload that cannot be published
-- to NATS, the relay retries it forever. The row stays published=false
-- permanently, wasting relay cycles and DB locks on every poll.
--
-- Solution: Track per-event relay attempts. After MaxRelayAttempts (10)
-- consecutive failures, the event is marked relay_failed=true and excluded
-- from relay polling. Failed events remain queryable for manual inspection
-- and operational remediation.
--
-- The existing idx_outbox_unpublished partial index (sequence_number WHERE
-- published=false) still works but now also matches relay_failed events.
-- A new partial index idx_outbox_unpublished_active narrows the scan to
-- only active (non-failed) unpublished events, which is what the relay
-- actually queries.
-- ============================================================================

-- Add relay_attempts counter for dead-letter detection.
-- After max attempts, events are marked as relay_failed and excluded from polling.
ALTER TABLE public.outbox ADD COLUMN IF NOT EXISTS relay_attempts int NOT NULL DEFAULT 0;
ALTER TABLE public.outbox ADD COLUMN IF NOT EXISTS relay_failed boolean NOT NULL DEFAULT false;

-- Partial index for unpublished, non-failed events (used by relay polling).
-- This replaces idx_outbox_unpublished as the primary relay scan path.
CREATE INDEX IF NOT EXISTS idx_outbox_unpublished_active
    ON public.outbox (created_at)
    WHERE published = false AND relay_failed = false;
