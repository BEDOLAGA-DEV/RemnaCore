-- name: InsertOutboxEvent :exec
INSERT INTO public.outbox (event_type, payload)
VALUES ($1, $2);

-- name: InsertOutboxEventWithID :exec
-- Inserts an outbox event using the domain event's UUIDv7 as the row ID,
-- ensuring the outbox row ID and domain event ID are identical. This enables
-- end-to-end deduplication: the relay uses this ID as the NATS Msg-Id header,
-- and consumers use it as the idempotency key.
INSERT INTO public.outbox (id, event_type, payload)
VALUES ($1, $2, $3);

-- name: GetUnpublishedOutboxEvents :many
-- Fetches unpublished events that have not been marked as permanently failed.
-- The relay_failed filter ensures dead-lettered events are excluded from polling.
SELECT id, event_type, payload, created_at, sequence_number, relay_attempts
FROM public.outbox
WHERE published = false AND relay_failed = false
ORDER BY sequence_number
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: IncrementRelayAttempts :exec
-- Atomically increments the relay_attempts counter for a failed event. When
-- the new count reaches $2 (MaxRelayAttempts), relay_failed is set to true,
-- permanently excluding the event from relay polling. The event remains in the
-- table for manual inspection.
-- created_at is required for partition pruning on the range-partitioned outbox.
UPDATE public.outbox
SET relay_attempts = relay_attempts + 1,
    relay_failed = CASE WHEN relay_attempts + 1 >= $2 THEN true ELSE false END
WHERE id = $1 AND created_at = $3;

-- name: MarkOutboxEventPublished :exec
-- Includes created_at for partition pruning on the range-partitioned outbox.
UPDATE public.outbox
SET published = true, published_at = now()
WHERE id = $1 AND created_at = $2;

-- name: DeleteOldPublishedOutboxEvents :exec
-- created_at < $1 enables partition pruning on the range-partitioned outbox.
-- This is safe because created_at <= published_at always holds.
DELETE FROM public.outbox
WHERE published = true AND published_at < $1 AND created_at < $1;

-- MarkPublishedBatch: implemented as raw pgx MERGE in outbox_repo.go because
-- sqlc does not support PG18 MERGE ... RETURNING syntax.
