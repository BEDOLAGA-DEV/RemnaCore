-- ============================================================================
-- Migration 039: Account management & invitations (RBAC Phase B)
-- ============================================================================
-- identity.invitations mirrors the token/TTL/single-use pattern of
-- email_verifications/password_resets, but is EMAIL-indexed (the invitee may not
-- exist yet): no user_id, no FK. role_key/tenant_id/commission_rate carry the
-- grant to apply on accept. No RLS (admin/owner-scoped reads filtered in app).
-- ============================================================================

CREATE TABLE IF NOT EXISTS identity.invitations (
    id              UUID        PRIMARY KEY DEFAULT uuidv7(),
    email           TEXT        NOT NULL,
    token           TEXT        NOT NULL,
    role_key        TEXT        NOT NULL,
    tenant_id       UUID,
    commission_rate INTEGER,
    invited_by      UUID,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_invitations_token ON identity.invitations (token);
CREATE INDEX IF NOT EXISTS idx_invitations_email ON identity.invitations (lower(email));
CREATE INDEX IF NOT EXISTS idx_invitations_tenant
    ON identity.invitations (tenant_id) WHERE tenant_id IS NOT NULL;

-- Admin direct-create sets this true; cleared by the password-change flow.
ALTER TABLE identity.platform_users
    ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT false;

-- Pending-owner shops (invite-owner flow) are created before the owner exists.
ALTER TABLE reseller.tenants ALTER COLUMN owner_user_id DROP NOT NULL;
