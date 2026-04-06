package multisub

import (
	"context"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
)

// BindingRepository defines persistence operations for the RemnawaveBinding aggregate.
type BindingRepository interface {
	GetByID(ctx context.Context, id string) (*aggregate.RemnawaveBinding, error)
	GetBySubscriptionID(ctx context.Context, subID string) ([]*aggregate.RemnawaveBinding, error)
	GetByPlatformUserID(ctx context.Context, userID string) ([]*aggregate.RemnawaveBinding, error)
	GetByRemnawaveUUID(ctx context.Context, rwUUID string) (*aggregate.RemnawaveBinding, error)
	GetActiveBySubscriptionID(ctx context.Context, subID string) ([]*aggregate.RemnawaveBinding, error)
	GetAllActive(ctx context.Context) ([]*aggregate.RemnawaveBinding, error)
	GetFailedWithRemnawaveUUID(ctx context.Context) ([]*aggregate.RemnawaveBinding, error)
	// GetFailedForReconciliation returns failed bindings that still reference a
	// Remnawave UUID, locked with FOR UPDATE SKIP LOCKED to prevent concurrent
	// processing by multiple reconciler instances.
	GetFailedForReconciliation(ctx context.Context, limit int) ([]*aggregate.RemnawaveBinding, error)
	// ResetAbandonedReconciling resets bindings stuck in 'reconciling' status for
	// longer than the given threshold back to 'failed', so they can be retried.
	// This recovers from reconciler crashes or timeouts.
	ResetAbandonedReconciling(ctx context.Context, olderThan time.Duration) (int64, error)
	Create(ctx context.Context, binding *aggregate.RemnawaveBinding) error
	Update(ctx context.Context, binding *aggregate.RemnawaveBinding) error
	Delete(ctx context.Context, id string) error
}
