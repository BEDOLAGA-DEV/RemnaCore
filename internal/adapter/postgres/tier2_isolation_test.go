//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

// tier2WalletKind is the balance.wallets.wallet_kind value used by the Tier-2
// isolation test. balance.wallets has no synthetic id column — its PRIMARY KEY
// is (user_id, wallet_kind) — so the wallet identity used by the per-table
// isolation assertions is user_id under this fixed kind. Named to avoid a magic
// string in the seed/assertion path.
const tier2WalletKind = "money"

// seedTier2 inserts a minimal subscription + invoice + payment row owned by the
// given user/tenant, using the admin (superuser) pool. tenantID "" => NULL.
// Returns the subscription, invoice, and payment IDs.
func seedTier2(t *testing.T, ctx context.Context, admin *pgxpool.Pool, planID string, tenantID string) (subID, invID, payID string) {
	t.Helper()
	userID := uuid.Must(uuid.NewV7()).String()
	subID = uuid.Must(uuid.NewV7()).String()
	invID = uuid.Must(uuid.NewV7()).String()
	payID = uuid.Must(uuid.NewV7()).String()

	var tid any
	if tenantID != "" {
		tid = tenantID
	} else {
		tid = nil
	}

	_, err := admin.Exec(ctx,
		`INSERT INTO billing.subscriptions (id, user_id, plan_id, status, period_start, period_end, period_interval, tenant_id)
		 VALUES ($1, $2, $3, 'active', now(), now() + interval '30 days', 'month', $4)`,
		subID, userID, planID, tid)
	require.NoError(t, err)

	_, err = admin.Exec(ctx,
		`INSERT INTO billing.invoices (id, subscription_id, user_id, subtotal_amount, total_amount, status, tenant_id)
		 VALUES ($1, $2, $3, 1000, 1000, 'paid', $4)`,
		invID, subID, userID, tid)
	require.NoError(t, err)

	_, err = admin.Exec(ctx,
		`INSERT INTO payment.payment_records (id, invoice_id, provider, external_id, amount, currency, status, tenant_id)
		 VALUES ($1, $2, 'stripe', $3, 1000, 'USD', 'completed', $4)`,
		payID, invID, "ext-"+payID, tid)
	require.NoError(t, err)

	return subID, invID, payID
}

// seedWallet inserts one balance.wallets row owned by the given tenant via the
// admin (superuser) pool. tenantID "" => NULL (platform-private). Returns the
// wallet's user_id (balance.wallets has no synthetic id column; its PK is
// (user_id, wallet_kind)). Used to regression-guard the migration 040
// fail-open->fail-closed flip on the balance schema (#14).
func seedWallet(t *testing.T, ctx context.Context, admin *pgxpool.Pool, tenantID string) (walletUserID string) {
	t.Helper()
	walletUserID = uuid.Must(uuid.NewV7()).String()

	var tid any
	if tenantID != "" {
		tid = tenantID
	} else {
		tid = nil
	}

	_, err := admin.Exec(ctx,
		`INSERT INTO balance.wallets (user_id, wallet_kind, currency, balance_cents, tenant_id)
		 VALUES ($1, $2, 'USD', 0, $3)`,
		walletUserID, tier2WalletKind, tid)
	require.NoError(t, err)
	return walletUserID
}

