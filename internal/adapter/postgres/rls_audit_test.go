//go:build integration

package postgres_test

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
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
	{"reseller", "shop_bots", policyPrefix + "shop_bots", "045"},
}

// phaseCSchemaSet returns the distinct schemas covered by phaseCTenantTables,
// sorted for deterministic output. Deriving the set from the single source of
// truth (instead of re-listing schema names) means a future Phase-C table in a
// new schema is automatically swept by the fail-open audit — it cannot escape.
func phaseCSchemaSet() []string {
	seen := make(map[string]struct{}, len(phaseCTenantTables))
	var schemas []string
	for _, tbl := range phaseCTenantTables {
		if _, ok := seen[tbl.schema]; ok {
			continue
		}
		seen[tbl.schema] = struct{}{}
		schemas = append(schemas, tbl.schema)
	}
	sort.Strings(schemas)
	return schemas
}

// setupAuditDB boots a container with the full Phase-C migration chain.
func setupAuditDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, _ := setupTestDBWith(t)
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

// TestRLSAudit_AllTablesEnableAndForceRLS verifies each Phase-C tenant table is
// both ENABLE and FORCE row-level security. FORCE is mandatory: without it the
// table owner (the runtime role) bypasses the policy, defeating the C0 boot
// assertion's premise. relrowsecurity=ENABLE, relforcerowsecurity=FORCE.
func TestRLSAudit_AllTablesEnableAndForceRLS(t *testing.T) {
	pool := setupAuditDB(t)
	ctx := context.Background()

	for _, tbl := range phaseCTenantTables {
		var enabled, forced bool
		err := pool.QueryRow(ctx,
			`SELECT c.relrowsecurity, c.relforcerowsecurity
			   FROM pg_class c
			   JOIN pg_namespace n ON n.oid = c.relnamespace
			  WHERE n.nspname = $1 AND c.relname = $2`,
			tbl.schema, tbl.table).Scan(&enabled, &forced)
		require.NoErrorf(t, err, "reading RLS flags for %s.%s", tbl.schema, tbl.table)

		assert.Truef(t, enabled,
			"%s.%s (migration %s) must have ENABLE ROW LEVEL SECURITY",
			tbl.schema, tbl.table, tbl.migration)
		assert.Truef(t, forced,
			"%s.%s (migration %s) must have FORCE ROW LEVEL SECURITY (FORCE applied last, after backfill)",
			tbl.schema, tbl.table, tbl.migration)
	}
}

// TestRLSAudit_SentinelPolicyShape verifies each Phase-C table has its
// tenant_isolation_<table> policy and that the policy is the sentinel form:
// USING references current_setting('app.tenant_id', true) = '<sentinel>' and a
// non-empty WITH CHECK exists (the write-side clause; USING-only is the old shape).
func TestRLSAudit_SentinelPolicyShape(t *testing.T) {
	pool := setupAuditDB(t)
	ctx := context.Background()

	// pg_policies renders the parsed expression in Postgres's canonical form,
	// which adds explicit ::text casts to the literal and the GUC argument. Match
	// that normalized shape, e.g.:
	//   (current_setting('app.tenant_id'::text, true) = '*'::text)
	sentinelFragment := "current_setting('app.tenant_id'::text, true) = '" + sentinelGUC + "'::text"

	for _, tbl := range phaseCTenantTables {
		policyName := tbl.policy

		var qual, withCheck *string
		err := pool.QueryRow(ctx,
			`SELECT qual, with_check
			   FROM pg_policies
			  WHERE schemaname = $1 AND tablename = $2 AND policyname = $3`,
			tbl.schema, tbl.table, policyName).Scan(&qual, &withCheck)
		require.NoErrorf(t, err,
			"policy %s on %s.%s (migration %s) must exist",
			policyName, tbl.schema, tbl.table, tbl.migration)

		require.NotNilf(t, qual, "policy %s must have a USING expression", policyName)
		assert.Containsf(t, *qual, sentinelFragment,
			"policy %s USING must match the platform sentinel via current_setting (table %s.%s, migration %s)",
			policyName, tbl.schema, tbl.table, tbl.migration)

		require.NotNilf(t, withCheck, "policy %s must have a WITH CHECK clause (write-side isolation)", policyName)
		assert.Containsf(t, *withCheck, sentinelFragment,
			"policy %s WITH CHECK must match the platform sentinel (table %s.%s, migration %s)",
			policyName, tbl.schema, tbl.table, tbl.migration)
	}
}

// TestRLSAudit_NoFailOpenUsingBranch fails if ANY policy in a Phase-C schema
// keeps a fail-open USING branch: the 018/035/036 `tenant_id IS NULL OR` NULL
// branch (makes platform-private rows visible to every shop) or the 028
// empty/unset-GUC branches. The IS NULL branch is allowed ONLY in WITH CHECK
// (tenant-less public inserts), so this inspects qual (USING) exclusively.
func TestRLSAudit_NoFailOpenUsingBranch(t *testing.T) {
	pool := setupAuditDB(t)
	ctx := context.Background()

	// Restrict to the schemas Phase C tenant-isolates so unrelated platform-only
	// tables (e.g. metrics.samples, identity.roles) are not falsely flagged.
	// Derived from phaseCTenantTables (the single source of truth) so a new
	// Phase-C schema cannot silently escape this audit.
	phaseCSchemas := phaseCSchemaSet()

	rows, err := pool.Query(ctx,
		`SELECT schemaname, tablename, policyname, qual
		   FROM pg_policies
		  WHERE schemaname = ANY($1)
		    AND qual IS NOT NULL`, phaseCSchemas)
	require.NoError(t, err)
	defer rows.Close()

	type pol struct{ schema, table, name, qual string }
	var policies []pol
	for rows.Next() {
		var p pol
		require.NoError(t, rows.Scan(&p.schema, &p.table, &p.name, &p.qual))
		policies = append(policies, p)
	}
	require.NoError(t, rows.Err())
	require.NotEmpty(t, policies, "expected RLS policies in Phase-C schemas")

	// Each fail-open USING branch is matched against the normalized qual (see
	// normalizeQual: lowercased, ::<type> casts stripped, whitespace collapsed).
	// Regexes — not plain substrings — because pg_get_expr parenthesizes every
	// boolean sub-expression, so the 018 NULL leak deparses as either
	// `tenant_id is null or ...` or `(tenant_id is null) or ...`; a fixed
	// substring would miss the parenthesized form and the check would go dead.
	for _, p := range policies {
		normalized := normalizeQual(p.qual)
		for _, marker := range failOpenUsingMarkers {
			assert.NotRegexpf(t, marker.re, normalized,
				"policy %s on %s.%s has a FAIL-OPEN USING branch (%s) — USING must contain no IS NULL / empty-GUC branch (see Phase-C contract §2); normalized qual: %s",
				p.name, p.schema, p.table, marker.desc, normalized)
		}
	}
}

