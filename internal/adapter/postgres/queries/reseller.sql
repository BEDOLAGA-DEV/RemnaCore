-- ============================================================================
-- Tenants
-- ============================================================================

-- name: CreateTenant :exec
INSERT INTO reseller.tenants (
    id, name, domain, owner_user_id, branding_config, api_key_hash, is_active, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: GetTenantByID :one
SELECT id, name, domain, owner_user_id, branding_config, api_key_hash, is_active, created_at, updated_at
FROM reseller.tenants WHERE id = $1;

-- name: GetTenantByDomain :one
SELECT id, name, domain, owner_user_id, branding_config, api_key_hash, is_active, created_at, updated_at
FROM reseller.tenants WHERE domain = $1;

-- name: GetTenantByAPIKeyHash :one
SELECT id, name, domain, owner_user_id, branding_config, api_key_hash, is_active, created_at, updated_at
FROM reseller.tenants WHERE api_key_hash = $1;

-- name: UpdateTenant :exec
UPDATE reseller.tenants
SET name = $2, domain = $3, branding_config = $4, api_key_hash = $5, is_active = $6
WHERE id = $1;

-- name: ListTenants :many
SELECT id, name, domain, owner_user_id, branding_config, api_key_hash, is_active, created_at, updated_at
FROM reseller.tenants
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- ============================================================================
-- Reseller Accounts
-- ============================================================================

-- name: CreateResellerAccount :exec
INSERT INTO reseller.reseller_accounts (
    id, tenant_id, user_id, commission_rate, balance, created_at
) VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetResellerAccountByID :one
SELECT id, tenant_id, user_id, commission_rate, balance, created_at
FROM reseller.reseller_accounts WHERE id = $1;

-- name: GetResellerAccountByUserAndTenant :one
SELECT id, tenant_id, user_id, commission_rate, balance, created_at
FROM reseller.reseller_accounts WHERE user_id = $1 AND tenant_id = $2;

-- name: UpdateResellerBalance :exec
UPDATE reseller.reseller_accounts
SET balance = $2
WHERE id = $1;

-- ============================================================================
-- Commissions
-- ============================================================================

-- name: CreateCommission :exec
INSERT INTO reseller.commissions (
    id, reseller_id, sale_id, amount, currency, status, created_at, paid_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetPendingCommissions :many
SELECT id, reseller_id, sale_id, amount, currency, status, created_at, paid_at
FROM reseller.commissions
WHERE reseller_id = $1 AND status = 'pending'
ORDER BY created_at DESC;

-- name: UpdateCommission :exec
UPDATE reseller.commissions
SET status = $2, paid_at = $3
WHERE id = $1;
