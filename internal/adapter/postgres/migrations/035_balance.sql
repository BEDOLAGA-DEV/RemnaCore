-- Balance Plugin: dual-wallet ledger with triggers, partitioning, and RLS.

CREATE SCHEMA IF NOT EXISTS balance;

-- Wallet — denormalised snapshot for fast reads.
-- Updated by trigger from ledger_entries.
CREATE TABLE balance.wallets (
    user_id         UUID NOT NULL,
    tenant_id       UUID,
    wallet_kind     TEXT NOT NULL CHECK (wallet_kind IN ('money', 'bonus')),
    currency        TEXT NOT NULL,
    balance_cents   BIGINT NOT NULL DEFAULT 0 CHECK (balance_cents >= 0),
    hold_cents      BIGINT NOT NULL DEFAULT 0 CHECK (hold_cents >= 0 AND hold_cents <= balance_cents),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, wallet_kind)
);

CREATE INDEX idx_wallets_tenant ON balance.wallets (tenant_id) WHERE tenant_id IS NOT NULL;

ALTER TABLE balance.wallets ENABLE ROW LEVEL SECURITY;
ALTER TABLE balance.wallets FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_wallets ON balance.wallets
    USING (tenant_id IS NULL OR tenant_id::text = current_setting('app.tenant_id', true));

-- LedgerEntry — immutable journal of all movements.
-- Partitioned by created_at quarterly.
CREATE TABLE balance.ledger_entries (
    id              UUID NOT NULL DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    tenant_id       UUID,
    wallet_kind     TEXT NOT NULL CHECK (wallet_kind IN ('money', 'bonus')),
    amount_cents    BIGINT NOT NULL CHECK (amount_cents <> 0),
    currency        TEXT NOT NULL,
    kind            TEXT NOT NULL,
    source_type     TEXT NOT NULL,
    source_id       TEXT,
    reference_id    TEXT NOT NULL,
    description     TEXT,
    balance_after   BIGINT NOT NULL DEFAULT 0,
    expires_at      TIMESTAMPTZ,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by      TEXT NOT NULL DEFAULT 'system',
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Idempotency: unique per (user, wallet, source_type, reference_id)
CREATE UNIQUE INDEX idx_ledger_idempotency
    ON balance.ledger_entries (user_id, wallet_kind, source_type, reference_id);

CREATE INDEX idx_ledger_user_created
    ON balance.ledger_entries (user_id, created_at DESC);

CREATE INDEX idx_ledger_expires
    ON balance.ledger_entries (expires_at)
    WHERE expires_at IS NOT NULL AND wallet_kind = 'bonus';

ALTER TABLE balance.ledger_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE balance.ledger_entries FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_ledger ON balance.ledger_entries
    USING (tenant_id IS NULL OR tenant_id::text = current_setting('app.tenant_id', true));

-- Quarterly partitions
CREATE TABLE balance.ledger_entries_2026_q2 PARTITION OF balance.ledger_entries
    FOR VALUES FROM ('2026-04-01') TO ('2026-07-01');
CREATE TABLE balance.ledger_entries_2026_q3 PARTITION OF balance.ledger_entries
    FOR VALUES FROM ('2026-07-01') TO ('2026-10-01');
CREATE TABLE balance.ledger_entries_2026_q4 PARTITION OF balance.ledger_entries
    FOR VALUES FROM ('2026-10-01') TO ('2027-01-01');
CREATE TABLE balance.ledger_entries_default PARTITION OF balance.ledger_entries DEFAULT;

-- Trigger: after INSERT into ledger → atomically UPDATE wallet
CREATE OR REPLACE FUNCTION balance.apply_ledger_entry() RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO balance.wallets (user_id, tenant_id, wallet_kind, currency, balance_cents, updated_at)
    VALUES (NEW.user_id, NEW.tenant_id, NEW.wallet_kind, NEW.currency, NEW.amount_cents, NEW.created_at)
    ON CONFLICT (user_id, wallet_kind) DO UPDATE
    SET balance_cents = balance.wallets.balance_cents + NEW.amount_cents,
        updated_at    = NEW.created_at;

    -- Snapshot balance_after in the ledger entry for audit
    UPDATE balance.ledger_entries
    SET balance_after = (SELECT balance_cents FROM balance.wallets
                          WHERE user_id = NEW.user_id AND wallet_kind = NEW.wallet_kind)
    WHERE id = NEW.id AND created_at = NEW.created_at;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_ledger_apply
    AFTER INSERT ON balance.ledger_entries
    FOR EACH ROW EXECUTE FUNCTION balance.apply_ledger_entry();

-- TopUp table (pending → completed/failed)
CREATE TABLE balance.topups (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    tenant_id       UUID,
    amount_cents    BIGINT NOT NULL CHECK (amount_cents > 0),
    currency        TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    payment_method  TEXT NOT NULL,
    payment_id      TEXT,
    ledger_entry_id UUID,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    CONSTRAINT topup_status CHECK (status IN ('pending','processing','completed','failed','cancelled'))
);

CREATE INDEX idx_topups_user ON balance.topups (user_id, created_at DESC);
CREATE INDEX idx_topups_pending ON balance.topups (created_at) WHERE status IN ('pending','processing');
CREATE INDEX idx_topups_payment ON balance.topups (payment_id) WHERE payment_id IS NOT NULL;

ALTER TABLE balance.topups ENABLE ROW LEVEL SECURITY;
ALTER TABLE balance.topups FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_topups ON balance.topups
    USING (tenant_id IS NULL OR tenant_id::text = current_setting('app.tenant_id', true));
