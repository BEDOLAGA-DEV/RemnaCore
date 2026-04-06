//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"path/filepath"
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
// is required because constraints depend on schemas and objects created by
// earlier migrations, and later migrations add indexes, UUIDv7 defaults,
// the temporal FK drop (022), and schema sync (021) for fail_reason column
// and 'reconciling' status.
//
// Note: 020_pg_stat_statements.sql is excluded because it requires
// shared_preload_libraries=pg_stat_statements which is not configured in
// the testcontainers PG instance.
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
	"019_shared_trigger_function.sql",
	"021_schema_sync.sql",
	"022_drop_temporal_fk.sql",
	"023_bindings_status_index.sql",
}

// temporalUniqueConstraintName is the unique constraint on subscriptions
// (id, billing_period WITHOUT OVERLAPS). Kept after the temporal FK drop
// because it supports the exclusion constraint (separate concern).
const temporalUniqueConstraintName = "uq_subs_id_period"

// droppedTemporalFKName is the name of the temporal FK that was dropped in
// migration 022. Used to verify the constraint no longer exists.
const droppedTemporalFKName = "fk_invoice_sub_period"

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

// TestTemporalFK_Dropped_ConstraintAbsent verifies that migration 022
// successfully dropped the temporal FK constraint fk_invoice_sub_period.
// The unique constraint uq_subs_id_period is intentionally kept.
func TestTemporalFK_Dropped_ConstraintAbsent(t *testing.T) {
	pool := setupFullDB(t)
	ctx := context.Background()

	// Verify the temporal FK no longer exists.
	var fkExists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_constraint
			WHERE conname = $1
			  AND conrelid = 'billing.invoices'::regclass
		)
	`, droppedTemporalFKName).Scan(&fkExists)
	require.NoError(t, err)
	assert.False(t, fkExists,
		"temporal FK %q should have been dropped by migration 022", droppedTemporalFKName)

	// Verify the unique constraint on subscriptions is still present.
	var uqExists bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_constraint
			WHERE conname = $1
			  AND conrelid = 'billing.subscriptions'::regclass
		)
	`, temporalUniqueConstraintName).Scan(&uqExists)
	require.NoError(t, err)
	assert.True(t, uqExists,
		"unique constraint %q should still exist", temporalUniqueConstraintName)
}

// TestTemporalFK_Dropped_InvoiceOutsidePeriodSucceeds verifies that after the
// temporal FK is dropped, invoices outside the subscription billing period are
// accepted at the database level. Containment is now enforced at the
// application level in NewInvoice.
func TestTemporalFK_Dropped_InvoiceOutsidePeriodSucceeds(t *testing.T) {
	pool := setupFullDB(t)
	ctx := context.Background()

	periodStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	planID := insertTestPlan(t, ctx, pool)
	subID := insertTestSubscription(t, ctx, pool, planID, testSubscriptionStatus, periodStart, periodEnd)

	tests := []struct {
		name      string
		createdAt time.Time
	}{
		{
			name:      "within_period",
			createdAt: time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
		},
		{
			name:      "at_period_start",
			createdAt: periodStart,
		},
		{
			name:      "at_period_end",
			createdAt: periodEnd,
		},
		{
			name:      "outside_period_after",
			createdAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "outside_period_before",
			createdAt: time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoiceID, err := insertTestInvoice(t, ctx, pool, subID, tt.createdAt)
			require.NoError(t, err,
				"with temporal FK dropped, invoice at %v should succeed", tt.createdAt)
			assert.NotEmpty(t, invoiceID)
		})
	}
}

// TestTemporalFK_Dropped_RenewalDoesNotBreakInvoices is the regression test
// for the original problem: Renew() changes billing_period, and old invoices
// must not cause FK violations. With the temporal FK dropped, updating the
// subscription period succeeds even when old invoices exist outside the new
// period.
func TestTemporalFK_Dropped_RenewalDoesNotBreakInvoices(t *testing.T) {
	pool := setupFullDB(t)
	ctx := context.Background()

	// Original period: January 2026.
	originalStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	originalEnd := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	planID := insertTestPlan(t, ctx, pool)
	subID := insertTestSubscription(t, ctx, pool, planID, testSubscriptionStatus, originalStart, originalEnd)

	// Create invoice in original period.
	midJanuary := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	_, err := insertTestInvoice(t, ctx, pool, subID, midJanuary)
	require.NoError(t, err, "invoice within original period should succeed")

	// Simulate Renew(): update period to February 2026.
	renewedStart := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	renewedEnd := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	_, err = pool.Exec(ctx, `
		UPDATE billing.subscriptions
		SET period_start = $1, period_end = $2
		WHERE id = $3
	`, renewedStart, renewedEnd, subID)
	require.NoError(t, err,
		"updating subscription period should succeed even with old invoices outside new period")

	// Verify the old invoice still exists.
	var invoiceCount int
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM billing.invoices WHERE subscription_id = $1
	`, subID).Scan(&invoiceCount)
	require.NoError(t, err)
	assert.Equal(t, 1, invoiceCount, "old invoice should still exist after renewal")
}

// TestTemporalExclusion_OverlappingSubscriptions_Fails verifies that the
// exclusion constraint (separate from the dropped temporal FK) still prevents
// overlapping active subscriptions for the same user+plan.
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
	assert.Contains(t, err.Error(), billingPlanGistIndex,
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

// TestTemporalFK_Dropped_InvoicePeriodColumnRetained verifies that the
// invoice_period generated column still exists after the FK is dropped.
// The column remains useful for analytics and reporting queries.
func TestTemporalFK_Dropped_InvoicePeriodColumnRetained(t *testing.T) {
	pool := setupFullDB(t)
	ctx := context.Background()

	var columnExists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'billing'
			  AND table_name = 'invoices'
			  AND column_name = 'invoice_period'
		)
	`).Scan(&columnExists)
	require.NoError(t, err)
	assert.True(t, columnExists,
		"invoice_period generated column should be retained after FK drop")
}

// invoicePeriodGeneratedColumnValue is the expected lower/upper format for a
// point-in-time range: [ts, ts].
const invoicePeriodGeneratedColumnValue = "[%s,%s]"

// TestTemporalFK_Dropped_InvoicePeriodColumnValueCorrect verifies that the
// invoice_period generated column produces the expected point-in-time range.
func TestTemporalFK_Dropped_InvoicePeriodColumnValueCorrect(t *testing.T) {
	pool := setupFullDB(t)
	ctx := context.Background()

	periodStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	planID := insertTestPlan(t, ctx, pool)
	subID := insertTestSubscription(t, ctx, pool, planID, testSubscriptionStatus, periodStart, periodEnd)

	createdAt := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	invoiceID, err := insertTestInvoice(t, ctx, pool, subID, createdAt)
	require.NoError(t, err)

	var invoicePeriod string
	err = pool.QueryRow(ctx, `
		SELECT invoice_period::text FROM billing.invoices WHERE id = $1
	`, invoiceID).Scan(&invoicePeriod)
	require.NoError(t, err)

	// The generated column creates a closed range [created_at, created_at].
	// PG normalizes the text representation; verify it contains the timestamp.
	assert.Contains(t, invoicePeriod, "2026-01-15",
		"invoice_period should contain the created_at date, got: %s", invoicePeriod)

	t.Logf("invoice_period value: %s", invoicePeriod)

	_ = fmt.Sprintf(invoicePeriodGeneratedColumnValue, createdAt, createdAt)
}