func TestTier2RLS_TwoShopIsolation(t *testing.T) {
	ctx := context.Background()
	admin, baseConnStr := setupTier2DB(t)
	app := connectAsRLSApp(t, admin, baseConnStr) // shared helper (rls_testutil_test.go); t.Fatals if role bypasses RLS
	grantTier2SchemasToRLSApp(t, ctx, admin)

	// Parent plan (billing.subscriptions.plan_id REFERENCES billing.plans).
	planID := uuid.Must(uuid.NewV7()).String()
	_, err := admin.Exec(ctx,
		`INSERT INTO billing.plans (id, name, base_price_amount, base_price_currency, billing_interval, created_at, updated_at)
		 VALUES ($1, 'Test Plan', 1000, 'usd', 'month', now(), now())`,
		planID)
	require.NoError(t, err, "seed billing.plans parent")

	shopA := uuid.Must(uuid.NewV7()).String()
	shopB := uuid.Must(uuid.NewV7()).String()

	subA, invA, payA := seedTier2(t, ctx, admin, planID, shopA)
	subB, invB, payB := seedTier2(t, ctx, admin, planID, shopB)
	subNull, invNull, payNull := seedTier2(t, ctx, admin, planID, "") // platform-private

	// Wallet rows for the balance-schema isolation assertion. balance.wallets
	// went fail-open->fail-closed in migration 040; this is the regression guard
	// (#14). seedWallet returns the wallet user_id for the given tenant ("" => NULL).
	walA := seedWallet(t, ctx, admin, shopA)
	walB := seedWallet(t, ctx, admin, shopB)
	walNull := seedWallet(t, ctx, admin, "")

	// listIDs lists a text identity column from an arbitrary Tier-2 table,
	// visible to the rls_app pool under the given GUC. One generic helper drives
	// the per-table isolation assertions (subscriptions, invoices, payments,
	// wallets) so the test exercises RLS across multiple tables, not one (#14).
	listIDs := func(query, guc string) []string {
		txm := postgres.NewTxManager(app)
		var ids []string
		runCtx := ctx
		if guc != "" {
			runCtx = tenantctx.WithTenantID(ctx, guc)
		}
		err := txm.RunInTx(runCtx, func(txCtx context.Context) error {
			db := postgres.DBFromContext(txCtx, app)
			rows, qErr := db.Query(txCtx, query)
			if qErr != nil {
				return qErr
			}
			defer rows.Close()
			for rows.Next() {
				var id string
				if sErr := rows.Scan(&id); sErr != nil {
					return sErr
				}
				ids = append(ids, id)
			}
			return rows.Err()
		})
		require.NoError(t, err)
		return ids
	}
	listSubs := func(guc string) []string {
		return listIDs(`SELECT id::text FROM billing.subscriptions`, guc)
	}
	listInvoices := func(guc string) []string {
		return listIDs(`SELECT id::text FROM billing.invoices`, guc)
	}
	listPayments := func(guc string) []string {
		return listIDs(`SELECT id::text FROM payment.payment_records`, guc)
	}
	listWallets := func(guc string) []string {
		return listIDs(`SELECT user_id::text FROM balance.wallets`, guc)
	}

	t.Run("shopA_GUC_sees_only_shopA", func(t *testing.T) {
		subs := listSubs(shopA)
		assert.Contains(t, subs, subA)
		assert.NotContains(t, subs, subB, "shop A must not see shop B subscriptions")
		assert.NotContains(t, subs, subNull, "shop A must not see platform-private (NULL-tenant) rows")

		// Cross-table zero-read: invoices + payments + wallets must isolate too (#14).
		invs := listInvoices(shopA)
		assert.Contains(t, invs, invA)
		assert.NotContains(t, invs, invB, "shop A must not see shop B invoices")
		assert.NotContains(t, invs, invNull, "shop A must not see platform-private invoices")

		pays := listPayments(shopA)
		assert.Contains(t, pays, payA)
		assert.NotContains(t, pays, payB, "shop A must not see shop B payment_records")
		assert.NotContains(t, pays, payNull, "shop A must not see platform-private payment_records")

		// balance.wallets fail-open->fail-closed regression guard (migration 040, #14).
		wals := listWallets(shopA)
		assert.Contains(t, wals, walA)
		assert.NotContains(t, wals, walB, "shop A must not see shop B wallets")
		assert.NotContains(t, wals, walNull, "shop A must not see platform-private wallets")
	})

	t.Run("shopB_GUC_sees_only_shopB", func(t *testing.T) {
		subs := listSubs(shopB)
		assert.Contains(t, subs, subB)
		assert.NotContains(t, subs, subA, "shop B must not see shop A subscriptions")
		assert.NotContains(t, subs, subNull, "shop B must not see platform-private (NULL-tenant) rows")

		invs := listInvoices(shopB)
		assert.Contains(t, invs, invB)
		assert.NotContains(t, invs, invA, "shop B must not see shop A invoices")
		assert.NotContains(t, invs, invNull, "shop B must not see platform-private invoices")

		pays := listPayments(shopB)
		assert.Contains(t, pays, payB)
		assert.NotContains(t, pays, payA, "shop B must not see shop A payment_records")
		assert.NotContains(t, pays, payNull, "shop B must not see platform-private payment_records")

		wals := listWallets(shopB)
		assert.Contains(t, wals, walB)
		assert.NotContains(t, wals, walA, "shop B must not see shop A wallets")
		assert.NotContains(t, wals, walNull, "shop B must not see platform-private wallets")
	})

	t.Run("sentinel_sees_all", func(t *testing.T) {
		subs := listSubs(tenantctx.PlatformScopeSentinel)
		assert.Contains(t, subs, subA)
		assert.Contains(t, subs, subB)
		assert.Contains(t, subs, subNull, "sentinel must see platform-private rows")

		invs := listInvoices(tenantctx.PlatformScopeSentinel)
		assert.Subset(t, invs, []string{invA, invB, invNull}, "sentinel must see all invoices")

		pays := listPayments(tenantctx.PlatformScopeSentinel)
		assert.Subset(t, pays, []string{payA, payB, payNull}, "sentinel must see all payment_records")

		wals := listWallets(tenantctx.PlatformScopeSentinel)
		assert.Subset(t, wals, []string{walA, walB, walNull}, "sentinel must see all wallets")
	})

	t.Run("unset_GUC_sees_none", func(t *testing.T) {
		assert.Empty(t, listSubs(""), "unset GUC must fail closed on subscriptions")
		assert.Empty(t, listInvoices(""), "unset GUC must fail closed on invoices")
		assert.Empty(t, listPayments(""), "unset GUC must fail closed on payment_records")
		assert.Empty(t, listWallets(""), "unset GUC must fail closed on wallets (040 fail-open->fail-closed guard)")
	})

	t.Run("with_check_rejects_foreign_tenant_insert", func(t *testing.T) {
		txm := postgres.NewTxManager(app)
		err := txm.RunInTx(tenantctx.WithTenantID(ctx, shopA), func(txCtx context.Context) error {
			db := postgres.DBFromContext(txCtx, app)
			id := uuid.Must(uuid.NewV7()).String()
			uid := uuid.Must(uuid.NewV7()).String()
			_, e := db.Exec(txCtx,
				`INSERT INTO billing.subscriptions (id, user_id, plan_id, status, period_start, period_end, period_interval, tenant_id)
				 VALUES ($1, $2, $3, 'active', now(), now() + interval '30 days', 'month', $4)`,
				id, uid, planID, shopB) // foreign tenant_id while GUC = shopA
			return e
		})
		require.Error(t, err, "WITH CHECK must reject inserting a row stamped with a foreign tenant_id")
	})

	// Migration 044 gates the NULL-tenant WITH CHECK branch on the platform
	// sentinel: a tenant-less row may be written ONLY under the '*' scope, never
	// under a shop-UUID GUC (where a NULL row would otherwise escape isolation).
	t.Run("null_tenant_insert_rejected_under_shop_guc", func(t *testing.T) {
		txm := postgres.NewTxManager(app)
		err := txm.RunInTx(tenantctx.WithTenantID(ctx, shopA), func(txCtx context.Context) error {
			db := postgres.DBFromContext(txCtx, app)
			id := uuid.Must(uuid.NewV7()).String()
			uid := uuid.Must(uuid.NewV7()).String()
			_, e := db.Exec(txCtx,
				`INSERT INTO billing.subscriptions (id, user_id, plan_id, status, period_start, period_end, period_interval, tenant_id)
				 VALUES ($1, $2, $3, 'active', now(), now() + interval '30 days', 'month', NULL)`,
				id, uid, planID)
			return e
		})
		require.Error(t, err, "post-044 WITH CHECK must reject a tenant-less (NULL) insert under a shop GUC")
	})

	t.Run("null_tenant_insert_succeeds_under_platform_scope", func(t *testing.T) {
		txm := postgres.NewTxManager(app)
		err := txm.RunInTx(tenantctx.WithPlatformScope(ctx), func(txCtx context.Context) error {
			db := postgres.DBFromContext(txCtx, app)
			id := uuid.Must(uuid.NewV7()).String()
			uid := uuid.Must(uuid.NewV7()).String()
			_, e := db.Exec(txCtx,
				`INSERT INTO billing.subscriptions (id, user_id, plan_id, status, period_start, period_end, period_interval, tenant_id)
				 VALUES ($1, $2, $3, 'active', now(), now() + interval '30 days', 'month', NULL)`,
				id, uid, planID)
			return e
		})
		require.NoError(t, err, "WITH CHECK must permit a tenant-less (NULL) insert under the platform sentinel")
	})
}

