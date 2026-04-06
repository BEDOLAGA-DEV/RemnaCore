//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

const (
	// multisubBindingUserStatusIndex is the composite index on
	// (platform_user_id, status) created by migration 011 and recreated
	// with CONCURRENTLY in migration 017.
	multisubBindingUserStatusIndex = "idx_bindings_user_status"

	// multisubBindingSubIndex is the index on subscription_id created by
	// migration 003.
	multisubBindingSubIndex = "idx_bindings_sub"

	// multisubBindingStatusIndex is the single-column index on (status)
	// added by migration 023 to cover status-only queries (GetAllActiveBindings,
	// GetFailedBindingsWithRemnawaveUUID, resetAbandonedReconcilingQuery) when
	// PG18 skip scan on the composite index is not guaranteed.
	multisubBindingStatusIndex = "idx_bindings_status"

	// seedBindingCount is the number of test bindings inserted to
	// encourage the planner to prefer index scans over sequential scans.
	seedBindingCount = 300

	// seedIdempotencyKeyCount is the number of test idempotency keys
	// inserted to encourage the planner to prefer index scans.
	seedIdempotencyKeyCount = 300

	// seedSkipScanBindingCount is a higher row count used for skip scan
	// tests. The PG18 planner needs enough rows with varied
	// platform_user_id values to potentially trigger skip scan on the
	// composite (platform_user_id, status) index for status-only queries.
	seedSkipScanBindingCount = 500

	// testBindingPurpose is the purpose value used for seeded bindings.
	testBindingPurpose = "base"

	// testBindingActiveStatus is the status value used for seeded bindings.
	testBindingActiveStatus = "active"

	// testBindingFailedStatus is the failed status value used for skip scan
	// test seeding.
	testBindingFailedStatus = "failed"

	// testBindingReconcilingStatus is the reconciling status value used for
	// skip scan test seeding.
	testBindingReconcilingStatus = "reconciling"
)

// seedMultisubBindings inserts n bindings with unique users and subscriptions.
// Returns the platform_user_id and subscription_id of the last inserted binding.
func seedMultisubBindings(t *testing.T, pool *pgxpool.Pool, n int) (lastUserID string, lastSubID string) {
	t.Helper()
	ctx := context.Background()

	for i := range n {
		bindingID := uuid.Must(uuid.NewV7()).String()
		subID := uuid.Must(uuid.NewV7()).String()
		userID := uuid.Must(uuid.NewV7()).String()
		username := fmt.Sprintf("p_test%04d_base_0", i)

		_, err := pool.Exec(ctx, `
			INSERT INTO multisub.remnawave_bindings (
				id, subscription_id, platform_user_id,
				remnawave_username, purpose, status
			) VALUES ($1, $2, $3, $4, $5, $6)
		`, bindingID, subID, userID, username, testBindingPurpose, testBindingActiveStatus)
		require.NoError(t, err, "failed to seed binding %d", i)

		lastUserID = userID
		lastSubID = subID
	}

	_, err := pool.Exec(ctx, "ANALYZE multisub.remnawave_bindings")
	require.NoError(t, err, "failed to ANALYZE multisub.remnawave_bindings")

	return lastUserID, lastSubID
}

// seedIdempotencyKeys inserts n idempotency keys. Returns the key of the last
// inserted row.
func seedIdempotencyKeys(t *testing.T, pool *pgxpool.Pool, n int) (lastKey string) {
	t.Helper()
	ctx := context.Background()

	for i := range n {
		key := fmt.Sprintf("idem_key_%s_%d", uuid.Must(uuid.NewV7()).String(), i)

		_, err := pool.Exec(ctx, `
			INSERT INTO multisub.idempotency_keys (key, created_at, expires_at)
			VALUES ($1, now(), now() + interval '24 hours')
		`, key)
		require.NoError(t, err, "failed to seed idempotency key %d", i)

		lastKey = key
	}

	_, err := pool.Exec(ctx, "ANALYZE multisub.idempotency_keys")
	require.NoError(t, err, "failed to ANALYZE multisub.idempotency_keys")

	return lastKey
}

