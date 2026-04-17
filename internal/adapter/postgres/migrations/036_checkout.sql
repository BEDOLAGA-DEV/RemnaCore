-- Checkout Plugin: unified checkout sessions with split-payment support.

CREATE SCHEMA IF NOT EXISTS checkout;

CREATE TABLE checkout.sessions (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL,
    tenant_id         UUID,
    tariff_id         TEXT NOT NULL,
    quantity          INT NOT NULL DEFAULT 1,
    currency          TEXT NOT NULL,
    final_price_cents BIGINT NOT NULL,
    promo_code        TEXT,
    promo_discount    BIGINT NOT NULL DEFAULT 0,
    payment_mode      TEXT NOT NULL DEFAULT 'single',
    status            TEXT NOT NULL DEFAULT 'pending',
    price_breakdown   JSONB NOT NULL DEFAULT '{}',
    sources           JSONB NOT NULL DEFAULT '[]',
    sub_invoices      JSONB NOT NULL DEFAULT '[]',
    metadata          JSONB NOT NULL DEFAULT '{}',
    user_agent        TEXT,
    ip_address        INET,
    referrer_url      TEXT,
    expires_at        TIMESTAMPTZ NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at      TIMESTAMPTZ,
    abandoned_at      TIMESTAMPTZ,
    reminder_sent_at  TIMESTAMPTZ,
    CONSTRAINT session_status CHECK (status IN
        ('pending','processing','completed','failed','expired','abandoned','cancelled'))
);

CREATE INDEX idx_sessions_user ON checkout.sessions (user_id, created_at DESC);
CREATE INDEX idx_sessions_status ON checkout.sessions (status, expires_at)
    WHERE status IN ('pending','processing');
CREATE INDEX idx_sessions_abandoned ON checkout.sessions (updated_at)
    WHERE status = 'pending' AND reminder_sent_at IS NULL;

ALTER TABLE checkout.sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE checkout.sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_checkout ON checkout.sessions
    USING (tenant_id IS NULL OR tenant_id::text = current_setting('app.tenant_id', true));

-- Saved payment methods (tokens from providers)
CREATE TABLE checkout.saved_methods (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    tenant_id       UUID,
    provider        TEXT NOT NULL,
    token_id        TEXT NOT NULL,
    last4           TEXT,
    brand           TEXT,
    expires_month   INT,
    expires_year    INT,
    is_default      BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_saved_methods_user ON checkout.saved_methods (user_id);

ALTER TABLE checkout.saved_methods ENABLE ROW LEVEL SECURITY;
ALTER TABLE checkout.saved_methods FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_methods ON checkout.saved_methods
    USING (tenant_id IS NULL OR tenant_id::text = current_setting('app.tenant_id', true));
