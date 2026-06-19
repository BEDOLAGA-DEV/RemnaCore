package postgres_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tier2Migration042 is the canonical filename for the C3 Tier-2 RLS migration,
// reused by the two-shop isolation integration test (single source of truth).
const tier2Migration042 = "042_tier2_tenant_rls.sql"

// tier2RLSTables is every table that migration 042 puts under tenant RLS.
// webhook_log is intentionally absent (public ingestion, no resolvable tenant).
var tier2RLSTables = []string{
	"billing.subscriptions",
	"billing.invoices",
	"billing.invoice_line_items",
	"billing.family_groups",
	"billing.family_members",
	"payment.payment_records",
	"multisub.remnawave_bindings",
	"multisub.binding_sync_log",
}

func read042(t *testing.T) string {
	t.Helper()
	path := filepath.Join("migrations", tier2Migration042)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration %s: %v", path, err)
	}
	return string(b)
}

func TestMigration042_AllTier2TablesForced(t *testing.T) {
	sql := read042(t)
	for _, table := range tier2RLSTables {
		if !strings.Contains(sql, "ALTER TABLE "+table+" FORCE ROW LEVEL SECURITY;") {
			t.Errorf("migration 042 missing FORCE ROW LEVEL SECURITY for %s", table)
		}
	}
}

func TestMigration042_UsingClauseIsFailClosed(t *testing.T) {
	// USING must have no IS NULL branch; the only legitimate "OR tenant_id IS NULL"
	// lines belong to the WITH CHECK clauses (one per table).
	sql := read042(t)
	got := strings.Count(sql, "OR tenant_id IS NULL")
	if got != len(tier2RLSTables) {
		t.Errorf("expected exactly %d 'OR tenant_id IS NULL' lines (one WITH CHECK per table); got %d — "+
			"a fail-open IS NULL branch may have leaked into a USING clause", len(tier2RLSTables), got)
	}
}

func TestMigration042_WebhookLogExcluded(t *testing.T) {
	sql := read042(t)
	if strings.Contains(sql, "ALTER TABLE payment.webhook_log") {
		t.Error("payment.webhook_log must NOT be altered by migration 042 (public ingestion, no tenant key)")
	}
}

func TestMigration042_BackfillUnderSentinel(t *testing.T) {
	sql := read042(t)
	if !strings.Contains(sql, "SET LOCAL app.tenant_id = '*'") {
		t.Error("backfill must run under SET LOCAL app.tenant_id = '*' (the platform sentinel)")
	}
}