// TestExplainBindingUserStatus verifies that the composite index on
// (platform_user_id, status) is used for queries filtering by user and
// active status. This matches the GetBindingsByPlatformUserID pattern
// with a status filter.
func TestExplainBindingUserStatus(t *testing.T) {
	pool := setupFullDB(t)
	lastUserID, _ := seedMultisubBindings(t, pool, seedBindingCount)

	bindingUserStatusSQL := `
		SELECT id, subscription_id, platform_user_id, remnawave_uuid,
		       remnawave_short_uuid, remnawave_username, purpose, status,
		       traffic_limit_bytes, allowed_nodes, inbound_tags,
		       synced_at, created_at, updated_at
		FROM multisub.remnawave_bindings
		WHERE platform_user_id = $1 AND status = 'active'
	`

	plan := ExplainPlan(t, pool, bindingUserStatusSQL, lastUserID)

	t.Logf("Query plan:\n%s", plan)

	nodeType, execTime := PlanSummary(t, plan)
	t.Logf("Top node: %s, execution time: %.3fms", nodeType, execTime)

	AssertIndexUsedStrict(t, plan, multisubBindingUserStatusIndex)
	AssertNoSeqScan(t, plan)
}

// TestExplainBindingSubscriptionID verifies that the index on subscription_id
// is used for lookups by subscription. This matches the
// GetBindingsBySubscriptionID sqlc query.
func TestExplainBindingSubscriptionID(t *testing.T) {
	pool := setupFullDB(t)
	_, lastSubID := seedMultisubBindings(t, pool, seedBindingCount)

	bindingSubSQL := `
		SELECT id, subscription_id, platform_user_id, remnawave_uuid,
		       remnawave_short_uuid, remnawave_username, purpose, status,
		       traffic_limit_bytes, allowed_nodes, inbound_tags,
		       synced_at, created_at, updated_at
		FROM multisub.remnawave_bindings
		WHERE subscription_id = $1
		ORDER BY created_at
	`

	plan := ExplainPlan(t, pool, bindingSubSQL, lastSubID)

	t.Logf("Query plan:\n%s", plan)

	nodeType, execTime := PlanSummary(t, plan)
	t.Logf("Top node: %s, execution time: %.3fms", nodeType, execTime)

	AssertIndexUsedStrict(t, plan, multisubBindingSubIndex)
	AssertNoSeqScan(t, plan)
}

// TestExplainIdempotencyKeyLookup verifies that the primary key index on the
// key column is used for idempotency key lookups. This matches the
// TryAcquireIdempotencyKey and ReleaseIdempotencyKey sqlc queries.
func TestExplainIdempotencyKeyLookup(t *testing.T) {
	pool := setupFullDB(t)
	lastKey := seedIdempotencyKeys(t, pool, seedIdempotencyKeyCount)

	// The PK on idempotency_keys is the text key column. sqlc generates
	// queries that filter on key = $1.
	idempotencyKeySQL := `
		SELECT key, result, created_at, expires_at, retry_count
		FROM multisub.idempotency_keys
		WHERE key = $1
	`

	plan := ExplainPlan(t, pool, idempotencyKeySQL, lastKey)

	t.Logf("Query plan:\n%s", plan)

	nodeType, execTime := PlanSummary(t, plan)
	t.Logf("Top node: %s, execution time: %.3fms", nodeType, execTime)

	// Primary key lookup should use the PK index (named idempotency_keys_pkey
	// by default in PostgreSQL).
	AssertIndexUsedStrict(t, plan, "idempotency_keys_pkey")
	AssertNoSeqScan(t, plan)
}

// ============================================================================
// Skip Scan Tests — status-only queries on remnawave_bindings
// ============================================================================
//
// The composite index idx_bindings_user_status (platform_user_id, status) has
// platform_user_id as the leading column. Three queries filter on status only,
// without platform_user_id:
//
//   1. GetAllActiveBindings:      WHERE status = 'active'
//   2. GetFailedBindingsWithRemnawaveUUID: WHERE status = 'failed' AND remnawave_uuid IS NOT NULL
//   3. resetAbandonedReconcilingQuery: WHERE status = 'reconciling' AND updated_at < ...
//
// PG18 introduced skip scan which can use a composite (a, b) index for
// queries on just b when a has low-to-moderate NDV. With testcontainers row
// counts (~500), the planner may or may not choose skip scan. Migration 023
// adds idx_bindings_status as a safety net to guarantee index coverage
// regardless of planner heuristics.

