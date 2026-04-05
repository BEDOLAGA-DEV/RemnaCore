-- name: InsertOutboxEvent :exec
INSERT INTO public.outbox (event_type, payload)
VALUES ($1, $2);

-- name: GetUnpublishedOutboxEvents :many
SELECT id, event_type, payload, created_at, sequence_number
FROM public.outbox
WHERE published = false
ORDER BY sequence_number
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxEventPublished :exec
-- Includes created_at for partition pruning on the range-partitioned outbox.
UPDATE public.outbox
SET published = true, published_at = now()
WHERE id = $1 AND created_at = $2;

-- name: DeleteOldPublishedOutboxEvents :exec
DELETE FROM public.outbox
WHERE published = true AND published_at < $1;

-- MarkPublishedBatch: implemented as raw pgx MERGE in outbox_repo.go because
-- sqlc does not support PG18 MERGE ... RETURNING syntax.
