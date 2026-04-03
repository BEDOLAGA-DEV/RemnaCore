-- Transactional outbox table for reliable domain event publishing.
-- Events are written here within the same database transaction as business
-- logic, then asynchronously relayed to NATS by the OutboxRelay. This
-- guarantees at-least-once delivery even when the message broker is unavailable.

CREATE TABLE IF NOT EXISTS public.outbox (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type   TEXT        NOT NULL,
    payload      JSONB       NOT NULL,
    published    BOOLEAN     NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);

-- Partial index for the relay query — only scans unpublished rows.
CREATE INDEX idx_outbox_unpublished ON public.outbox (created_at) WHERE published = false;
