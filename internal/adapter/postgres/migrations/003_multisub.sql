CREATE SCHEMA IF NOT EXISTS multisub;

CREATE TABLE IF NOT EXISTS multisub.remnawave_bindings (
    id UUID PRIMARY KEY,
    subscription_id UUID NOT NULL,
    platform_user_id UUID NOT NULL,
    remnawave_uuid TEXT,
    remnawave_short_uuid TEXT,
    remnawave_username TEXT NOT NULL,
    purpose TEXT NOT NULL CHECK (purpose IN ('base', 'gaming', 'streaming', 'family_member')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'failed', 'disabled', 'deprovisioned')),
    traffic_limit_bytes BIGINT NOT NULL DEFAULT 0,
    allowed_nodes TEXT[] NOT NULL DEFAULT '{}',
    inbound_tags TEXT[] NOT NULL DEFAULT '{}',
    synced_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_bindings_sub ON multisub.remnawave_bindings (subscription_id);
CREATE INDEX IF NOT EXISTS idx_bindings_user ON multisub.remnawave_bindings (platform_user_id);
CREATE INDEX IF NOT EXISTS idx_bindings_rw_uuid ON multisub.remnawave_bindings (remnawave_uuid) WHERE remnawave_uuid IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_bindings_status ON multisub.remnawave_bindings (status);

CREATE TABLE IF NOT EXISTS multisub.binding_sync_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    binding_id UUID NOT NULL REFERENCES multisub.remnawave_bindings(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    details JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_sync_log_binding ON multisub.binding_sync_log (binding_id);

CREATE TABLE IF NOT EXISTS multisub.idempotency_keys (
    key TEXT PRIMARY KEY,
    result JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_idempotency_expires ON multisub.idempotency_keys (expires_at);

DROP TRIGGER IF EXISTS trigger_bindings_updated ON multisub.remnawave_bindings;
CREATE TRIGGER trigger_bindings_updated
    BEFORE UPDATE ON multisub.remnawave_bindings
    FOR EACH ROW EXECUTE FUNCTION identity.set_updated_at();
