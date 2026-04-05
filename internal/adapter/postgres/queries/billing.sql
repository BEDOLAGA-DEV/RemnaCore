-- ============================================================================
-- Plans
-- ============================================================================

-- name: CreatePlan :exec
INSERT INTO billing.plans (
    id, name, description, base_price_amount, base_price_currency,
    billing_interval, traffic_limit_bytes, device_limit,
    allowed_countries, allowed_protocols, tier,
    max_remnawave_bindings, family_enabled, max_family_members,
    is_active, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
);

-- name: GetPlanByID :one
SELECT id, name, description, base_price_amount, base_price_currency,
       billing_interval, traffic_limit_bytes, device_limit,
       allowed_countries, allowed_protocols, tier,
       max_remnawave_bindings, family_enabled, max_family_members,
       is_active, created_at, updated_at
FROM billing.plans WHERE id = $1;

-- name: GetAllPlans :many
SELECT id, name, description, base_price_amount, base_price_currency,
       billing_interval, traffic_limit_bytes, device_limit,
       allowed_countries, allowed_protocols, tier,
       max_remnawave_bindings, family_enabled, max_family_members,
       is_active, created_at, updated_at
FROM billing.plans ORDER BY created_at;

-- name: GetActivePlans :many
SELECT id, name, description, base_price_amount, base_price_currency,
       billing_interval, traffic_limit_bytes, device_limit,
       allowed_countries, allowed_protocols, tier,
       max_remnawave_bindings, family_enabled, max_family_members,
       is_active, created_at, updated_at
FROM billing.plans WHERE is_active = true ORDER BY created_at;

-- name: UpdatePlan :exec
UPDATE billing.plans
SET name = $2, description = $3, base_price_amount = $4, base_price_currency = $5,
    billing_interval = $6, traffic_limit_bytes = $7, device_limit = $8,
    allowed_countries = $9, allowed_protocols = $10, tier = $11,
    max_remnawave_bindings = $12, family_enabled = $13, max_family_members = $14,
    is_active = $15
WHERE id = $1;

-- ============================================================================
-- Plan Addons
-- ============================================================================

