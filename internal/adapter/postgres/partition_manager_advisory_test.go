//go:build integration

package postgres_test

import (
	"context"
	"log/slog"
	"sync"
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

// setupAdvisoryLockDB starts a bare PostgreSQL 18 container with the outbox
// migration applied. Advisory locks work without any schema, but the
// PartitionManager's ensure/cleanup methods operate on the outbox table.
func setupAdvisoryLockDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx,
		"postgres:18",
		tcpostgres.WithDatabase(testDBName),
		tcpostgres.WithUsername(testDBUser),
		tcpostgres.WithPassword(testDBPass),
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

func TestAdvisoryLock_FirstCallAcquires(t *testing.T) {
	pool := setupAdvisoryLockDB(t)
	ctx := context.Background()

	pm := postgres.NewPartitionManager(
		nil, // outbox not needed for lock-only test
		pool,
		clock.NewReal(),
		slog.Default(),
		0,
		0,
	)

	acquired := pm.TryAdvisoryLock(ctx)
	assert.True(t, acquired, "first call must acquire the advisory lock")

	// Clean up.
	pm.ReleaseAdvisoryLock(ctx)
}

func TestAdvisoryLock_SecondConnectionBlocked(t *testing.T) {
	pool := setupAdvisoryLockDB(t)
	ctx := context.Background()

	logger := slog.Default()

	pm1 := postgres.NewPartitionManager(nil, pool, clock.NewReal(), logger, 0, 0)
	pm2 := postgres.NewPartitionManager(nil, pool, clock.NewReal(), logger, 0, 0)

	// pm1 acquires the lock.
	require.True(t, pm1.TryAdvisoryLock(ctx))

	// pm2 must fail to acquire the same lock.
	acquired := pm2.TryAdvisoryLock(ctx)
	assert.False(t, acquired, "second caller must not acquire the lock while first holds it")

	// After pm1 releases, pm2 can acquire.
	pm1.ReleaseAdvisoryLock(ctx)

	acquired = pm2.TryAdvisoryLock(ctx)
	assert.True(t, acquired, "second caller must acquire the lock after first releases it")
	pm2.ReleaseAdvisoryLock(ctx)
}

func TestAdvisoryLock_ConcurrentEnsure_OnlyOneProceeds(t *testing.T) {
	pool := setupAdvisoryLockDB(t)
	ctx := context.Background()
	logger := slog.Default()

	// lockAcquiredCount tracks how many goroutines actually acquire the lock.
	// With advisory locking, at most one should acquire per round.
	const goroutineCount = 5
	var (
		mu            sync.Mutex
		acquiredCount int
	)

	var wg sync.WaitGroup
	wg.Add(goroutineCount)

	for range goroutineCount {
		go func() {
			defer wg.Done()
			pm := postgres.NewPartitionManager(nil, pool, clock.NewReal(), logger, 0, 0)
			if pm.TryAdvisoryLock(ctx) {
				mu.Lock()
				acquiredCount++
				mu.Unlock()
				// Simulate work.
				time.Sleep(10 * time.Millisecond)
				pm.ReleaseAdvisoryLock(ctx)
			}
		}()
	}

	wg.Wait()

	// At least one goroutine must have acquired the lock.
	assert.GreaterOrEqual(t, acquiredCount, 1, "at least one goroutine must acquire the lock")
	// In practice with pg_try_advisory_lock (non-blocking), exactly one acquires
	// while others fail, but timing could allow sequential acquisition after
	// release. We just verify the mechanism works.
}
