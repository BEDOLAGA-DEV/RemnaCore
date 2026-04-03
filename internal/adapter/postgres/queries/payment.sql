-- ============================================================================
-- Payment Records
-- ============================================================================

-- name: CreatePaymentRecord :exec
INSERT INTO payment.payment_records (
    id, invoice_id, provider, external_id, amount, currency, status, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: GetPaymentRecordByID :one
SELECT id, invoice_id, provider, external_id, amount, currency, status, created_at, updated_at
FROM payment.payment_records WHERE id = $1;

-- name: GetPaymentRecordByExternalID :one
SELECT id, invoice_id, provider, external_id, amount, currency, status, created_at, updated_at
FROM payment.payment_records WHERE provider = $1 AND external_id = $2;

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
