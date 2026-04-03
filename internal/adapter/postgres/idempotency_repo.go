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

// Cleanup deletes all expired idempotency keys.
func (r *IdempotencyRepository) Cleanup(ctx context.Context) error {
	if err := r.queries.CleanupExpiredIdempotencyKeys(ctx); err != nil {
		return fmt.Errorf("cleanup expired idempotency keys: %w", err)
	}
	return nil
}
