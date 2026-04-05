//go:build integration

package postgres_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// billingPlanGistIndex is the exclusion constraint that implicitly creates
	// a GiST index on (user_id, plan_id, billing_period).
	billingPlanGistIndex = "uq_subs_user_plan_no_overlap"

	// seedSubscriptionCount is the number of test subscriptions inserted to
	// encourage the planner to prefer index scans over sequential scans.
	// With fewer rows the planner may choose a seq scan regardless of index
	// availability.
	seedSubscriptionCount = 200

	// testSubscriptionStatus is a non-terminal status used for seeded data
	// so the exclusion constraint's WHERE clause applies.
	testSubscriptionStatus = "active"
)

// setupBillingDB starts a PostgreSQL 18 container with identity + billing +
// PG18 feature migrations applied. Returns a connected pool. The container is
// terminated when the test finishes.
func setupBillingDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	migrationPath, err := filepath.Abs("migrations")
	require.NoError(t, err)

	ctr, err := tcpostgres.Run(ctx,
		"postgres:18",
		tcpostgres.WithDatabase(testDBName),
		tcpostgres.WithUsername(testDBUser),
		tcpostgres.WithPassword(testDBPass),
		tcpostgres.WithInitScripts(
			filepath.Join(migrationPath, "001_identity.sql"),
			filepath.Join(migrationPath, "002_billing.sql"),
			filepath.Join(migrationPath, "011_pg18_features.sql"),
		),
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

// seedBillingSubscriptions inserts a plan and n subscriptions with
// non-overlapping billing periods, each assigned to a unique user.
// Returns the plan ID and the user ID of the last inserted subscription.
func seedBillingSubscriptions(t *testing.T, pool *pgxpool.Pool, n int) (planID string, lastUserID string) {
	t.Helper()
	ctx := context.Background()

	planID = uuid.Must(uuid.NewV7()).String()

	_, err := pool.Exec(ctx, `
		INSERT INTO billing.plans (
			id, name, base_price_amount, base_price_currency,
			billing_interval, traffic_limit_bytes, device_limit,
			tier, max_remnawave_bindings
		) VALUES ($1, 'Test Plan', 999, 'usd', 'month', 0, 1, 'basic', 1)
	`, planID)
	require.NoError(t, err, "failed to seed plan")

	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := range n {
		userID := uuid.Must(uuid.NewV7()).String()
		subID := uuid.Must(uuid.NewV7()).String()

		// Each subscription gets a unique 30-day period offset by index.
		periodStart := baseTime.Add(time.Duration(i) * 30 * 24 * time.Hour)
		periodEnd := periodStart.Add(30 * 24 * time.Hour)

		_, err := pool.Exec(ctx, `
			INSERT INTO billing.subscriptions (
				id, user_id, plan_id, status,
				period_start, period_end, period_interval
			) VALUES ($1, $2, $3, $4, $5, $6, 'month')
		`, subID, userID, planID, testSubscriptionStatus, periodStart, periodEnd)
		require.NoError(t, err, "failed to seed subscription %d", i)

		lastUserID = userID
	}

	// Run ANALYZE to update statistics so the planner uses indexes.
	_, err = pool.Exec(ctx, "ANALYZE billing.subscriptions")
	require.NoError(t, err, "failed to ANALYZE billing.subscriptions")

	return planID, lastUserID
}

// TestExplainBillingPeriodContainment verifies that the @> containment query
// on billing.subscriptions uses the GiST index created by the
// uq_subs_user_plan_no_overlap exclusion constraint.
func TestExplainBillingPeriodContainment(t *testing.T) {
	pool := setupBillingDB(t)
	_, lastUserID := seedBillingSubscriptions(t, pool, seedSubscriptionCount)

	// Query that should use the GiST index via @> containment.
	containmentSQL := `
		SELECT id, user_id, plan_id, status, period_start, period_end,
		       period_interval, addon_ids, assigned_to, cancelled_at,
		       paused_at, created_at, updated_at
		FROM billing.subscriptions
		WHERE user_id = $1
		  AND billing_period @> $2::timestamptz
		  AND status IN ('trial', 'active', 'past_due')
		LIMIT 1
	`

	// Pick a timestamp inside the last subscription's billing period.
	queryTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).
		Add(time.Duration(seedSubscriptionCount-1) * 30 * 24 * time.Hour).
		Add(15 * 24 * time.Hour) // mid-period

	plan := ExplainPlan(t, pool, containmentSQL, lastUserID, queryTime)

	t.Logf("Query plan:\n%s", plan)

	nodeType, execTime := PlanSummary(t, plan)
	t.Logf("Top node: %s, execution time: %.3fms", nodeType, execTime)

	// The GiST index should be used. With enough rows the planner prefers
	// an index scan. The constraint name doubles as the index name.
	AssertIndexUsedStrict(t, plan, billingPlanGistIndex)
	AssertNoSeqScan(t, plan)
}

// TestExplainBillingPeriodOverlap verifies that the && overlap query on
// billing.subscriptions uses the GiST index.
func TestExplainBillingPeriodOverlap(t *testing.T) {
	pool := setupBillingDB(t)
	planID, lastUserID := seedBillingSubscriptions(t, pool, seedSubscriptionCount)

	overlapSQL := `
		SELECT id, user_id, plan_id, status, period_start, period_end,
		       period_interval, addon_ids, assigned_to, cancelled_at,
		       paused_at, created_at, updated_at
		FROM billing.subscriptions
		WHERE user_id = $1
		  AND plan_id = $2
		  AND billing_period && tstzrange($3, $4, '[)')
		  AND status IN ('trial', 'active', 'past_due', 'paused')
	`

	// Overlap with the last subscription's period.
	overlapStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).
		Add(time.Duration(seedSubscriptionCount-1) * 30 * 24 * time.Hour)
	overlapEnd := overlapStart.Add(60 * 24 * time.Hour)

	plan := ExplainPlan(t, pool, overlapSQL, lastUserID, planID, overlapStart, overlapEnd)

	t.Logf("Query plan:\n%s", plan)

	nodeType, execTime := PlanSummary(t, plan)
	t.Logf("Top node: %s, execution time: %.3fms", nodeType, execTime)

	AssertIndexUsedStrict(t, plan, billingPlanGistIndex)
	AssertNoSeqScan(t, plan)
}

// TestExplainSkipScanUserStatus verifies that the composite (user_id, status)
// index is used for queries filtering on user_id.
func TestExplainSkipScanUserStatus(t *testing.T) {
	pool := setupBillingDB(t)
	_, lastUserID := seedBillingSubscriptions(t, pool, seedSubscriptionCount)

	userStatusSQL := `
		SELECT id, user_id, plan_id, status, period_start, period_end,
		       period_interval, addon_ids, assigned_to, cancelled_at,
		       paused_at, created_at, updated_at
		FROM billing.subscriptions
		WHERE user_id = $1
		  AND status IN ('trial', 'active', 'past_due')
	`

	plan := ExplainPlan(t, pool, userStatusSQL, lastUserID)

	t.Logf("Query plan:\n%s", plan)

	nodeType, execTime := PlanSummary(t, plan)
	t.Logf("Top node: %s, execution time: %.3fms", nodeType, execTime)

	// Should use the composite index, not a seq scan.
	AssertNoSeqScan(t, plan)
	AssertIndexUsed(t, plan, "idx_subs_user_status")
}
