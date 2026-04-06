//go:build integration

package postgres_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// allMigrations lists every migration file in sequential order. The full set
// is required because temporal FK constraints (migration 012) depend on schemas
// and objects created by earlier migrations, and later migrations add indexes
// and UUIDv7 defaults.
var allMigrations = []string{
	"001_identity.sql",
	"002_billing.sql",
	"003_multisub.sql",
	"004_plugins.sql",
	"005_payment.sql",
	"006_reseller.sql",
	"007_password_resets.sql",
	"008_outbox.sql",
	"009_outbox_sequence.sql",
	"010_wasm_cas.sql",
	"011_pg18_features.sql",
	"012_pg18_temporal_fk_outbox_part.sql",
	"013_saga_instances.sql",
	"014_idempotency_retry.sql",
	"015_uuidv7_remaining.sql",
	"016_updated_at_indexes.sql",
	"017_recreate_indexes_concurrently.sql",
	"018_row_level_security.sql",
}

// temporalFKConstraintName is the foreign key constraint that enforces invoices
// fall within their subscription's billing period.
const temporalFKConstraintName = "fk_invoice_sub_period"

// temporalUniqueConstraintName is the unique constraint on subscriptions
// (id, billing_period WITHOUT OVERLAPS) that serves as the FK target.
const temporalUniqueConstraintName = "uq_subs_id_period"