-- name: CreatePlanAddon :exec
INSERT INTO billing.plan_addons (
    id, plan_id, name, price_amount, price_currency,
    addon_type, extra_traffic_bytes, extra_nodes, extra_feature_flags, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: GetAddonsByPlanID :many
SELECT id, plan_id, name, price_amount, price_currency,
       addon_type, extra_traffic_bytes, extra_nodes, extra_feature_flags, created_at
FROM billing.plan_addons WHERE plan_id = $1 ORDER BY created_at;

-- name: DeleteAddonsByPlanID :exec
DELETE FROM billing.plan_addons WHERE plan_id = $1;

-- ============================================================================
-- Subscriptions
-- ============================================================================

-- name: CreateSubscription :exec
INSERT INTO billing.subscriptions (
    id, user_id, plan_id, status, period_start, period_end, period_interval,
    addon_ids, assigned_to, cancelled_at, paused_at, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

-- name: GetSubscriptionByID :one
SELECT id, user_id, plan_id, status, period_start, period_end, period_interval,
       addon_ids, assigned_to, cancelled_at, paused_at, created_at, updated_at
FROM billing.subscriptions WHERE id = $1;

-- name: GetSubscriptionsByUserID :many
SELECT id, user_id, plan_id, status, period_start, period_end, period_interval,
       addon_ids, assigned_to, cancelled_at, paused_at, created_at, updated_at
FROM billing.subscriptions WHERE user_id = $1 ORDER BY created_at DESC;

-- name: GetActiveSubscriptionsByUserID :many
SELECT id, user_id, plan_id, status, period_start, period_end, period_interval,
       addon_ids, assigned_to, cancelled_at, paused_at, created_at, updated_at
FROM billing.subscriptions WHERE user_id = $1 AND status IN ('trial', 'active') ORDER BY created_at DESC;

-- name: GetAllSubscriptions :many
SELECT id, user_id, plan_id, status, period_start, period_end, period_interval,
       addon_ids, assigned_to, cancelled_at, paused_at, created_at, updated_at
FROM billing.subscriptions ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: UpdateSubscription :exec
UPDATE billing.subscriptions
SET status = $2, period_start = $3, period_end = $4, period_interval = $5,
    addon_ids = $6, assigned_to = $7, cancelled_at = $8, paused_at = $9
WHERE id = $1;

-- NOTE: UpdateSubscriptionStatus uses PG18 native OLD/NEW RETURNING syntax
-- and is implemented as a raw pgx query in billing_repo.go (bypassing sqlc,
-- which does not yet support OLD/NEW). See SubscriptionRepository.UpdateStatus.

-- ============================================================================
-- Invoices
-- ============================================================================

-- name: CreateInvoice :exec
INSERT INTO billing.invoices (
    id, subscription_id, user_id, subtotal_amount, total_discount_amount,
    total_amount, currency, status, paid_at, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: GetInvoiceByID :one
SELECT id, subscription_id, user_id, subtotal_amount, total_discount_amount,
       total_amount, currency, status, paid_at, created_at, updated_at
FROM billing.invoices WHERE id = $1;

-- name: GetInvoicesBySubscriptionID :many
SELECT id, subscription_id, user_id, subtotal_amount, total_discount_amount,
       total_amount, currency, status, paid_at, created_at, updated_at
FROM billing.invoices WHERE subscription_id = $1 ORDER BY created_at DESC;

-- name: GetPendingInvoicesByUserID :many
SELECT id, subscription_id, user_id, subtotal_amount, total_discount_amount,
       total_amount, currency, status, paid_at, created_at, updated_at
FROM billing.invoices WHERE user_id = $1 AND status = 'pending' ORDER BY created_at DESC;

-- name: GetAllInvoices :many
SELECT id, subscription_id, user_id, subtotal_amount, total_discount_amount,
       total_amount, currency, status, paid_at, created_at, updated_at
FROM billing.invoices ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: UpdateInvoice :exec
UPDATE billing.invoices
SET status = $2, paid_at = $3, subtotal_amount = $4, total_discount_amount = $5, total_amount = $6
WHERE id = $1;

-- ============================================================================
-- Invoice Line Items
-- ============================================================================

-- name: CreateInvoiceLineItem :exec
INSERT INTO billing.invoice_line_items (invoice_id, description, item_type, amount, currency, quantity)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetLineItemsByInvoiceID :many
SELECT id, invoice_id, description, item_type, amount, currency, quantity
FROM billing.invoice_line_items WHERE invoice_id = $1 ORDER BY id;

-- name: DeleteLineItemsByInvoiceID :exec
DELETE FROM billing.invoice_line_items WHERE invoice_id = $1;

-- ============================================================================
-- Family Groups
-- ============================================================================

-- name: CreateFamilyGroup :exec
INSERT INTO billing.family_groups (id, owner_id, max_members, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5);

-- name: GetFamilyGroupByID :one
SELECT id, owner_id, max_members, created_at, updated_at
FROM billing.family_groups WHERE id = $1;

-- name: GetFamilyGroupByOwnerID :one
SELECT id, owner_id, max_members, created_at, updated_at
FROM billing.family_groups WHERE owner_id = $1;

-- name: UpdateFamilyGroup :exec
UPDATE billing.family_groups SET max_members = $2 WHERE id = $1;

-- name: DeleteFamilyGroup :exec
DELETE FROM billing.family_groups WHERE id = $1;

-- ============================================================================
-- Family Members
-- ============================================================================

-- name: CreateFamilyMember :exec
INSERT INTO billing.family_members (family_group_id, user_id, role, nickname, joined_at)
VALUES ($1, $2, $3, $4, $5);

-- name: GetFamilyMembersByGroupID :many
SELECT id, family_group_id, user_id, role, nickname, joined_at
FROM billing.family_members WHERE family_group_id = $1 ORDER BY joined_at;

-- name: DeleteFamilyMembersByGroupID :exec
DELETE FROM billing.family_members WHERE family_group_id = $1;
