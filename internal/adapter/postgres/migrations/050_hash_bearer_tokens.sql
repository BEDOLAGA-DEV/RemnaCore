-- ============================================================================
-- Migration 050: hash bearer tokens at rest.
--
-- refresh tokens, password-reset, email-verification and invitation tokens were
-- stored (and looked up) in PLAINTEXT. Any read of these tables — a leaked
-- backup, SQL injection, a mis-scoped query — yielded directly replayable
-- credentials (session hijack / account takeover, no cracking needed).
--
-- The application now stores and looks up hex(SHA-256(token)); the raw token is
-- only ever returned to the client at creation. This migration converts the
-- existing rows in place so active sessions and in-flight reset/verify links
-- keep working: a client still presents the raw token T, the app looks up
-- hex(sha256(T)), and the stored value is now hex(sha256(previously-raw T)).
--
-- Postgres 18 has a built-in sha256(bytea); no pgcrypto needed. The digest here
-- MUST match pkg/tokenhash.Hash (hex(sha256(utf8 bytes))).
--
-- NOT guarded by a "looks already hashed" predicate: raw tokens are themselves
-- 64-char hex (hex-encoded 32 random bytes), so such a guard would wrongly skip
-- them. Idempotency is provided by the migration ledger (this file runs once).
-- ============================================================================
SET row_security = off;

UPDATE identity.sessions
    SET refresh_token = encode(sha256(refresh_token::bytea), 'hex');

UPDATE identity.password_resets
    SET token = encode(sha256(token::bytea), 'hex');

UPDATE identity.email_verifications
    SET token = encode(sha256(token::bytea), 'hex');

UPDATE identity.invitations
    SET token = encode(sha256(token::bytea), 'hex');