// setupFullDB starts a PostgreSQL 18 container with ALL migrations applied.
// Returns a connected pool. The container is terminated when the test finishes.
func setupFullDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	migrationPath, err := filepath.Abs("migrations")
	require.NoError(t, err)

	scripts := make([]string, len(allMigrations))
	for i, name := range allMigrations {
		scripts[i] = filepath.Join(migrationPath, name)
	}

	ctr, err := tcpostgres.Run(ctx,
		"postgres:18",
		tcpostgres.WithDatabase(testDBName),
		tcpostgres.WithUsername(testDBUser),
		tcpostgres.WithPassword(testDBPass),
		tcpostgres.WithInitScripts(scripts...),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(testContainerStartupTimeout),
		),
	)
	if err != nil {
		t.Skipf("skipping integration test: could not start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = ctr.Terminate(context.Background()) })

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return pool
}

// insertTestPlan creates a billing plan and returns its ID.
func insertTestPlan(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()
	planID := uuid.Must(uuid.NewV7()).String()

	_, err := pool.Exec(ctx, `
		INSERT INTO billing.plans (
			id, name, base_price_amount, base_price_currency,
			billing_interval, traffic_limit_bytes, device_limit,
			tier, max_remnawave_bindings
		) VALUES ($1, 'Temporal FK Test Plan', 999, 'usd', 'month', 0, 1, 'basic', 1)
	`, planID)
	require.NoError(t, err, "failed to insert test plan")

	return planID
}

// insertTestSubscription creates a subscription with the given period and
// returns its ID.
func insertTestSubscription(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	planID string,
	status string,
	periodStart time.Time,
	periodEnd time.Time,
) string {
	t.Helper()

	subID := uuid.Must(uuid.NewV7()).String()
	userID := uuid.Must(uuid.NewV7()).String()

	_, err := pool.Exec(ctx, `
		INSERT INTO billing.subscriptions (
			id, user_id, plan_id, status,
			period_start, period_end, period_interval
		) VALUES ($1, $2, $3, $4, $5, $6, 'month')
	`, subID, userID, planID, status, periodStart, periodEnd)
	require.NoError(t, err, "failed to insert test subscription")

	return subID
}

// insertTestSubscriptionWithUser creates a subscription for a specific user
// with the given period and returns the subscription ID.
func insertTestSubscriptionWithUser(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	userID string,
	planID string,
	status string,
	periodStart time.Time,
	periodEnd time.Time,
) string {
	t.Helper()

	subID := uuid.Must(uuid.NewV7()).String()

	_, err := pool.Exec(ctx, `
		INSERT INTO billing.subscriptions (
			id, user_id, plan_id, status,
			period_start, period_end, period_interval
		) VALUES ($1, $2, $3, $4, $5, $6, 'month')
	`, subID, userID, planID, status, periodStart, periodEnd)
	require.NoError(t, err, "failed to insert test subscription")

	return subID
}

// insertTestInvoice attempts to insert an invoice with the given created_at
// timestamp. Returns the invoice ID and any error.
func insertTestInvoice(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	subID string,
	createdAt time.Time,
) (string, error) {
	t.Helper()

	invoiceID := uuid.Must(uuid.NewV7()).String()
	userID := uuid.Must(uuid.NewV7()).String()

	_, err := pool.Exec(ctx, `
		INSERT INTO billing.invoices (
			id, subscription_id, user_id,
			subtotal_amount, total_amount, currency,
			status, created_at
		) VALUES ($1, $2, $3, 999, 999, 'usd', 'draft', $4)
	`, invoiceID, subID, userID, createdAt)

	return invoiceID, err
}

func TestTemporalFK_InvoiceWithinPeriod_Succeeds(t *testing.T) {
	pool := setupFullDB(t)
	ctx := context.Background()

	periodStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	planID := insertTestPlan(t, ctx, pool)
	subID := insertTestSubscription(t, ctx, pool, planID, testSubscriptionStatus, periodStart, periodEnd)

	// Invoice created_at mid-period should succeed.
	midPeriod := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	invoiceID, err := insertTestInvoice(t, ctx, pool, subID, midPeriod)
	require.NoError(t, err, "invoice within period should succeed")
	assert.NotEmpty(t, invoiceID)
}

func TestTemporalFK_InvoiceAtPeriodStart_Succeeds(t *testing.T) {
	pool := setupFullDB(t)
	ctx := context.Background()

	periodStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	planID := insertTestPlan(t, ctx, pool)
	subID := insertTestSubscription(t, ctx, pool, planID, testSubscriptionStatus, periodStart, periodEnd)

	// Invoice at exactly period_start should succeed (half-open '[)' includes start).
	invoiceID, err := insertTestInvoice(t, ctx, pool, subID, periodStart)
	require.NoError(t, err, "invoice at period start should succeed (inclusive)")
	assert.NotEmpty(t, invoiceID)
}

func TestTemporalFK_InvoiceOutsidePeriod_Fails(t *testing.T) {
	pool := setupFullDB(t)
	ctx := context.Background()

	periodStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	planID := insertTestPlan(t, ctx, pool)
	subID := insertTestSubscription(t, ctx, pool, planID, testSubscriptionStatus, periodStart, periodEnd)

	// Invoice created_at well outside the period should fail.
	outsidePeriod := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	_, err := insertTestInvoice(t, ctx, pool, subID, outsidePeriod)
	require.Error(t, err, "invoice outside period should be rejected by temporal FK")
	assert.True(t, strings.Contains(err.Error(), temporalFKConstraintName),
		"expected error to reference constraint %q, got: %v", temporalFKConstraintName, err)
}

func TestTemporalFK_InvoiceAtPeriodEnd_Fails(t *testing.T) {
	pool := setupFullDB(t)
	ctx := context.Background()

	periodStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	planID := insertTestPlan(t, ctx, pool)
	subID := insertTestSubscription(t, ctx, pool, planID, testSubscriptionStatus, periodStart, periodEnd)

	// Invoice at exactly period_end should fail (half-open '[)' excludes end).
	_, err := insertTestInvoice(t, ctx, pool, subID, periodEnd)
	require.Error(t, err, "invoice at period_end should be rejected (half-open interval excludes end)")
	assert.True(t, strings.Contains(err.Error(), temporalFKConstraintName),
		"expected error to reference constraint %q, got: %v", temporalFKConstraintName, err)
}

func TestTemporalFK_InvoiceBeforePeriodStart_Fails(t *testing.T) {
	pool := setupFullDB(t)
	ctx := context.Background()

	periodStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	planID := insertTestPlan(t, ctx, pool)
	subID := insertTestSubscription(t, ctx, pool, planID, testSubscriptionStatus, periodStart, periodEnd)

	// Invoice before period_start should fail.
	beforePeriod := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)
	_, err := insertTestInvoice(t, ctx, pool, subID, beforePeriod)
	require.Error(t, err, "invoice before period_start should be rejected by temporal FK")
	assert.True(t, strings.Contains(err.Error(), temporalFKConstraintName),
		"expected error to reference constraint %q, got: %v", temporalFKConstraintName, err)
}

