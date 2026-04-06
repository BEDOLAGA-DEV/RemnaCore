-- ============================================================================
-- Migration 015: UUIDv7 defaults for all remaining tables
-- ============================================================================
-- Ensures DB-side UUID generation is consistent across all tables.
-- Application code already uses uuid.NewV7() in Go, but direct SQL inserts
-- (e.g. seeding, ad-hoc fixes, future services) need the DB default to also
-- produce time-ordered UUIDs for B-tree locality.
--
-- Tables already using uuidv7():
--   billing.invoice_line_items  (011)
--   billing.family_members      (011)
--   multisub.binding_sync_log   (011)
--   payment.webhook_log         (011)
--   public.outbox               (012)
--   multisub.saga_instances     (013)
--
-- Tables skipped (non-UUID primary key):
--   plugins.plugin_storage      (composite TEXT PK)
--   plugins.wasm_store          (TEXT PK)
--   multisub.idempotency_keys   (TEXT PK)
-- ============================================================================

-- identity
ALTER TABLE identity.platform_users      ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE identity.sessions            ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE identity.email_verifications ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE identity.password_resets     ALTER COLUMN id SET DEFAULT uuidv7();

-- billing
ALTER TABLE billing.plans                ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE billing.plan_addons          ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE billing.subscriptions        ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE billing.invoices             ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE billing.family_groups        ALTER COLUMN id SET DEFAULT uuidv7();

-- multisub
ALTER TABLE multisub.remnawave_bindings  ALTER COLUMN id SET DEFAULT uuidv7();

-- payment
ALTER TABLE payment.payment_records      ALTER COLUMN id SET DEFAULT uuidv7();

-- plugins
ALTER TABLE plugins.plugin_registry      ALTER COLUMN id SET DEFAULT uuidv7();

-- reseller
ALTER TABLE reseller.tenants             ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE reseller.reseller_accounts   ALTER COLUMN id SET DEFAULT uuidv7();
ALTER TABLE reseller.commissions         ALTER COLUMN id SET DEFAULT uuidv7();
