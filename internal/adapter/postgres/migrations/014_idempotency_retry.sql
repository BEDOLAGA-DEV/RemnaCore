ALTER TABLE multisub.idempotency_keys ADD COLUMN IF NOT EXISTS retry_count INT NOT NULL DEFAULT 0;
