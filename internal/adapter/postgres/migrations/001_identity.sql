CREATE SCHEMA IF NOT EXISTS identity;

CREATE TABLE identity.platform_users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL,
    email_lower TEXT NOT NULL GENERATED ALWAYS AS (lower(email)) STORED,
    password_hash TEXT NOT NULL,
    display_name TEXT,
    email_verified BOOLEAN NOT NULL DEFAULT false,
    telegram_id BIGINT,
    role TEXT NOT NULL DEFAULT 'customer' CHECK (role IN ('customer', 'reseller', 'admin')),
    tenant_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_users_email_lower ON identity.platform_users (email_lower);
CREATE INDEX idx_users_telegram_id ON identity.platform_users (telegram_id) WHERE telegram_id IS NOT NULL;
CREATE INDEX idx_users_role ON identity.platform_users (role);
CREATE INDEX idx_users_tenant ON identity.platform_users (tenant_id) WHERE tenant_id IS NOT NULL;

CREATE TABLE identity.sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES identity.platform_users(id) ON DELETE CASCADE,
    refresh_token TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_sessions_refresh_token ON identity.sessions (refresh_token);
CREATE INDEX idx_sessions_user_id ON identity.sessions (user_id);
CREATE INDEX idx_sessions_expires ON identity.sessions (expires_at);

CREATE TABLE identity.email_verifications (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES identity.platform_users(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    token TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_verifications_token ON identity.email_verifications (token);
CREATE INDEX idx_verifications_user ON identity.email_verifications (user_id);

-- Auto-update trigger
CREATE OR REPLACE FUNCTION identity.set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_users_updated_at
    BEFORE UPDATE ON identity.platform_users
    FOR EACH ROW EXECUTE FUNCTION identity.set_updated_at();
