//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

// commissionsRLSMigrations is the minimal migration chain to stand up
// reseller.commissions with its 043 tenant_id + RLS policy.
var commissionsRLSMigrations = []string{
	"001_identity.sql",
	"006_reseller.sql",
	"018_row_level_security.sql",
	"028_rls_cleanup_docs_wasm.sql",
	"043_reseller_commissions_tenant.sql",
}

// DEDUP HELPER NOTE: connectAsRLSApp is NOT defined here. It is the single shared
// non-superuser helper created in C2 at internal/adapter/postgres/rls_testutil_test.go
// (package postgres_test). This file CALLS it; it must never redefine it (otherwise
// the package has two definitions and will not compile). The shared helper opens a
// pool as a NON-superuser, NON-BYPASSRLS role and t.Fatals if the connecting role
// bypasses RLS (current_setting('is_superuser')='on' OR rolbypassrls), so FORCE RLS
// is genuinely enforced (contract §8 — a superuser pool would pass vacuously).

// grantCommissionsSchemasToRLSApp grants the rls_app role (created by the shared
// connectAsRLSApp helper, which only grants the plugins schema) USAGE + DML on the
// schemas this test seeds/reads. Without these grants every statement would fail
// with "permission denied for schema" rather than exercising RLS. Grants go through
// the admin (superuser) pool. Mirrors grantTier2SchemasToRLSApp (C3).
func grantCommissionsSchemasToRLSApp(t *testing.T, ctx context.Context, admin *pgxpool.Pool) {
	t.Helper()
	for _, schema := range []string{"identity", "reseller"} {
		stmts := []string{
			fmt.Sprintf("GRANT USAGE ON SCHEMA %s TO %s", schema, testRLSRole),
			fmt.Sprintf("GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA %s TO %s", schema, testRLSRole),
		}
		for _, s := range stmts {
			_, err := admin.Exec(ctx, s)
			require.NoError(t, err)
		}
	}
}

// TestResellerCommissionsRLS_TwoShopIsolation asserts the 043 policy: a shop GUC
// sees only its own commissions, the sentinel sees all, an unset GUC sees none,
// and WITH CHECK rejects inserting a foreign tenant_id.
func TestResellerCommissionsRLS_TwoShopIsolation(t *testing.T) {
	admin, connStr := setupTestDBWith(t, commissionsRLSMigrations...)
	ctx := context.Background()
	// Connect as the shared NON-superuser rls_app role (rls_testutil_test.go, C2)
	// so FORCE RLS is actually enforced and the count assertions are NOT vacuous.
	pool := connectAsRLSApp(t, admin, connStr)
	grantCommissionsSchemasToRLSApp(t, ctx, admin)

	shopA := uuid.Must(uuid.NewV7()).String()
	shopB := uuid.Must(uuid.NewV7()).String()

	// Seed reseller accounts + commissions for both shops under the sentinel.
	seedCommission := func(tenant string) {
		ownerID := uuid.Must(uuid.NewV7()).String()
		acctID := uuid.Must(uuid.NewV7()).String()
		commID := uuid.Must(uuid.NewV7()).String()
		// system/sentinel scope for seeding (FORCE RLS WITH CHECK permits the
		// platform sentinel = tenantctx.PlatformScopeSentinel).
		seedTx, err := pool.Begin(ctx)
		require.NoError(t, err)
		_, err = seedTx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantctx.PlatformScopeSentinel)
		require.NoError(t, err)
		_, err = seedTx.Exec(ctx,
			`INSERT INTO identity.platform_users (id, email, password_hash, role, tenant_id) VALUES ($1,$2,'h','reseller',$3)`,
			ownerID, ownerID+"@x.io", tenant)
		require.NoError(t, err)
		_, err = seedTx.Exec(ctx,
			`INSERT INTO reseller.tenants (id, name, owner_user_id) VALUES ($1,'shop',$2)`,
			tenant, ownerID)
		require.NoError(t, err)
		_, err = seedTx.Exec(ctx,
			`INSERT INTO reseller.reseller_accounts (id, tenant_id, user_id, commission_rate, balance) VALUES ($1,$2,$3,10,0)`,
			acctID, tenant, ownerID)
		require.NoError(t, err)
		_, err = seedTx.Exec(ctx,
			`INSERT INTO reseller.commissions (id, reseller_id, sale_id, amount, currency, status, tenant_id) VALUES ($1,$2,'sale',500,'usd','pending',$3)`,
			commID, acctID, tenant)
		require.NoError(t, err)
		require.NoError(t, seedTx.Commit(ctx))
	}
	seedCommission(shopA)
	seedCommission(shopB)

	countUnder := func(gucValue string, setGUC bool) int {
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()
		if setGUC {
			_, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", gucValue)
			require.NoError(t, err)
		}
		var n int
		require.NoError(t, tx.QueryRow(ctx, "SELECT count(*) FROM reseller.commissions").Scan(&n))
		return n
	}

	// Sentinel sees all (2); shop A sees only A (1); shop B sees only B (1);
	// an unset GUC sees none (0, fail-closed).
	assert.Equal(t, 2, countUnder(tenantctx.PlatformScopeSentinel, true), "sentinel sees all commissions")
	assert.Equal(t, 1, countUnder(shopA, true), "shop A sees only its own commission")
	assert.Equal(t, 1, countUnder(shopB, true), "shop B sees only its own commission")
	assert.Equal(t, 0, countUnder("", false), "unset GUC sees no commissions (fail-closed)")

	// WITH CHECK: a shop GUC cannot INSERT a foreign tenant_id.
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", shopA)
	require.NoError(t, err)
	acctID := uuid.Must(uuid.NewV7()).String()
	commID := uuid.Must(uuid.NewV7()).String()
	_, err = tx.Exec(ctx,
		`INSERT INTO reseller.commissions (id, reseller_id, sale_id, amount, currency, status, tenant_id) VALUES ($1,$2,'sale',1,'usd','pending',$3)`,
		commID, acctID, shopB)
	require.Error(t, err, "shop A GUC must not insert a commission tagged shop B (WITH CHECK)")
}
