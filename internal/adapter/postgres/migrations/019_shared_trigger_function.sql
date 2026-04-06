-- ============================================================================
-- Migration 019: Move set_updated_at() to public schema
-- ============================================================================
-- Eliminates cross-schema dependency where billing/multisub/plugins/payment/
-- reseller triggers referenced identity.set_updated_at(). The function is now
-- in the public schema (shared, context-agnostic).
-- ============================================================================

-- Create the function in public schema (identical logic).
CREATE OR REPLACE FUNCTION public.set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Re-point all existing triggers to use public.set_updated_at().

-- Identity (original trigger name: trigger_users_updated_at)
DROP TRIGGER IF EXISTS trigger_users_updated_at ON identity.platform_users;
CREATE TRIGGER trigger_users_updated_at
    BEFORE UPDATE ON identity.platform_users
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

-- Billing
DROP TRIGGER IF EXISTS trigger_plans_updated ON billing.plans;
CREATE TRIGGER trigger_plans_updated
    BEFORE UPDATE ON billing.plans
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

DROP TRIGGER IF EXISTS trigger_subs_updated ON billing.subscriptions;
CREATE TRIGGER trigger_subs_updated
    BEFORE UPDATE ON billing.subscriptions
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

DROP TRIGGER IF EXISTS trigger_invoices_updated ON billing.invoices;
CREATE TRIGGER trigger_invoices_updated
    BEFORE UPDATE ON billing.invoices
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

DROP TRIGGER IF EXISTS trigger_family_updated ON billing.family_groups;
CREATE TRIGGER trigger_family_updated
    BEFORE UPDATE ON billing.family_groups
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

-- Multisub
DROP TRIGGER IF EXISTS trigger_bindings_updated ON multisub.remnawave_bindings;
CREATE TRIGGER trigger_bindings_updated
    BEFORE UPDATE ON multisub.remnawave_bindings
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

DROP TRIGGER IF EXISTS trigger_saga_instances_updated ON multisub.saga_instances;
CREATE TRIGGER trigger_saga_instances_updated
    BEFORE UPDATE ON multisub.saga_instances
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

-- Plugins
DROP TRIGGER IF EXISTS trigger_plugin_registry_updated ON plugins.plugin_registry;
CREATE TRIGGER trigger_plugin_registry_updated
    BEFORE UPDATE ON plugins.plugin_registry
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

DROP TRIGGER IF EXISTS trigger_plugin_storage_updated ON plugins.plugin_storage;
CREATE TRIGGER trigger_plugin_storage_updated
    BEFORE UPDATE ON plugins.plugin_storage
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

-- Payment
DROP TRIGGER IF EXISTS trigger_payment_updated ON payment.payment_records;
CREATE TRIGGER trigger_payment_updated
    BEFORE UPDATE ON payment.payment_records
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

-- Reseller
DROP TRIGGER IF EXISTS trigger_tenant_updated ON reseller.tenants;
CREATE TRIGGER trigger_tenant_updated
    BEFORE UPDATE ON reseller.tenants
    FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();

-- NOTE: The original identity.set_updated_at() is NOT dropped to maintain
-- backward compatibility with any external tools referencing it.
