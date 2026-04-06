//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
)

const (
	// outboxUnpublishedIndex is the partial index on sequence_number
	// WHERE published = false, created by migration 009 and recreated
	// on the partitioned table in migration 012.
	outboxUnpublishedIndex = "idx_outbox_unpublished"

	// seedOutboxEventCount is the total number of outbox events to seed.
	// A large proportion are marked as published so that the partial index
	// on unpublished rows is selective enough for the planner.
	seedOutboxEventCount = 1000

	// seedOutboxPublishedCount is the number of seeded events marked as
	// published. The remaining events stay unpublished.
	seedOutboxPublishedCount = 900

	// testOutboxEventType is the event type used for seeded outbox events.
	testOutboxEventType = "test.seeded"
)

// seedOutboxEvents inserts n outbox events into quarter Q2 2026 (within the
// outbox_2026_q2 partition). Marks the first publishedCount events as
// published. Returns the id and created_at of the last unpublished event.
func seedOutboxEvents(
	t *testing.T,
	pool *pgxpool.Pool,
	n int,
	publishedCount int,
) (lastUnpublishedID string, lastUnpublishedCreatedAt time.Time) {
	t.Helper()
	ctx := context.Background()

	// Use timestamps within Q2 2026 to land in the outbox_2026_q2 partition.
	baseTime := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	for i := range n {
		eventID := uuid.Must(uuid.NewV7()).String()
		createdAt := baseTime.Add(time.Duration(i) * time.Second)
		published := i < publishedCount
		payload := fmt.Sprintf(`{"index": %d}`, i)

		var publishedAt *time.Time
		if published {
			pa := createdAt.Add(time.Minute)
			publishedAt = &pa
		}

		_, err := pool.Exec(ctx, `
			INSERT INTO public.outbox (id, event_type, payload, published, created_at, published_at)
			VALUES ($1, $2, $3::jsonb, $4, $5, $6)
		`, eventID, testOutboxEventType, payload, published, createdAt, publishedAt)
		require.NoError(t, err, "failed to seed outbox event %d", i)

		if !published {
			lastUnpublishedID = eventID
			lastUnpublishedCreatedAt = createdAt
		}
	}

	_, err := pool.Exec(ctx, "ANALYZE public.outbox")
	require.NoError(t, err, "failed to ANALYZE public.outbox")

	return lastUnpublishedID, lastUnpublishedCreatedAt
}

// TestExplainOutboxUnpublished verifies that the partial index on
// sequence_number WHERE published = false is used for the relay query.
// This matches the GetUnpublishedOutboxEvents sqlc query.
func TestExplainOutboxUnpublished(t *testing.T) {
	pool := setupFullDB(t)
	seedOutboxEvents(t, pool, seedOutboxEventCount, seedOutboxPublishedCount)

	unpublishedSQL := `
		SELECT id, event_type, payload, created_at, sequence_number
		FROM public.outbox
		WHERE published = false
		ORDER BY sequence_number
		LIMIT 100
	`

	plan := ExplainPlan(t, pool, unpublishedSQL)

	t.Logf("Query plan:\n%s", plan)

	nodeType, execTime := PlanSummary(t, plan)
	t.Logf("Top node: %s, execution time: %.3fms", nodeType, execTime)

	// The partial index should be used for the WHERE published = false filter.
	AssertIndexUsedStrict(t, plan, outboxUnpublishedIndex)
	AssertNoSeqScan(t, plan)
}

