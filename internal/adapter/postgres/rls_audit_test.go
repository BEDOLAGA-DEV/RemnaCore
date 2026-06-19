//go:build integration

package postgres_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

// sentinelGUC is the app.tenant_id value RLS policies treat as "all tenants".
// It mirrors tenantctx.PlatformScopeSentinel; asserted equal in TestRLSAudit_SentinelConstMatches
// so this audit never drifts from the production constant (no magic string).
const sentinelGUC = "*"

// policyPrefix is the fixed prefix of every Phase-C tenant-isolation policy name
// (full name is policyPrefix + table). Centralised so the audit and the policies
// cannot disagree silently.
const policyPrefix = "tenant_isolation_"

// auditMigrations is the full ordered migration chain applied to the audit
// container: everything that creates a Phase-C tenant table or rewrites its
// policy, in filename order. 040 rewrites 018/028/035/036 policies; 041 adds
// plugins.collections; 042 adds the Tier-2 tables; 043 adds reseller.commissions.
var auditMigrations = []string{
	"001_identity.sql",
	"002_billing.sql",
	"003_multisub.sql",
	"005_payment.sql",
	"006_reseller.sql",
	"018_row_level_security.sql",
	"028_rls_cleanup_docs_wasm.sql",
	"034_plugin_collections.sql",
	"035_balance.sql",
	"036_checkout.sql",
	"040_rls_sentinel_rewrite.sql",
	"041_plugin_collections_tenant.sql",
	"042_tier2_tenant_rls.sql",
	"043_reseller_commissions_tenant.sql",
}

// phaseCTenantTable identifies one table whose RLS the Phase-C audit covers,
// plus the migration that owns its sentinel policy (for failure messages).
type phaseCTenantTable struct {
	schema    string
	table     string
	policy    string // exact policy name as shipped (abbreviated for legacy tables)
	migration string // owning sub-phase migration, for diagnostics only
}

// phaseCTenantTables is the SINGLE source of truth for which tables Phase C
// tenant-isolates. Every audit sub-test iterates this slice — adding a table
// here automatically extends ENABLE+FORCE, policy-shape, and fail-open checks.
var phaseCTenantTables = []phaseCTenantTable{
	{"identity", "platform_users", policyPrefix + "platform_users", "040"},
	{"reseller", "reseller_accounts", policyPrefix + "reseller_accounts", "040"},
	{"balance", "wallets", policyPrefix + "wallets", "040"},
	// 040 preserves the legacy abbreviated policy name from 035.
	{"balance", "ledger_entries", policyPrefix + "ledger", "040"},
	{"balance", "topups", policyPrefix + "topups", "040"},
	// 040 preserves the legacy abbreviated policy names from 036.
	{"checkout", "sessions", policyPrefix + "checkout", "040"},
	{"checkout", "saved_methods", policyPrefix + "methods", "040"},
	{"plugins", "collections", policyPrefix + "collections", "041"},
	{"billing", "subscriptions", policyPrefix + "subscriptions", "042"},
	{"billing", "invoices", policyPrefix + "invoices", "042"},
	{"billing", "invoice_line_items", policyPrefix + "invoice_line_items", "042"},
	{"billing", "family_groups", policyPrefix + "family_groups", "042"},
	{"billing", "family_members", policyPrefix + "family_members", "042"},
	{"payment", "payment_records", policyPrefix + "payment_records", "042"},
	{"multisub", "remnawave_bindings", policyPrefix + "remnawave_bindings", "042"},
	{"multisub", "binding_sync_log", policyPrefix + "binding_sync_log", "042"},
	{"reseller", "commissions", policyPrefix + "commissions", "043"},
}

// setupAuditDB boots a container with the full Phase-C migration chain.
func setupAuditDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, _ := setupTestDBWith(t, auditMigrations...)
	return pool
}

func TestRLSAudit_PhaseCTablesPresent(t *testing.T) {
	pool := setupAuditDB(t)
	ctx := context.Background()

	for _, tbl := range phaseCTenantTables {
		var exists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS (
			   SELECT 1 FROM pg_tables WHERE schemaname = $1 AND tablename = $2
			 )`, tbl.schema, tbl.table).Scan(&exists)
		require.NoError(t, err)
		assert.Truef(t, exists,
			"Phase-C tenant table %s.%s (owned by migration %s) must exist after the migration chain",
			tbl.schema, tbl.table, tbl.migration)
	}
}

// TestRLSAudit_SentinelConstMatches pins the audit's literal to the production
// constant so the policy-shape checks below can never validate the wrong value.
func TestRLSAudit_SentinelConstMatches(t *testing.T) {
	assert.Equal(t, tenantctx.PlatformScopeSentinel, sentinelGUC,
		"audit sentinelGUC must equal tenantctx.PlatformScopeSentinel")
}

// silence the imports used only by later sub-tests in this file until they land.
var _ = strings.TrimSpace
