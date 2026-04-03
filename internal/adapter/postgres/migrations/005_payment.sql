CREATE SCHEMA IF NOT EXISTS payment;

CREATE TABLE payment.payment_records (
    id UUID PRIMARY KEY,
    invoice_id UUID NOT NULL,
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,
    amount BIGINT NOT NULL,
    currency TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payments_invoice ON payment.payment_records (invoice_id);
CREATE INDEX idx_payments_external ON payment.payment_records (provider, external_id);

CREATE TABLE payment.webhook_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,
    raw_body BYTEA,
    status TEXT NOT NULL DEFAULT 'pending',
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_webhook_idempotency ON payment.webhook_log (provider, external_id);

CREATE TRIGGER trigger_payment_updated
    BEFORE UPDATE ON payment.payment_records
    FOR EACH ROW EXECUTE FUNCTION identity.set_updated_at();
