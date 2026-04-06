CREATE TABLE IF NOT EXISTS identity.password_resets (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES identity.platform_users(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    token TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_password_reset_token ON identity.password_resets (token);
CREATE INDEX IF NOT EXISTS idx_password_reset_user ON identity.password_resets (user_id);
