//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

// testContainerStartupTimeout is the maximum time to wait for the PostgreSQL
// container to become ready during integration tests.
const testContainerStartupTimeout = 30 * time.Second

// setupOutboxDB starts a PostgreSQL 18 container, applies the outbox migration,
// and returns a connected pool. The container is terminated when the test finishes.
func setupOutboxDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	migrationPath, err := filepath.Abs("migrations")
	require.NoError(t, err)

	ctr, err := tcpostgres.Run(ctx,
		"postgres:18",
		tcpostgres.WithDatabase(testDBName),
		tcpostgres.WithUsername(testDBUser),
		tcpostgres.WithPassword(testDBPass),
		tcpostgres.WithInitScripts(filepath.Join(migrationPath, "008_outbox.sql")),
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

func TestOutboxStore(t *testing.T) {
	pool := setupOutboxDB(t)
	repo := postgres.NewOutboxRepository(pool, clock.NewReal())
	ctx := context.Background()

	payload, err := json.Marshal(map[string]string{"subscription_id": "sub-123"})
	require.NoError(t, err)

	err = repo.Store(ctx, "subscription.activated", payload)
	require.NoError(t, err)

	// Verify the event was stored as unpublished.
	events, err := repo.GetUnpublished(ctx, 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "subscription.activated", events[0].EventType)
	assert.Equal(t, payload, events[0].Payload)
	assert.NotEmpty(t, events[0].ID)
	assert.False(t, events[0].CreatedAt.IsZero())
}

func TestOutboxGetUnpublished(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, repo *postgres.OutboxRepository, ctx context.Context)
		limit     int
		wantCount int
	}{
		{
			name:      "empty outbox returns empty slice",
			setup:     func(t *testing.T, _ *postgres.OutboxRepository, _ context.Context) {},
			limit:     10,
			wantCount: 0,
		},
		{
			name: "returns only unpublished events",
			setup: func(t *testing.T, repo *postgres.OutboxRepository, ctx context.Context) {
				t.Helper()
				for i := 0; i < 2; i++ {
					payload, _ := json.Marshal(map[string]int{"i": i})
					require.NoError(t, repo.Store(ctx, "test.event", payload))
				}
				events, err := repo.GetUnpublished(ctx, 10)
				require.NoError(t, err)
				require.NotEmpty(t, events)
				require.NoError(t, repo.MarkPublished(ctx, events[0].ID))
			},
			limit:     10,
			wantCount: 1,
		},
		{
			name: "respects limit parameter",
			setup: func(t *testing.T, repo *postgres.OutboxRepository, ctx context.Context) {
				t.Helper()
				for i := 0; i < 5; i++ {
					payload, _ := json.Marshal(map[string]int{"i": i})
					require.NoError(t, repo.Store(ctx, "test.event", payload))
				}
			},
			limit:     3,
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Each subtest gets a fresh DB to avoid cross-contamination.
			pool := setupOutboxDB(t)
			repo := postgres.NewOutboxRepository(pool, clock.NewReal())
			ctx := context.Background()

			tt.setup(t, repo, ctx)

			events, err := repo.GetUnpublished(ctx, tt.limit)
			require.NoError(t, err)
			assert.Len(t, events, tt.wantCount)
		})
	}
}

func TestOutboxMarkPublished(t *testing.T) {
	pool := setupOutboxDB(t)
	repo := postgres.NewOutboxRepository(pool, clock.NewReal())
	ctx := context.Background()

	payload, err := json.Marshal(map[string]string{"key": "value"})
	require.NoError(t, err)

	// Store an event.
	require.NoError(t, repo.Store(ctx, "invoice.created", payload))

	// Get the unpublished event.
	events, err := repo.GetUnpublished(ctx, 10)
	require.NoError(t, err)
	require.Len(t, events, 1)

	// Mark it as published.
	err = repo.MarkPublished(ctx, events[0].ID)
	require.NoError(t, err)

	// It should no longer appear in unpublished results.
	events, err = repo.GetUnpublished(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, events)
}