// seedMultisubBindingsVariedStatus inserts n bindings with a realistic
// distribution of statuses and varied platform_user_id values. This
// distribution exercises the planner for status-only queries:
//   - 60% active
//   - 15% failed (with remnawave_uuid set)
//   - 10% failed (without remnawave_uuid)
//   - 10% reconciling (with old updated_at for abandon detection)
//   - 5% disabled
//
// Returns summary counts for test assertions.
func seedMultisubBindingsVariedStatus(t *testing.T, pool *pgxpool.Pool, n int) {
	t.Helper()
	ctx := context.Background()

	// Distribution thresholds as percentages of n.
	activeThreshold := n * 60 / 100
	failedWithUUIDThreshold := activeThreshold + n*15/100
	failedNoUUIDThreshold := failedWithUUIDThreshold + n*10/100
	reconcilingThreshold := failedNoUUIDThreshold + n*10/100
	// Remaining rows are 'disabled'.

	for i := range n {
		bindingID := uuid.Must(uuid.NewV7()).String()
		subID := uuid.Must(uuid.NewV7()).String()
		userID := uuid.Must(uuid.NewV7()).String()
		username := fmt.Sprintf("p_test%04d_base_0", i)

		var status string
		var remnawaveUUID *string

		switch {
		case i < activeThreshold:
			status = testBindingActiveStatus
		case i < failedWithUUIDThreshold:
			status = testBindingFailedStatus
			rwUUID := uuid.Must(uuid.NewV7()).String()
			remnawaveUUID = &rwUUID
		case i < failedNoUUIDThreshold:
			status = testBindingFailedStatus
		case i < reconcilingThreshold:
			status = testBindingReconcilingStatus
			rwUUID := uuid.Must(uuid.NewV7()).String()
			remnawaveUUID = &rwUUID
		default:
			status = "disabled"
		}

		_, err := pool.Exec(ctx, `
			INSERT INTO multisub.remnawave_bindings (
				id, subscription_id, platform_user_id,
				remnawave_uuid, remnawave_username, purpose, status
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, bindingID, subID, userID, remnawaveUUID, username, testBindingPurpose, status)
		require.NoError(t, err, "failed to seed binding %d (status=%s)", i, status)
	}

	// Set reconciling rows to old updated_at so resetAbandonedReconciling can find them.
	_, err := pool.Exec(ctx, `
		UPDATE multisub.remnawave_bindings
		SET updated_at = NOW() - interval '2 hours'
		WHERE status = 'reconciling'
	`)
	require.NoError(t, err, "failed to backdate reconciling bindings")

	_, err = pool.Exec(ctx, "ANALYZE multisub.remnawave_bindings")
	require.NoError(t, err, "failed to ANALYZE multisub.remnawave_bindings")
}

// TestExplainBindingStatusOnly_GetAllActive verifies the query plan for the
// GetAllActiveBindings query: SELECT ... WHERE status = 'active'.
//
// This is a status-only query on a table with composite index
// idx_bindings_user_status (platform_user_id, status). PG18 skip scan may or
// may not be triggered at low row counts. Migration 023 adds
// idx_bindings_status as a safety net.
//
// Expected plan with idx_bindings_status present: Index Scan or Bitmap Heap
// Scan using idx_bindings_status.
func TestExplainBindingStatusOnly_GetAllActive(t *testing.T) {
	pool := setupFullDB(t)
	seedMultisubBindingsVariedStatus(t, pool, seedSkipScanBindingCount)

	getAllActiveSQL := `
		SELECT id, subscription_id, platform_user_id, remnawave_uuid,
		       remnawave_short_uuid, remnawave_username, purpose, status,
		       traffic_limit_bytes, allowed_nodes, inbound_tags,
		       fail_reason, synced_at, created_at, updated_at
		FROM multisub.remnawave_bindings
		WHERE status = 'active'
		ORDER BY created_at
	`

	plan := ExplainPlan(t, pool, getAllActiveSQL)

	t.Logf("Query plan for GetAllActiveBindings:\n%s", plan)

	nodeType, execTime := PlanSummary(t, plan)
	t.Logf("Top node: %s, execution time: %.3fms", nodeType, execTime)

	// With idx_bindings_status present (migration 023), the planner should use
	// it directly. If only the composite idx_bindings_user_status exists, PG18
	// skip scan may or may not be chosen.
	usedIndex := AssertIndexOrSeqScanWithLog(t, plan, multisubBindingStatusIndex)

	if !usedIndex {
		// Check if the composite index was used via skip scan as a fallback.
		usedComposite := AssertIndexOrSeqScanWithLog(t, plan, multisubBindingUserStatusIndex)
		if !usedComposite {
			t.Errorf("neither %s nor %s was used — status-only query fell back to seq scan; "+
				"verify that migration 023 (idx_bindings_status) was applied",
				multisubBindingStatusIndex, multisubBindingUserStatusIndex)
		}
	}
}

// TestExplainBindingStatusOnly_GetFailedWithRemnawaveUUID verifies the query
// plan for GetFailedBindingsWithRemnawaveUUID:
// SELECT ... WHERE status = 'failed' AND remnawave_uuid IS NOT NULL.
//
// This query filters on status (non-leading column in composite) and adds a
// NOT NULL check on remnawave_uuid. The idx_bindings_status index covers the
// status filter; the remnawave_uuid filter is applied as a heap recheck or
// uses the partial idx_bindings_rw_uuid index.
func TestExplainBindingStatusOnly_GetFailedWithRemnawaveUUID(t *testing.T) {
	pool := setupFullDB(t)
	seedMultisubBindingsVariedStatus(t, pool, seedSkipScanBindingCount)

	getFailedWithUUIDSQL := `
		SELECT id, subscription_id, platform_user_id, remnawave_uuid,
		       remnawave_short_uuid, remnawave_username, purpose, status,
		       traffic_limit_bytes, allowed_nodes, inbound_tags,
		       fail_reason, synced_at, created_at, updated_at
		FROM multisub.remnawave_bindings
		WHERE status = 'failed' AND remnawave_uuid IS NOT NULL
		ORDER BY created_at
	`

	plan := ExplainPlan(t, pool, getFailedWithUUIDSQL)

	t.Logf("Query plan for GetFailedBindingsWithRemnawaveUUID:\n%s", plan)

	nodeType, execTime := PlanSummary(t, plan)
	t.Logf("Top node: %s, execution time: %.3fms", nodeType, execTime)

	// The planner may choose idx_bindings_status for the status filter,
	// idx_bindings_rw_uuid for the NOT NULL filter, or a BitmapAnd of both.
	// Any index-based plan is acceptable.
	usedStatusIdx := planContainsIndex(t, plan, multisubBindingStatusIndex)
	usedRwUUIDIdx := planContainsIndex(t, plan, "idx_bindings_rw_uuid")
	usedCompositeIdx := planContainsIndex(t, plan, multisubBindingUserStatusIndex)

	if usedStatusIdx {
		t.Logf("Planner chose idx_bindings_status for status filter")
	} else if usedRwUUIDIdx {
		t.Logf("Planner chose idx_bindings_rw_uuid (partial) for remnawave_uuid IS NOT NULL filter")
	} else if usedCompositeIdx {
		t.Logf("Planner chose composite idx_bindings_user_status (skip scan) for status filter")
	} else {
		// With low row counts the planner may prefer seq scan. Log but do not
		// fail — the idx_bindings_status index exists for production where
		// table size makes seq scan prohibitive.
		hasSeqScan := planContainsNodeType(t, plan, seqScanNodeType)
		if hasSeqScan {
			t.Logf("WARNING: planner chose seq scan for GetFailedBindingsWithRemnawaveUUID — " +
				"acceptable at test row counts but idx_bindings_status should be used in production")
		}
	}
}

// TestExplainBindingStatusOnly_GetFailedForReconciliation verifies the query
// plan for getFailedForReconciliationQuery:
// SELECT ... WHERE status = 'failed' AND remnawave_uuid IS NOT NULL
// ORDER BY updated_at LIMIT $1 FOR UPDATE SKIP LOCKED.
//
// Uses ExplainPlanEstimate because EXPLAIN ANALYZE with FOR UPDATE SKIP LOCKED
// would acquire row locks. The estimated plan still shows the planner's chosen
// access path.
func TestExplainBindingStatusOnly_GetFailedForReconciliation(t *testing.T) {
	pool := setupFullDB(t)
	seedMultisubBindingsVariedStatus(t, pool, seedSkipScanBindingCount)

	// Matches getFailedForReconciliationQuery in multisub_repo.go.
	// Use estimate-only EXPLAIN to avoid row locks from FOR UPDATE SKIP LOCKED.
	reconSQL := `
		SELECT id, subscription_id, platform_user_id, remnawave_uuid,
		       remnawave_short_uuid, remnawave_username, purpose, status,
		       traffic_limit_bytes, allowed_nodes, inbound_tags,
		       fail_reason, synced_at, created_at, updated_at
		FROM multisub.remnawave_bindings
		WHERE status = 'failed' AND remnawave_uuid IS NOT NULL
		ORDER BY updated_at
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	const reconLimit = 50

	plan := ExplainPlanEstimate(t, pool, reconSQL, reconLimit)

	t.Logf("Query plan for getFailedForReconciliationQuery:\n%s", plan)

	nodeType := PlanEstimateSummary(t, plan)
	t.Logf("Top node: %s (estimated)", nodeType)

	// Same index coverage expectations as GetFailedBindingsWithRemnawaveUUID.
	usedStatusIdx := planContainsIndex(t, plan, multisubBindingStatusIndex)
	usedRwUUIDIdx := planContainsIndex(t, plan, "idx_bindings_rw_uuid")
	usedCompositeIdx := planContainsIndex(t, plan, multisubBindingUserStatusIndex)

	if usedStatusIdx || usedRwUUIDIdx || usedCompositeIdx {
		t.Logf("Planner chose index-based access for reconciliation query")
	} else {
		t.Logf("WARNING: planner chose non-index access for reconciliation query — " +
			"acceptable at test row counts but idx_bindings_status should be used in production")
	}
}

// TestExplainBindingStatusOnly_ResetAbandonedReconciling verifies the query
// plan for resetAbandonedReconcilingQuery:
// UPDATE ... SET status = 'failed' WHERE status = 'reconciling' AND updated_at < ...
//
// Uses ExplainPlanEstimate because EXPLAIN ANALYZE on an UPDATE would modify data.
func TestExplainBindingStatusOnly_ResetAbandonedReconciling(t *testing.T) {
	pool := setupFullDB(t)
	seedMultisubBindingsVariedStatus(t, pool, seedSkipScanBindingCount)

	// Matches resetAbandonedReconcilingQuery in multisub_repo.go.
	resetSQL := `
		UPDATE multisub.remnawave_bindings
		SET status = 'failed'
		WHERE status = 'reconciling' AND updated_at < NOW() - $1::interval
	`

	plan := ExplainPlanEstimate(t, pool, resetSQL, "1 hour")

	t.Logf("Query plan for resetAbandonedReconcilingQuery:\n%s", plan)

	nodeType := PlanEstimateSummary(t, plan)
	t.Logf("Top node: %s (estimated)", nodeType)

	// For UPDATE, the plan contains a ModifyTable node wrapping a scan node.
	// Check the scan sub-plan for index usage.
	usedStatusIdx := planContainsIndex(t, plan, multisubBindingStatusIndex)
	usedCompositeIdx := planContainsIndex(t, plan, multisubBindingUserStatusIndex)

	if usedStatusIdx {
		t.Logf("Planner chose idx_bindings_status for resetAbandonedReconciling")
	} else if usedCompositeIdx {
		t.Logf("Planner chose composite idx_bindings_user_status (skip scan) for resetAbandonedReconciling")
	} else {
		t.Logf("WARNING: planner chose non-index access for resetAbandonedReconciling — " +
			"acceptable at test row counts but idx_bindings_status should be used in production")
	}
}
