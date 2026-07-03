-- ============================================================================
-- Migration 049: identity.telegram_initdata_nonces — single-use replay guard
-- for Telegram Mini App logins (SP6-III).
--
-- A valid initData HMAC only proves the payload came from Telegram, not that it
-- is fresh: within the 24h validity window the SAME signed payload can be
-- replayed to mint sessions repeatedly. This table records the SHA-256 of each
-- consumed initData; the login path claims the nonce atomically (INSERT ...
-- ON CONFLICT DO NOTHING) and rejects the request if the row already exists.
--
-- The nonce is a hash of the payload (no user data, no tenant scoping needed);
-- expires_at = now + max-age (an upper bound on the payload's validity window;
-- validation already rejects payloads past authDate + max-age) drives cheap
-- pruning of stale rows.
-- ============================================================================
SET row_security = off;

CREATE TABLE IF NOT EXISTS identity.telegram_initdata_nonces (
    nonce      text        PRIMARY KEY,
    expires_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tg_initdata_nonces_expires
    ON identity.telegram_initdata_nonces (expires_at);

COMMENT ON TABLE identity.telegram_initdata_nonces IS
    'Consumed Telegram Mini App initData hashes (SHA-256) for single-use replay prevention; rows prunable once expires_at passes.';