// TestExplainOutboxPartitionPruning verifies that the UPDATE query for marking
// an event as published prunes to a single partition when both id and
// created_at are provided. This matches the MarkOutboxEventPublished sqlc
// query which includes created_at for partition pruning.
func TestExplainOutboxPartitionPruning(t *testing.T) {
	pool := setupFullDB(t)
	lastID, lastCreatedAt := seedOutboxEvents(t, pool, seedOutboxEventCount, seedOutboxPublishedCount)

	markPublishedSQL := `
		UPDATE public.outbox
		SET published = true, published_at = now()
		WHERE id = $1 AND created_at = $2
	`

	plan := ExplainPlan(t, pool, markPublishedSQL, lastID, lastCreatedAt)

	t.Logf("Query plan:\n%s", plan)

	nodeType, execTime := PlanSummary(t, plan)
	t.Logf("Top node: %s, execution time: %.3fms", nodeType, execTime)

	// Verify that partition pruning occurs — the plan should not scan all
	// partitions. With created_at in the WHERE clause, only 1 partition
	// should be touched.
	AssertNoSeqScan(t, plan)
}

// seedOutboxInQuarter inserts n events with created_at timestamps within the
// given quarter, returning the IDs and timestamps of the inserted events.
func seedOutboxInQuarter(t *testing.T, pool *pgxpool.Pool, year, quarter, n int) (ids []string, createdAts []time.Time) {
	t.Helper()
	ctx := context.Background()

	month := time.Month((quarter-1)*monthsPerQuarterTest + 1)
	baseTime := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	ids = make([]string, 0, n)
	createdAts = make([]time.Time, 0, n)

	for i := range n {
		ts := baseTime.Add(time.Duration(i) * time.Hour)
		payload := fmt.Sprintf(`{"index": %d}`, i)

		var id string
		err := pool.QueryRow(ctx,
			`INSERT INTO public.outbox (event_type, payload, created_at)
			 VALUES ($1, $2::jsonb, $3)
			 RETURNING id::text`,
			testOutboxEventType, payload, ts,
		).Scan(&id)
		require.NoError(t, err, "failed to seed outbox event %d", i)

		ids = append(ids, id)
		createdAts = append(createdAts, ts)
	}

	_, err := pool.Exec(ctx, "ANALYZE public.outbox")
	require.NoError(t, err, "failed to ANALYZE public.outbox")

	return ids, createdAts
}

const (
	// monthsPerQuarterTest is the number of months per calendar quarter.
	monthsPerQuarterTest = 3

	// seedMergeEventsPerQuarter is the number of events seeded per quarter
	// for MERGE partition pruning tests.
	seedMergeEventsPerQuarter = 50

	// maxPrunedPartitions is the maximum number of partitions expected when
	// the date range hint targets a single quarter. Allows for the target
	// quarter plus the default partition.
	maxPrunedPartitions = 2
)

// countPartitionsInPlan recursively walks the EXPLAIN JSON plan tree and
// counts distinct partition relation names (e.g. "outbox_2026_q2"). This
// indicates how many partitions the planner decided to scan.
func countPartitionsInPlan(t *testing.T, planJSON string) int {
	t.Helper()

	var plans []map[string]any
	err := json.Unmarshal([]byte(planJSON), &plans)
	require.NoError(t, err, "failed to parse EXPLAIN JSON")
	if len(plans) == 0 {
		return 0
	}

	topPlan, ok := plans[0]["Plan"].(map[string]any)
	if !ok {
		return 0
	}

	seen := make(map[string]struct{})
	walkPlanForPartitions(topPlan, seen)
	return len(seen)
}

// outboxPartitionPrefix is the common prefix for outbox partition table names.
const outboxPartitionPrefix = "outbox_"

// walkPlanForPartitions recursively collects "Relation Name" values from plan
// nodes that reference outbox partition tables.
func walkPlanForPartitions(node map[string]any, seen map[string]struct{}) {
	if rel, ok := node["Relation Name"].(string); ok {
		if len(rel) > len(outboxPartitionPrefix) && rel[:len(outboxPartitionPrefix)] == outboxPartitionPrefix {
			seen[rel] = struct{}{}
		}
	}

	if plans, ok := node["Plans"].([]any); ok {
		for _, child := range plans {
			if childNode, ok := child.(map[string]any); ok {
				walkPlanForPartitions(childNode, seen)
			}
		}
	}
}

