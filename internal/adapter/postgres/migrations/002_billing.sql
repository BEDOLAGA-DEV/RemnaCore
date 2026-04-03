CREATE SCHEMA IF NOT EXISTS billing;

CREATE TABLE billing.plans (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    base_price_amount BIGINT NOT NULL,
    base_price_currency TEXT NOT NULL DEFAULT 'usd',
    billing_interval TEXT NOT NULL CHECK (billing_interval IN ('month', 'quarter', 'year')),
    traffic_limit_bytes BIGINT NOT NULL DEFAULT 0,
    device_limit INT NOT NULL DEFAULT 1,
    allowed_countries TEXT[] NOT NULL DEFAULT '{}',
    allowed_protocols TEXT[] NOT NULL DEFAULT '{}',
    tier TEXT NOT NULL DEFAULT 'basic' CHECK (tier IN ('basic', 'premium', 'ultra')),
    max_remnawave_bindings INT NOT NULL DEFAULT 1,
    family_enabled BOOLEAN NOT NULL DEFAULT false,
    max_family_members INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE billing.plan_addons (
    id UUID PRIMARY KEY,
    plan_id UUID NOT NULL REFERENCES billing.plans(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    price_amount BIGINT NOT NULL,
    price_currency TEXT NOT NULL DEFAULT 'usd',
    addon_type TEXT NOT NULL CHECK (addon_type IN ('traffic', 'nodes', 'feature')),
    extra_traffic_bytes BIGINT NOT NULL DEFAULT 0,
    extra_nodes TEXT[] NOT NULL DEFAULT '{}',
    extra_feature_flags TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_addons_plan ON billing.plan_addons (plan_id);

CREATE TABLE billing.subscriptions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    plan_id UUID NOT NULL REFERENCES billing.plans(id),
    status TEXT NOT NULL DEFAULT 'trial' CHECK (status IN ('trial', 'active', 'past_due', 'cancelled', 'expired', 'paused')),
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    period_interval TEXT NOT NULL,
    addon_ids UUID[] NOT NULL DEFAULT '{}',
    assigned_to TEXT,
    cancelled_at TIMESTAMPTZ,
    paused_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_subs_user ON billing.subscriptions (user_id);
CREATE INDEX idx_subs_status ON billing.subscriptions (status);

CREATE TABLE billing.invoices (
    id UUID PRIMARY KEY,
    subscription_id UUID NOT NULL REFERENCES billing.subscriptions(id),
    user_id UUID NOT NULL,
    subtotal_amount BIGINT NOT NULL,
    total_discount_amount BIGINT NOT NULL DEFAULT 0,
    total_amount BIGINT NOT NULL,
    currency TEXT NOT NULL DEFAULT 'usd',
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'pending', 'paid', 'failed', 'refunded')),
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_invoices_sub ON billing.invoices (subscription_id);
CREATE INDEX idx_invoices_user ON billing.invoices (user_id);
CREATE INDEX idx_invoices_status ON billing.invoices (status);

CREATE TABLE billing.invoice_line_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id UUID NOT NULL REFERENCES billing.invoices(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    item_type TEXT NOT NULL CHECK (item_type IN ('plan', 'addon', 'credit')),
    amount BIGINT NOT NULL,
    currency TEXT NOT NULL,
    quantity INT NOT NULL DEFAULT 1
);

CREATE INDEX idx_line_items_invoice ON billing.invoice_line_items (invoice_id);

CREATE TABLE billing.family_groups (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL,
    max_members INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_family_owner ON billing.family_groups (owner_id);

CREATE TABLE billing.family_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    family_group_id UUID NOT NULL REFERENCES billing.family_groups(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'member')),
    nickname TEXT,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_family_members_group ON billing.family_members (family_group_id);
CREATE UNIQUE INDEX idx_family_members_unique ON billing.family_members (family_group_id, user_id);

-- Updated_at triggers
CREATE TRIGGER trigger_plans_updated
    BEFORE UPDATE ON billing.plans
    FOR EACH ROW EXECUTE FUNCTION identity.set_updated_at();

CREATE TRIGGER trigger_subs_updated
    BEFORE UPDATE ON billing.subscriptions
    FOR EACH ROW EXECUTE FUNCTION identity.set_updated_at();

CREATE TRIGGER trigger_invoices_updated
    BEFORE UPDATE ON billing.invoices
    FOR EACH ROW EXECUTE FUNCTION identity.set_updated_at();

CREATE TRIGGER trigger_family_updated
    BEFORE UPDATE ON billing.family_groups
    FOR EACH ROW EXECUTE FUNCTION identity.set_updated_at();
