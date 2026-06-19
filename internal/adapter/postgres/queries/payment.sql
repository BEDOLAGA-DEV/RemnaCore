-- ============================================================================
-- Payment Records
-- ============================================================================

-- name: CreatePaymentRecord :exec
-- tenant_id self-stamped from the app.tenant_id GUC (sentinel '*' -> NULL
-- platform row; shop GUC -> that shop's UUID), so a shop session can never
-- write a foreign tenant_id and platform/sentinel writes yield the NULL the
-- WITH CHECK permits. Do NOT pass tenant_id as a param.
INSERT INTO payment.payment_records (
    id, invoice_id, provider, external_id, amount, currency, status, created_at, updated_at, tenant_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9,
    NULLIF(current_setting('app.tenant_id', true), '*')::uuid
);

-- name: GetPaymentRecordByID :one
SELECT id, invoice_id, provider, external_id, amount, currency, status, created_at, updated_at
FROM payment.payment_records WHERE id = $1;

-- name: GetPaymentRecordByIDForUpdate :one
SELECT id, invoice_id, provider, external_id, amount, currency, status, created_at, updated_at
FROM payment.payment_records WHERE id = $1 FOR UPDATE;

-- name: GetPaymentRecordByExternalID :one
-- Belt-and-suspenders tenant predicate (spec §5.3) behind the RLS GUC, mirroring
-- the GetPaymentRecordByExternalIDForUpdate guarded variant: sentinel '*' sees
-- all, a shop GUC sees only its own row; no IS NULL fail-open branch.
SELECT id, invoice_id, provider, external_id, amount, currency, status, created_at, updated_at
FROM payment.payment_records
WHERE provider = $1 AND external_id = $2
  AND (tenant_id::text = current_setting('app.tenant_id', true) OR current_setting('app.tenant_id', true) = '*');

-- name: GetPaymentRecordByExternalIDForUpdate :one
SELECT id, invoice_id, provider, external_id, amount, currency, status, created_at, updated_at
FROM payment.payment_records WHERE provider = $1 AND external_id = $2 FOR UPDATE;

-- name: UpdatePaymentRecord :exec
UPDATE payment.payment_records
SET status = $2, updated_at = $3
WHERE id = $1;

-- ============================================================================
-- Webhook Log
-- ============================================================================

-- name: CreateWebhookLog :exec
INSERT INTO payment.webhook_log (
    id, provider, external_id, raw_body, status, processed_at, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetWebhookLogByProviderExternalID :one
SELECT id, provider, external_id, raw_body, status, processed_at, created_at
FROM payment.webhook_log WHERE provider = $1 AND external_id = $2;

-- name: UpdateWebhookLog :exec
UPDATE payment.webhook_log
SET status = $2, processed_at = $3
WHERE id = $1;