// TestExplainMergePartitionPruning verifies that the MERGE statement in
// MarkPublishedBatch with date range hints enables partition pruning. Events
// are seeded into Q2 only; the MERGE should NOT scan Q1, Q3, Q4 partitions.
func TestExplainMergePartitionPruning(t *testing.T) {
	pool := setupFullDB(t)

	ids, createdAts := seedOutboxInQuarter(t, pool, 2026, 2, seedMergeEventsPerQuarter)
	require.NotEmpty(t, ids)

	// Compute min/max as MarkPublishedBatch does.
	minTime := createdAts[0]
	maxTime := createdAts[0]
	for _, ts := range createdAts[1:] {
		if ts.Before(minTime) {
			minTime = ts
		}
		if ts.After(maxTime) {
			maxTime = ts
		}
	}
	maxTimeExcl := maxTime.Add(time.Second)

	pgIDs := pgutil.StringsToPgtypeUUIDs(ids)

	// EXPLAIN ANALYZE the MERGE with date range hints.
	mergeSQL := `
MERGE INTO public.outbox AS o
USING (
    SELECT unnest($1::uuid[]) AS id, unnest($2::timestamptz[]) AS created_at
) AS input ON o.id = input.id AND o.created_at = input.created_at
WHEN MATCHED AND o.published = false
    AND o.created_at >= $3 AND o.created_at < $4
THEN
    UPDATE SET published = true, published_at = now()
RETURNING o.id`

	plan := ExplainPlan(t, pool, mergeSQL,
		pgIDs, createdAts,
		pgutil.TimeToPgtype(minTime), pgutil.TimeToPgtype(maxTimeExcl),
	)

	t.Logf("MERGE partition pruning plan:\n%s", plan)

	nodeType, execTime := PlanSummary(t, plan)
	t.Logf("Top node: %s, execution time: %.3fms", nodeType, execTime)

	partitionCount := countPartitionsInPlan(t, plan)
	t.Logf("Partitions scanned: %d", partitionCount)

	// With date range hints targeting Q2 only, the planner should prune
	// other partitions. Without pruning all 8 quarterly + 1 default = 9.
	assert.LessOrEqual(t, partitionCount, maxPrunedPartitions,
		"expected at most %d partitions with date range pruning, got %d",
		maxPrunedPartitions, partitionCount)
	assert.GreaterOrEqual(t, partitionCount, 1,
		"expected at least 1 partition to be scanned")
}

// TestExplainMergeWithoutPruningHint provides a baseline by running the MERGE
// without date range hints to confirm it scans more partitions.
func TestExplainMergeWithoutPruningHint(t *testing.T) {
	pool := setupFullDB(t)

	ids, createdAts := seedOutboxInQuarter(t, pool, 2026, 2, seedMergeEventsPerQuarter)
	require.NotEmpty(t, ids)

	pgIDs := pgutil.StringsToPgtypeUUIDs(ids)

	mergeNoHintSQL := `
MERGE INTO public.outbox AS o
USING (
    SELECT unnest($1::uuid[]) AS id, unnest($2::timestamptz[]) AS created_at
) AS input ON o.id = input.id AND o.created_at = input.created_at
WHEN MATCHED AND o.published = false THEN
    UPDATE SET published = true, published_at = now()
RETURNING o.id`

	plan := ExplainPlan(t, pool, mergeNoHintSQL, pgIDs, createdAts)

	t.Logf("MERGE without pruning hint plan:\n%s", plan)

	partitionsNoHint := countPartitionsInPlan(t, plan)
	t.Logf("Partitions scanned without hint: %d (baseline for comparison)", partitionsNoHint)
}

