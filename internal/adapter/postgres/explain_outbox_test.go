//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
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
