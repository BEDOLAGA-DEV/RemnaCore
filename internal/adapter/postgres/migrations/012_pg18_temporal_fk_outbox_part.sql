-- ============================================================================
-- Migration 012: Temporal FK for invoices + Outbox partitioning
-- ============================================================================
-- Completes PG 18 adoption:
--   9. Temporal FOREIGN KEY ensures invoices fall within subscription periods
--  10. Range-partitioned outbox for efficient cleanup (DROP vs DELETE)
-- ============================================================================

-- ============================================================================
-- 9. Temporal FK: invoices → subscriptions billing period
-- ============================================================================

-- Point-in-time range from invoice created_at. Temporal FK checks containment:
-- the invoice's instant must fall within the subscription's billing period.
-- Uses '[)' (half-open) to match subscription billing_period semantics:
-- billing_period is [period_start, period_end), so an invoice at exactly
-- period_end is considered outside the period.
ALTER TABLE billing.invoices
    ADD COLUMN invoice_period tstzrange
    GENERATED ALWAYS AS (tstzrange(created_at, created_at, '[]')) STORED;

-- Pre-flight check: verify no existing invoices fall outside their
-- subscription's billing period. If this SELECT returns rows, fix the data
-- before running this migration:
--
--   SELECT i.id, i.created_at, s.period_start, s.period_end
--   FROM billing.invoices i
--   JOIN billing.subscriptions s ON s.id = i.subscription_id
--   WHERE i.created_at < s.period_start OR i.created_at >= s.period_end;
--
-- PG18 temporal constraints use WITHOUT OVERLAPS and PERIOD keywords.
-- Wrapped in DO $$ because sqlc's parser does not support this syntax.
-- The UNIQUE on (id, billing_period WITHOUT OVERLAPS) is the FK target.
-- The temporal FK ensures each invoice is created within its subscription period.
--
-- The FK is added as NOT VALID first (instant, no table scan) to avoid blocking
-- writes during deployment. A separate VALIDATE runs afterward; it checks
-- existing rows but does NOT block concurrent DML (PG18 zero-downtime pattern).
DO $$
BEGIN
    EXECUTE 'ALTER TABLE billing.subscriptions ADD CONSTRAINT uq_subs_id_period UNIQUE (id, billing_period WITHOUT OVERLAPS)';
    EXECUTE 'ALTER TABLE billing.invoices ADD CONSTRAINT fk_invoice_sub_period FOREIGN KEY (subscription_id, PERIOD invoice_period) REFERENCES billing.subscriptions (id, PERIOD billing_period) NOT VALID';
END $$;

-- Validate the temporal FK in a separate statement. This scans existing rows
-- to verify they satisfy the constraint, but does NOT hold an exclusive lock
-- on the table — concurrent inserts and updates proceed normally.
-- If any existing invoice falls outside its subscription's billing period,
-- this will fail and must be fixed with a data migration first.
DO $$
BEGIN
    EXECUTE 'ALTER TABLE billing.invoices VALIDATE CONSTRAINT fk_invoice_sub_period';
END $$;

-- ============================================================================
-- 10. Outbox partitioning by created_at
-- ============================================================================
-- Range-partitioning the outbox enables:
--   - Future cleanup path: DETACH + DROP old partitions (no vacuum bloat)
--     Currently DeleteOld still uses DELETE; partition-based cleanup should
--     be added as a scheduled job when the table grows large enough.
--   - Better vacuum: each partition is vacuumed independently
--   - Efficient relay scan: only recent partitions contain unpublished rows
--
-- PK changes from (id) to (id, created_at) because PG requires the partition
-- key in unique/PK constraints. MarkOutboxEventPublished now includes
-- created_at for partition pruning.

-- Step 1: Rename current table and its index.
ALTER TABLE public.outbox RENAME TO outbox_legacy;
ALTER INDEX idx_outbox_unpublished RENAME TO idx_outbox_unpublished_legacy;

-- Step 2: Detach sequence ownership from the legacy table so it survives the
-- DROP TABLE. PG does NOT rename sequences when tables are renamed — the
-- sequence is still named outbox_sequence_number_seq (from the original
-- BIGSERIAL in migration 009).
ALTER SEQUENCE outbox_sequence_number_seq OWNED BY NONE;

-- Step 3: Create partitioned table with same columns.
-- NOTE: The outbox relay must be stopped before running this migration to
-- prevent events being inserted into outbox_legacy after the data migration.
CREATE TABLE public.outbox (
    id              UUID            NOT NULL DEFAULT uuidv7(),
    event_type      TEXT            NOT NULL,
    payload         JSONB           NOT NULL,
    published       BOOLEAN         NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    published_at    TIMESTAMPTZ,
    sequence_number BIGINT          NOT NULL DEFAULT nextval('outbox_sequence_number_seq'),
    version         INT             NOT NULL DEFAULT 1,
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Step 4: Create partitions — quarterly for 2026-2027 + default for overflow.
--
-- Partition management strategy:
--   CREATE: Add new yearly partitions via migration before each year starts.
--   CLEANUP: Old partitions can be detached and dropped instead of DELETE:
--     ALTER TABLE public.outbox DETACH PARTITION outbox_2026_q1 CONCURRENTLY;
--     DROP TABLE outbox_2026_q1;
--   The existing DeleteOldPublishedOutboxEvents (DELETE-based) remains as
--   a fallback for intra-partition cleanup of recent published events.
CREATE TABLE outbox_default PARTITION OF public.outbox DEFAULT;
CREATE TABLE outbox_2026_q1 PARTITION OF public.outbox
    FOR VALUES FROM ('2026-01-01') TO ('2026-04-01');
CREATE TABLE outbox_2026_q2 PARTITION OF public.outbox
    FOR VALUES FROM ('2026-04-01') TO ('2026-07-01');
CREATE TABLE outbox_2026_q3 PARTITION OF public.outbox
    FOR VALUES FROM ('2026-07-01') TO ('2026-10-01');
CREATE TABLE outbox_2026_q4 PARTITION OF public.outbox
    FOR VALUES FROM ('2026-10-01') TO ('2027-01-01');
CREATE TABLE outbox_2027_q1 PARTITION OF public.outbox
    FOR VALUES FROM ('2027-01-01') TO ('2027-04-01');
CREATE TABLE outbox_2027_q2 PARTITION OF public.outbox
    FOR VALUES FROM ('2027-04-01') TO ('2027-07-01');
CREATE TABLE outbox_2027_q3 PARTITION OF public.outbox
    FOR VALUES FROM ('2027-07-01') TO ('2027-10-01');
CREATE TABLE outbox_2027_q4 PARTITION OF public.outbox
    FOR VALUES FROM ('2027-10-01') TO ('2028-01-01');

-- Step 5: Migrate existing data.
INSERT INTO public.outbox (id, event_type, payload, published, created_at, published_at, sequence_number, version)
SELECT id, event_type, payload, published, created_at, published_at, sequence_number, version
FROM outbox_legacy;

-- Step 6: Drop legacy table (sequence survives since ownership was detached).
DROP TABLE outbox_legacy;

-- Step 7: Reassign sequence ownership to the new partitioned table.
ALTER SEQUENCE outbox_sequence_number_seq OWNED BY public.outbox.sequence_number;

-- Step 6: Recreate partial index for relay query on the partitioned table.
CREATE INDEX idx_outbox_unpublished ON public.outbox (sequence_number)
    WHERE published = false;