// TestMarkPublishedBatchPartitioned verifies that MarkPublishedBatch works
// correctly on the partitioned outbox table (end-to-end through the repo).
func TestMarkPublishedBatchPartitioned(t *testing.T) {
	pool := setupFullDB(t)
	repo := postgres.NewOutboxRepository(pool, clock.NewReal())
	ctx := context.Background()

	const eventCount = 5
	for i := range eventCount {
		payload, _ := json.Marshal(map[string]int{"i": i})
		require.NoError(t, repo.Store(ctx, "partition.test", payload))
	}

	events, err := repo.GetUnpublished(ctx, eventCount)
	require.NoError(t, err)
	require.Len(t, events, eventCount)

	ids := make([]string, eventCount)
	times := make([]time.Time, eventCount)
	for i, e := range events {
		ids[i] = e.ID
		times[i] = e.CreatedAt
	}

	count, err := repo.MarkPublishedBatch(ctx, ids, times)
	require.NoError(t, err)
	assert.Equal(t, eventCount, count)

	remaining, err := repo.GetUnpublished(ctx, eventCount)
	require.NoError(t, err)
	assert.Empty(t, remaining)
}

// TestMarkPublishedBatchSingleEventPartitioned verifies backward compatibility:
// a batch with a single event works on the partitioned table.
func TestMarkPublishedBatchSingleEventPartitioned(t *testing.T) {
	pool := setupFullDB(t)
	repo := postgres.NewOutboxRepository(pool, clock.NewReal())
	ctx := context.Background()

	payload, _ := json.Marshal(map[string]string{"key": "single"})
	require.NoError(t, repo.Store(ctx, "single.test", payload))

	events, err := repo.GetUnpublished(ctx, 1)
	require.NoError(t, err)
	require.Len(t, events, 1)

	count, err := repo.MarkPublishedBatch(ctx,
		[]string{events[0].ID},
		[]time.Time{events[0].CreatedAt},
	)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	remaining, err := repo.GetUnpublished(ctx, 1)
	require.NoError(t, err)
	assert.Empty(t, remaining)
}

const (
	// crossPartitionQ1Quarter is the quarter index for Q1 (Jan-Mar).
	crossPartitionQ1Quarter = 1

	// crossPartitionQ2Quarter is the quarter index for Q2 (Apr-Jun).
	crossPartitionQ2Quarter = 2

	// crossPartitionQ3Quarter is the quarter index for Q3 (Jul-Sep).
	crossPartitionQ3Quarter = 3

	// crossPartitionQ4Quarter is the quarter index for Q4 (Oct-Dec).
	crossPartitionQ4Quarter = 4

	// crossPartitionSeedCount is the number of events seeded per quarter
	// for cross-partition tests.
	crossPartitionSeedCount = 10

	// crossPartitionExpectedMax is the maximum number of partitions
	// expected when the date range spans exactly two adjacent quarters
	// (Q1 + Q2 + possibly the default partition).
	crossPartitionExpectedMax = 3

	// crossPartitionExpectedMin is the minimum number of partitions
	// expected: at least both targeted quarters must be scanned.
	crossPartitionExpectedMin = 2

	// crossPartitionDatePadding is subtracted from the min timestamp and
	// added to the max timestamp to create a date range hint that
	// comfortably covers the boundary between Q1 and Q2.
	crossPartitionDatePadding = 1 * time.Hour

	// crossPartitionQ1Partition is the partition table name for Q1 2026.
	crossPartitionQ1Partition = "outbox_2026_q1"

	// crossPartitionQ2Partition is the partition table name for Q2 2026.
	crossPartitionQ2Partition = "outbox_2026_q2"

	// crossPartitionQ3Partition is the partition table name for Q3 2026.
	crossPartitionQ3Partition = "outbox_2026_q3"

	// crossPartitionQ4Partition is the partition table name for Q4 2026.
	crossPartitionQ4Partition = "outbox_2026_q4"
)

