-- Add a BIGSERIAL sequence number to the outbox table so that events written
-- in the same transaction (with identical created_at timestamps) are relayed
-- in deterministic insertion order.

ALTER TABLE public.outbox ADD COLUMN IF NOT EXISTS sequence_number BIGSERIAL;

-- Replace the created_at-based partial index with a sequence_number-based one.
DROP INDEX IF EXISTS idx_outbox_unpublished;
CREATE INDEX idx_outbox_unpublished ON public.outbox (sequence_number) WHERE published = false;
