-- name: InsertOutboxEvent :exec
INSERT INTO public.outbox (event_type, payload)
VALUES ($1, $2);

-- name: GetUnpublishedOutboxEvents :many
SELECT id, event_type, payload, created_at
FROM public.outbox
WHERE published = false
ORDER BY created_at
LIMIT $1;

-- name: MarkOutboxEventPublished :exec
UPDATE public.outbox
SET published = true, published_at = now()
WHERE id = $1;

-- name: DeleteOldPublishedOutboxEvents :exec
DELETE FROM public.outbox
WHERE published = true AND published_at < $1;
