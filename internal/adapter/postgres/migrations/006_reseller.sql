CREATE SCHEMA IF NOT EXISTS reseller;

CREATE TABLE reseller.tenants (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    domain TEXT,
    owner_user_id UUID NOT NULL,
    branding_config JSONB NOT NULL DEFAULT '{}',
    api_key_hash TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_tenant_domain ON reseller.tenants (domain) WHERE domain IS NOT NULL;
CREATE INDEX idx_tenant_owner ON reseller.tenants (owner_user_id);

CREATE TABLE reseller.reseller_accounts (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES reseller.tenants(id),
    user_id UUID NOT NULL,
    commission_rate INT NOT NULL CHECK (commission_rate >= 0 AND commission_rate <= 100),
    balance BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_reseller_user ON reseller.reseller_accounts (tenant_id, user_id);

CREATE TABLE reseller.commissions (
    id UUID PRIMARY KEY,
    reseller_id UUID NOT NULL REFERENCES reseller.reseller_accounts(id),
    sale_id TEXT NOT NULL,
    amount BIGINT NOT NULL,
    currency TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at TIMESTAMPTZ
);

CREATE INDEX idx_commissions_reseller ON reseller.commissions (reseller_id);
CREATE INDEX idx_commissions_status ON reseller.commissions (status);

CREATE TRIGGER trigger_tenant_updated
    BEFORE UPDATE ON reseller.tenants
    FOR EACH ROW EXECUTE FUNCTION identity.set_updated_at();
