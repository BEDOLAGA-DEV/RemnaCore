-- ============================================================================
-- Migration 048: reseller.shop_bots — deterministic bot-token hash + UNIQUE
-- index (SP6-III shared-bot-token guard).
--
-- bot_token_enc is AES-GCM with a random nonce, so two tenants configuring the
-- SAME bot token produce DIFFERENT ciphertexts — a UNIQUE(bot_token_enc) index
-- cannot detect the collision. bot_token_hash holds a deterministic keyed HMAC
-- of the plaintext token (pkg/secretbox.Box.HMAC), so identical tokens map to
-- the same hash and the partial UNIQUE index rejects the second shop.
--
-- Two tenants sharing one Telegram bot token is a cross-tenant hijack (Telegram
-- allows one webhook per token; whichever registers last wins and receives the
-- other shop's updates). This guard makes SetShopBot fail closed on duplicates.
--
-- The index is PARTIAL (WHERE bot_token_hash IS NOT NULL) so pre-existing rows
-- without a hash (backfilled lazily on next SetShopBot) do not collide on NULL.
-- ============================================================================
SET row_security = off;

ALTER TABLE reseller.shop_bots
    ADD COLUMN IF NOT EXISTS bot_token_hash text;

CREATE UNIQUE INDEX IF NOT EXISTS idx_shop_bots_token_hash
    ON reseller.shop_bots (bot_token_hash)
    WHERE bot_token_hash IS NOT NULL;

COMMENT ON COLUMN reseller.shop_bots.bot_token_hash IS
    'Deterministic keyed HMAC of the plaintext bot token (secretbox.Box.HMAC); UNIQUE to prevent two shops sharing one Telegram bot token.';
