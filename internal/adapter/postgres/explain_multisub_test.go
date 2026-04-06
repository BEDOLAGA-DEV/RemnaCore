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

	// seedBindingCount is the number of test bindings inserted to
	// encourage the planner to prefer index scans over sequential scans.
	seedBindingCount = 300

	// seedIdempotencyKeyCount is the number of test idempotency keys
	// inserted to encourage the planner to prefer index scans.
	seedIdempotencyKeyCount = 300

	// testBindingPurpose is the purpose value used for seeded bindings.
	testBindingPurpose = "base"

	// testBindingActiveStatus is the status value used for seeded bindings.
	testBindingActiveStatus = "active"
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