// grantTier2SchemasToRLSApp grants the rls_app role (created by the shared
// connectAsRLSApp helper, which only grants the plugins schema) the privileges
// it needs to read/write the Tier-2 schemas this test exercises. Without these
// grants every query would fail with "permission denied for schema" rather than
// exercising RLS. Grants go through the admin (superuser) pool.
func grantTier2SchemasToRLSApp(t *testing.T, ctx context.Context, admin *pgxpool.Pool) {
	t.Helper()
	for _, schema := range []string{"billing", "payment", "balance"} {
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

// setupTier2DB boots postgres:18 with the migrations needed for the Tier-2 RLS
// test and returns the superuser pool plus the connection string (so a second
// non-superuser pool can be opened). Mirrors setupTestDBWith but also exposes
// the conn string. The 042 filename is single-sourced via tier2Migration042.
//
// The list carries 006/035/036/040 in addition to the 001/002/003/005/042
// Tier-2 core: migration 040 (the balance.wallets fail-open->fail-closed flip
// this test regression-guards, #14) ALTERs reseller.reseller_accounts (006),
// balance.* (035), and checkout.* (036), so those tables must exist first or
// 040 fails to apply. 018_row_level_security.sql is deliberately excluded (#6):
// it ALTERs reseller.reseller_accounts before 006 in the old harness and the
// Tier-2 RLS this test reads comes entirely from 040/042.
func setupTier2DB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx,
		"postgres:18",
		tcpostgres.WithDatabase(testDBName),
		tcpostgres.WithUsername(testDBUser),
		tcpostgres.WithPassword(testDBPass),
		tcpostgres.WithInitScripts(allMigrationScripts(t)...),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(testContainerStartupTimeout),
		),
	)
	if err != nil {
		// Harden (#6): only a Docker-absent failure is a legitimate skip. A
		// migration-apply error (e.g. a missing dependency or bad SQL) is surfaced
		// in the container logs as a startup failure — distinguish it from "no
		// Docker" so a broken migration list can never produce a vacuous skip.
		if isDockerUnavailable(err) {
			failOrSkip(t, "skipping integration test: docker/container unavailable: %v", err)
		}
		t.Fatalf("postgres container failed to start (migration apply or SQL error, NOT docker-absent): %v", err)
	}
	t.Cleanup(func() { _ = ctr.Terminate(context.Background()) })

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return pool, connStr
}

// isDockerUnavailable reports whether err is the container runtime being
// absent/unreachable (the only legitimate reason to t.Skip). Any other startup
// failure — including a migration that fails to apply — is a hard t.Fatal so a
// broken Tier-2 migration list can never masquerade as a vacuous skip (#6).
func isDockerUnavailable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// Match ONLY Docker-daemon-absence phrasing. Generic markers like "no such
	// file or directory" and "connection refused" are deliberately excluded:
	// under tcpostgres.WithInitScripts a missing migration file also surfaces as
	// "no such file or directory", so those markers would mask a broken migration
	// list as a vacuous skip (#6). A missing migration is now caught up front by
	// the os.Stat check in setupTier2DB and is always a hard t.Fatal.
	for _, marker := range []string{
		"cannot connect to the docker daemon",
		"docker: command not found",
		"permission denied while trying to connect to the docker daemon socket",
		"rootless docker not found",
		"failed to find rootless docker",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}
