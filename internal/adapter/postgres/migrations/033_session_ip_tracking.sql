-- 033_session_ip_tracking: Add IP address and user-agent tracking to sessions.
ALTER TABLE identity.sessions
    ADD COLUMN IF NOT EXISTS ip_address TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS user_agent TEXT NOT NULL DEFAULT '';
