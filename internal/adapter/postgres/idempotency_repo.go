package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
)

// IdempotencyKeyTTL is the retention period for idempotency keys before
// they are eligible for cleanup.
const IdempotencyKeyTTL = 24 * time.Hour

// IdempotencyRepository provides message-level deduplication backed by the
// multisub.idempotency_keys table.
type IdempotencyRepository struct {
	pool    *pgxpool.Pool
	queries *gen.Queries
}

// NewIdempotencyRepository returns a new IdempotencyRepository using the given
// connection pool.
func NewIdempotencyRepository(pool *pgxpool.Pool) *IdempotencyRepository {
	return &IdempotencyRepository{
		pool:    pool,
		queries: gen.New(pool),
	}
}

// TryAcquire attempts to insert an idempotency key. Returns true if this is
// the first time the key was seen (row inserted), false if it already exists
// (duplicate). Uses INSERT ... ON CONFLICT DO NOTHING so the check and insert
// are atomic.
func (r *IdempotencyRepository) TryAcquire(ctx context.Context, key string) (bool, error) {
	result, err := r.queries.TryAcquireIdempotencyKey(ctx, key)
	if err != nil {
		return false, fmt.Errorf("try acquire idempotency key: %w", err)
	}

	// RowsAffected: 1 = new key inserted, 0 = conflict (duplicate).
	return result.RowsAffected() == 1, nil
}

// Release removes an idempotency key so that a redelivered message can be
// reprocessed. This is called when event processing fails — without it, the
// next delivery would be silently skipped as a duplicate.
func (r *IdempotencyRepository) Release(ctx context.Context, key string) error {
	if err := r.queries.ReleaseIdempotencyKey(ctx, key); err != nil {
		return fmt.Errorf("release idempotency key: %w", err)
	}
	return nil
}

// IncrementRetry atomically increments and returns the retry count for the
// given key. Uses INSERT ... ON CONFLICT DO UPDATE so the counter persists
// across NATS redeliveries (where Watermill metadata is lost).
func (r *IdempotencyRepository) IncrementRetry(ctx context.Context, key string) (int, error) {
	count, err := r.queries.IncrementIdempotencyRetry(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("increment idempotency retry: %w", err)
	}
	return int(count), nil
}

// Cleanup deletes all expired idempotency keys.
func (r *IdempotencyRepository) Cleanup(ctx context.Context) error {
	if err := r.queries.CleanupExpiredIdempotencyKeys(ctx); err != nil {
		return fmt.Errorf("cleanup expired idempotency keys: %w", err)
	}
	return nil
}
