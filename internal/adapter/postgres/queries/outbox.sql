-- name: InsertOutboxEvent :exec
INSERT INTO public.outbox (event_type, payload, entity_id)
VALUES ($1, $2, $3);

-- name: InsertOutboxEventWithID :exec
-- Inserts an outbox event using the domain event's UUIDv7 as the row ID,
-- ensuring the outbox row ID and domain event ID are identical. This enables
-- end-to-end deduplication: the relay uses this ID as the NATS Msg-Id header,
-- and consumers use it as the idempotency key.
INSERT INTO public.outbox (id, event_type, payload, entity_id)
VALUES ($1, $2, $3, $4);

-- name: GetUnpublishedOutboxEvents :many
-- Fetches unpublished events that have not been marked as permanently failed.
-- The relay_failed filter ensures dead-lettered events are excluded from polling.
SELECT id, event_type, payload, created_at, sequence_number, relay_attempts
FROM public.outbox
WHERE published = false AND relay_failed = false
ORDER BY sequence_number
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: GetUnpublishedForWorker :many
-- Fetches unpublished events assigned to a specific relay worker via
-- entity_id hash affinity. Each worker only processes events where
-- abs(hashtext(entity_id)) % worker_count = worker_id, ensuring events
-- for the same entity always go to the same worker and preserving
-- per-entity sequence ordering. hashtext is a stable PG built-in.
SELECT id, event_type, payload, created_at, sequence_number, relay_attempts
FROM public.outbox
WHERE published = false AND relay_failed = false
  AND abs(hashtext(entity_id)) % sqlc.arg(worker_count)::int = sqlc.arg(worker_id)::int
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
-- Deletes published events that are past retention AND within the current
-- quarter's partition (created_at >= $2). Older partitions are left for
-- PartitionManager.DetachAndDrop, which is O(1) and avoids autovacuum.
-- created_at < $1 enables partition pruning on the range-partitioned outbox.
-- This is safe because created_at <= published_at always holds.
DELETE FROM public.outbox
WHERE published = true AND published_at < $1 AND created_at < $1 AND created_at >= $2;

-- name: GetLastPublishedOutboxSequence :one
-- Returns the highest sequence_number among published outbox events.
-- Used by the reconciliation check to compare outbox state with JetStream
-- stream state. Returns 0 if no published events exist.
SELECT COALESCE(MAX(sequence_number), 0)::bigint AS last_sequence
FROM public.outbox
WHERE published = true;

-- MarkPublishedBatch: implemented as raw pgx MERGE in outbox_repo.go because
-- sqlc does not support PG18 MERGE ... RETURNING syntax.
