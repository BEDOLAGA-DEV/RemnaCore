-- ============================================================================
-- Migration 045: reseller.shop_bots — per-shop Telegram bot config (SP1).
-- One row per shop; bot_token_enc holds an AES-GCM blob (pkg/secretbox), never
-- plaintext. RLS sentinel form: a shop sees only its own row; the platform
-- sentinel ('*' == tenantctx.PlatformScopeSentinel) sees all (for the bot
-- manager, SP3). FORCE applied last.
-- ============================================================================
BEGIN;
SET LOCAL app.tenant_id = '*';

CREATE TABLE IF NOT EXISTS reseller.shop_bots (
    tenant_id      uuid PRIMARY KEY REFERENCES reseller.tenants(id) ON DELETE CASCADE,
    bot_token_enc  text NOT NULL,
    webhook_secret text NOT NULL,
    cabinet_url    text NOT NULL,
    bot_username   text,
    enabled        boolean NOT NULL DEFAULT false,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

DROP TRIGGER IF EXISTS trigger_shop_bots_updated ON reseller.shop_bots;
CREATE TRIGGER trigger_shop_bots_updated
    BEFORE UPDATE ON reseller.shop_bots
    FOR EACH ROW EXECUTE FUNCTION identity.set_updated_at();

COMMIT;

ALTER TABLE reseller.shop_bots ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_shop_bots ON reseller.shop_bots;
CREATE POLICY tenant_isolation_shop_bots ON reseller.shop_bots
    USING (
        current_setting('app.tenant_id', true) = '*'
        OR tenant_id::text = current_setting('app.tenant_id', true)
    )
    WITH CHECK (
        current_setting('app.tenant_id', true) = '*'
        OR tenant_id::text = current_setting('app.tenant_id', true)
    );
ALTER TABLE reseller.shop_bots FORCE ROW LEVEL SECURITY;