// failOpenUsingMarker is one regressed fail-open USING branch the audit forbids.
type failOpenUsingMarker struct {
	re   *regexp.Regexp
	desc string
}

// failOpenUsingMarkers are the fail-open USING (read) branches that must NEVER
// appear in a Phase-C policy. Patterns run against the normalized qual and are
// tolerant of pg_get_expr deparse reformatting (casts already stripped by
// normalizeQual; optional parentheses allowed around the boolean sub-expression).
var failOpenUsingMarkers = []failOpenUsingMarker{
	// 018/035/036 NULL leak: a `tenant_id IS NULL` disjunct. pg_get_expr may
	// render it `(tenant_id is null) or ...`, so allow an optional ) before or.
	{regexp.MustCompile(`tenant_id is null\s*\)?\s+or`), "018/035/036 tenant_id IS NULL OR leak"},
	// 028 unset-GUC leak: current_setting(...) IS NULL.
	{regexp.MustCompile(`current_setting\('app\.tenant_id', true\) is null`), "028 unset-GUC IS NULL leak"},
	// 028 empty-GUC leak: current_setting(...) = '' (empty-string compare).
	{regexp.MustCompile(`current_setting\('app\.tenant_id', true\) = ''`), "028 empty-GUC = '' leak"},
}

// castRE matches PostgreSQL ::<type> cast suffixes that pg_get_expr inserts
// when deparsing a policy's qual (for example the ::text appended to an
// app.tenant_id literal or an empty string literal). The type name may contain
// interior spaces (e.g. "character varying"), so interior spaces are allowed but
// trailing whitespace is not consumed. Stripping these lets cast-free fail-open
// markers match the deparsed expression reliably.
var castRE = regexp.MustCompile(`::[a-z_]+(?: [a-z_]+)*`)

// normalizeQual canonicalises a deparsed pg_policies.qual for marker matching:
// lowercase, drop ::<type> casts, and collapse all runs of whitespace to single
// spaces. This makes fail-open detection tolerant of pg_get_expr's cast insertion
// and reformatting so a regressed 018/028-style policy is reliably caught.
// Parentheses are preserved so the current_setting(...) markers still anchor on
// the function call; the OR-branch marker handles parens itself.
func normalizeQual(qual string) string {
	lowered := strings.ToLower(qual)
	withoutCasts := castRE.ReplaceAllString(lowered, "")
	return strings.Join(strings.Fields(withoutCasts), " ")
}

// TestRLSWithCheck_NullTenantRequiresSentinel asserts the 044 hardening: under a
// shop GUC a NULL-tenant (platform) insert is rejected by WITH CHECK, while the
// platform sentinel may still create NULL-tenant rows. plugins.collections is the
// representative table (connectAsRLSApp grants only on schema plugins).
//
// Each case runs in its own transaction with set_config(..., true) so the GUC is
// transaction-local and pinned to the same physical connection as the INSERT.
// A pool-level SET would be racy: pgxpool acquires a (possibly different)
// connection per Exec, so the GUC could be absent by the time the INSERT runs.
func TestRLSWithCheck_NullTenantRequiresSentinel(t *testing.T) {
	admin, connStr := setupTestDBWith(t)
	ctx := context.Background()
	app := connectAsRLSApp(t, admin, connStr)

	shopID := uuid.Must(uuid.NewV7()).String()
	const insertSQL = "INSERT INTO plugins.collections " +
		"(plugin_slug, collection, document, tenant_id) VALUES ($1, $2, '{}'::jsonb, NULL)"

	// Shop-GUC case: a NULL-tenant insert must be rejected by WITH CHECK.
	// The INSERT errors and aborts the transaction; Rollback cleans up.
	shopTx, err := app.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = shopTx.Rollback(ctx) }()
	_, err = shopTx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", shopID)
	require.NoError(t, err)
	_, err = shopTx.Exec(ctx, insertSQL, testCollectionPlugin, testCollectionName)
	require.Error(t, err, "shop GUC must not insert a NULL-tenant (platform) row")

	// Sentinel case: the same NULL-tenant insert succeeds under PlatformScopeSentinel.
	// A fresh transaction is required because the shop-GUC INSERT above aborted
	// its transaction (WITH CHECK failure leaves it in an error state).
	sentinelTx, err := app.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = sentinelTx.Rollback(ctx) }()
	_, err = sentinelTx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantctx.PlatformScopeSentinel)
	require.NoError(t, err)
	_, err = sentinelTx.Exec(ctx, insertSQL, testCollectionPlugin, testCollectionName)
	require.NoError(t, err, "platform sentinel may insert a NULL-tenant row")
}