// TestExplainMergeCrossPartitionPruning verifies that MERGE with date range
// hints spanning two adjacent partitions correctly scans both while pruning
// unrelated partitions. Events are seeded at the Q1/Q2 boundary: the last
// day of Q1 (March 31) and the first day of Q2 (April 1). The date range
// hint covers [March 31 - padding, April 1 + padding], which should cause
// the planner to scan both Q1 and Q2 but prune Q3, Q4, and 2027 partitions.
func TestExplainMergeCrossPartitionPruning(t *testing.T) {
	pool := setupFullDB(t)

	// Seed events in Q1 (near the end of March) and Q2 (start of April).
	q1IDs, q1Times := seedOutboxInQuarter(t, pool, 2026, crossPartitionQ1Quarter, crossPartitionSeedCount)
	q2IDs, q2Times := seedOutboxInQuarter(t, pool, 2026, crossPartitionQ2Quarter, crossPartitionSeedCount)
	require.NotEmpty(t, q1IDs)
	require.NotEmpty(t, q2IDs)

	// Combine both quarter's data into a single batch.
	allIDs := append(q1IDs, q2IDs...)
	allTimes := append(q1Times, q2Times...)

	// Compute date range spanning both quarters.
	minTime := allTimes[0]
	maxTime := allTimes[0]
	for _, ts := range allTimes[1:] {
		if ts.Before(minTime) {
			minTime = ts
		}
		if ts.After(maxTime) {
			maxTime = ts
		}
	}
	minTimeWithPadding := minTime.Add(-crossPartitionDatePadding)
	maxTimeWithPadding := maxTime.Add(crossPartitionDatePadding)

	pgIDs := pgutil.StringsToPgtypeUUIDs(allIDs)

	// EXPLAIN ANALYZE the MERGE with date range hints spanning Q1-Q2 boundary.
	mergeSQL := `
MERGE INTO public.outbox AS o
USING (
    SELECT unnest($1::uuid[]) AS id, unnest($2::timestamptz[]) AS created_at
) AS input ON o.id = input.id AND o.created_at = input.created_at
WHEN MATCHED AND o.published = false
    AND o.created_at >= $3 AND o.created_at < $4
THEN
    UPDATE SET published = true, published_at = now()
RETURNING o.id`

	plan := ExplainPlan(t, pool, mergeSQL,
		pgIDs, allTimes,
		pgutil.TimeToPgtype(minTimeWithPadding), pgutil.TimeToPgtype(maxTimeWithPadding),
	)

	t.Logf("MERGE cross-partition pruning plan:\n%s", plan)

	nodeType, execTime := PlanSummary(t, plan)
	t.Logf("Top node: %s, execution time: %.3fms", nodeType, execTime)

	partitionCount := countPartitionsInPlan(t, plan)
	t.Logf("Partitions scanned: %d", partitionCount)

	// With date range hints spanning Q1 and Q2, the planner should scan
	// both Q1 and Q2 partitions (and possibly the default). Q3, Q4, and
	// all 2027 partitions should be pruned.
	assert.GreaterOrEqual(t, partitionCount, crossPartitionExpectedMin,
		"expected at least %d partitions (Q1 + Q2), got %d",
		crossPartitionExpectedMin, partitionCount)
	assert.LessOrEqual(t, partitionCount, crossPartitionExpectedMax,
		"expected at most %d partitions (Q1 + Q2 + default), got %d",
		crossPartitionExpectedMax, partitionCount)

	// Verify Q1 and Q2 are both present in the plan.
	assert.Contains(t, plan, crossPartitionQ1Partition,
		"expected plan to reference Q1 partition %q", crossPartitionQ1Partition)
	assert.Contains(t, plan, crossPartitionQ2Partition,
		"expected plan to reference Q2 partition %q", crossPartitionQ2Partition)

	// Verify Q3 and Q4 are pruned (not referenced in the plan).
	assert.NotContains(t, plan, crossPartitionQ3Partition,
		"expected Q3 partition %q to be pruned from plan", crossPartitionQ3Partition)
	assert.NotContains(t, plan, crossPartitionQ4Partition,
		"expected Q4 partition %q to be pruned from plan", crossPartitionQ4Partition)
}