func TestTemporalExclusion_OverlappingSubscriptions_Fails(t *testing.T) {
	pool := setupFullDB(t)
	ctx := context.Background()

	userID := uuid.Must(uuid.NewV7()).String()
	planID := insertTestPlan(t, ctx, pool)

	// First subscription: [2026-01-01, 2026-02-01)
	periodStart1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd1 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	_ = insertTestSubscriptionWithUser(t, ctx, pool, userID, planID, testSubscriptionStatus, periodStart1, periodEnd1)

	// Second subscription with overlapping period: [2026-01-15, 2026-02-15)
	periodStart2 := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	periodEnd2 := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)

	subID2 := uuid.Must(uuid.NewV7()).String()
	_, err := pool.Exec(ctx, `
		INSERT INTO billing.subscriptions (
			id, user_id, plan_id, status,
			period_start, period_end, period_interval
		) VALUES ($1, $2, $3, $4, $5, $6, 'month')
	`, subID2, userID, planID, testSubscriptionStatus, periodStart2, periodEnd2)

	require.Error(t, err, "overlapping active subscriptions for same user+plan should be rejected")
	assert.True(t, strings.Contains(err.Error(), billingPlanGistIndex),
		"expected error to reference exclusion constraint %q, got: %v", billingPlanGistIndex, err)
}

func TestTemporalExclusion_CancelledDoesNotBlock(t *testing.T) {
	pool := setupFullDB(t)
	ctx := context.Background()

	userID := uuid.Must(uuid.NewV7()).String()
	planID := insertTestPlan(t, ctx, pool)

	// First subscription with cancelled status: [2026-01-01, 2026-02-01)
	cancelledStatus := "cancelled"
	periodStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	_ = insertTestSubscriptionWithUser(t, ctx, pool, userID, planID, cancelledStatus, periodStart, periodEnd)

	// New active subscription with same user+plan and overlapping period should succeed
	// because cancelled subs are excluded from the exclusion constraint WHERE clause.
	subID2 := insertTestSubscriptionWithUser(t, ctx, pool, userID, planID, testSubscriptionStatus, periodStart, periodEnd)
	assert.NotEmpty(t, subID2, "active subscription should not be blocked by cancelled subscription")
}

func TestTemporalExclusion_AdjacentPeriodsSucceed(t *testing.T) {
	pool := setupFullDB(t)
	ctx := context.Background()

	userID := uuid.Must(uuid.NewV7()).String()
	planID := insertTestPlan(t, ctx, pool)

	// First period: [2026-01-01, 2026-02-01)
	periodStart1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd1 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	_ = insertTestSubscriptionWithUser(t, ctx, pool, userID, planID, testSubscriptionStatus, periodStart1, periodEnd1)

	// Adjacent period: [2026-02-01, 2026-03-01) — no overlap because '[)' is half-open.
	periodStart2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	periodEnd2 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	subID2 := insertTestSubscriptionWithUser(t, ctx, pool, userID, planID, testSubscriptionStatus, periodStart2, periodEnd2)
	assert.NotEmpty(t, subID2, "adjacent non-overlapping periods should succeed")
}
